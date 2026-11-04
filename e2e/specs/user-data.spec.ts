import { type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  count,
  createAdditionalCost,
  createLead,
  createNotification,
  createRefueling,
  createRule,
  createVehicle,
} from "../support/db";
import { backend, env } from "../support/env";
import { type Account, expect, login, newAccount, openUserMenu, test } from "../support/fixtures";

type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

const GOLDEN = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../fixtures/golden/export.json");
const COLLECTIONS = [
  "vehicles",
  "refuelings",
  "additional_costs",
  "maintenance_reminder_rules",
  "maintenance_reminder_leads",
  "maintenance_notifications",
] as const;

async function seedGoldenData(userId: number) {
  const ts = (s: string) => `2026-01-02 03:04:${s}`;
  const octavia = await createVehicle(userId, "Golden Octavia", "diesel", ts("05.123456"));
  await createVehicle(userId, "Golden Zoe", "electricity", ts("06"));
  await createRefueling(octavia, { on: "2026-01-10", distance: "512.30", amount: "35.50", cost: "56.80", createdAt: ts("07.5") });
  await createRefueling(octavia, { on: "2026-02-10", distance: 600, amount: 40, cost: "60.25", createdAt: ts("08") });
  const oil = await createAdditionalCost(octavia, {
    on: "2026-01-15",
    kind: "oil_change",
    cost: "89.90",
    comment: "Castrol 5W-30",
    createdAt: ts("09"),
  });
  await createAdditionalCost(octavia, { on: "2026-02-01", kind: "repair", cost: 120, createdAt: ts("10") });
  const rule = await createRule(octavia, { kind: "oil_change", intervalDays: 365, intervalKm: 15000, createdAt: ts("11") });
  const lead = await createLead(rule, { daysBefore: 30, createdAt: ts("12") });
  await createLead(rule, { kilometresBefore: 1000, createdAt: ts("13") });
  await createNotification({
    ruleId: rule,
    leadId: lead,
    costId: oil,
    kind: "advance",
    trigger: "date",
    status: "sent",
    daysRemaining: 25,
    sentAt: "2026-02-01 08:00:00.5",
    createdAt: ts("14"),
  });
}

/** Replaces database ids with stable per-collection ordinals and masks per-run account fields. */
function normalize(data: { [key: string]: Json }) {
  const ids = new Map<string, string>();
  const ref = (collection: string, id: Json) => {
    const key = `${collection}:${id}`;
    if (!ids.has(key)) ids.set(key, `${collection}#${[...ids.keys()].filter((k) => k.startsWith(`${collection}:`)).length + 1}`);
    return ids.get(key)!;
  };
  const walk = (value: Json, collection?: string): Json => {
    if (Array.isArray(value)) return value.map((v) => walk(v, collection));
    if (value === null || typeof value !== "object") return value;
    const out: { [key: string]: Json } = {};
    for (const [k, v] of Object.entries(value)) {
      if (k === "id" && collection) out[k] = ref(collection, v);
      else if (k.endsWith("_id") && v !== null) out[k] = ref(`${k.slice(0, -3)}s`, v);
      else out[k] = walk(v, (COLLECTIONS as readonly string[]).includes(k) ? k : undefined);
    }
    return out;
  };
  const result = walk(data) as { [key: string]: Json };
  result.exported_at = "<exported_at>";
  result.account = { email_address: "<email>", created_at: "<created_at>", updated_at: "<updated_at>" };
  return result;
}

async function exportFile(page: Page): Promise<{ name: string; text: string }> {
  await page.goto("/");
  await openUserMenu(page);
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("link", { name: "Exportovať JSON" }).click();
  const download = await downloadPromise;
  const text = fs.readFileSync(await download.path(), "utf8");
  return { name: download.suggestedFilename(), text };
}

async function importFile(page: Page, content: string | Buffer, name = "spotreba.json") {
  await page.goto("/");
  await openUserMenu(page);
  const chooserPromise = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Importovať JSON" }).click();
  const chooser = await chooserPromise;
  await chooser.setFiles({ name, mimeType: "application/json", buffer: Buffer.from(content) });
}

async function totals(account: Account) {
  const out: Record<string, number> = {};
  for (const table of COLLECTIONS) out[table] = await count(table, account.id);
  return out;
}

test.describe("data export and import", () => {
  test("exports the user's data as a v1 JSON attachment", async ({ page, signedIn }) => {
    await seedGoldenData(signedIn.id);
    const other = await newAccount("other");
    await createVehicle(other.id, "Cudzie");

    const { name, text } = await exportFile(page);
    expect(name).toMatch(/^spotreba-\d{8}-\d{6}\.json$/);
    const data = JSON.parse(text);
    expect(text).toBe(JSON.stringify(data, null, 2));
    expect(data.format).toBe("spotreba");
    expect(data.version).toBe(1);
    expect(data.exported_at).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/);
    expect(data.account.email_address).toBe(signedIn.email);

    const normalized = JSON.stringify(normalize(data), null, 2) + "\n";
      if (env.updateGolden) {
      fs.mkdirSync(path.dirname(GOLDEN), { recursive: true });
      fs.writeFileSync(GOLDEN, normalized);
    }
    expect(normalized).toBe(fs.readFileSync(GOLDEN, "utf8"));
  });

  test("exports an empty account", async ({ page, signedIn: _ }) => {
    const data = JSON.parse((await exportFile(page)).text);
    expect(data.vehicles).toEqual([]);
  });

  test("requires authentication to export", async ({ request }) => {
    const res = await request.get(backend.exportPath, { maxRedirects: 0 });
    expect(res.status()).not.toBe(200);
    expect(await res.text()).not.toContain('"format"');
  });

  test("round-trips an export into another account and skips duplicates on re-import", async ({ page, browser }) => {
    const source = await newAccount("source");
    await seedGoldenData(source.id);
    await login(page, source.email, source.password);
    const exported = (await exportFile(page)).text;

    const target = await newAccount("target");
    const targetContext = await browser.newContext({ extraHTTPHeaders: { "X-Forwarded-For": "10.250.0.1" } });
    const targetPage = await targetContext.newPage();
    await login(targetPage, target.email, target.password);

    await importFile(targetPage, exported);
    await expect(targetPage.getByText("Import: 10 nových, 0 preskočených záznamov.")).toBeVisible();
    expect(await totals(target)).toEqual(await totals(source));
    const reExported = JSON.parse((await exportFile(targetPage)).text);
    expect(normalize(reExported)).toEqual(normalize(JSON.parse(exported)));

    await importFile(targetPage, exported);
    await expect(targetPage.getByText("Import: 0 nových, 10 preskočených záznamov. Duplicity: 10.")).toBeVisible();
    expect(await totals(target)).toEqual(await totals(source));
    await targetContext.close();
  });

  test("marks imported queued notifications as skipped", async ({ page, signedIn }) => {
    const data = JSON.parse(fs.readFileSync(GOLDEN, "utf8"));
    data.account = { email_address: "x@example.com", created_at: "2026-01-01T00:00:00.000000Z", updated_at: "2026-01-01T00:00:00.000000Z" };
    data.exported_at = "2026-03-01T00:00:00.000000Z";
    let next = 1;
    const ids = new Map<string, number>();
    const numeric = (v: string) => (ids.has(v) ? ids.get(v)! : (ids.set(v, next++), next - 1));
    const replace = (value: unknown): unknown => {
      if (Array.isArray(value)) return value.map(replace);
      if (typeof value === "string" && /^[a-z_]+#\d+$/.test(value)) return numeric(value);
      if (value && typeof value === "object") {
        return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, replace(v)]));
      }
      return value;
    };
    const file = replace(data) as typeof data;
    file.vehicles[0].maintenance_notifications[0].status = "queued";
    file.vehicles[0].maintenance_notifications[0].sent_at = null;

    await importFile(page, JSON.stringify(file));
    await expect(
      page.getByText("Import: 10 nových, 0 preskočených záznamov. Čakajúce upozornenia označené ako preskočené: 1."),
    ).toBeVisible();
    expect(await count("maintenance_notifications", signedIn.id)).toBe(1);
  });

  test("rejects malformed JSON without changes", async ({ page, signedIn }) => {
    await importFile(page, "{");
    await expect(page.getByText("Import zlyhal: Neplatný JSON alebo príliš hlboké vnorenie.")).toBeVisible();
    expect(await count("vehicles", signedIn.id)).toBe(0);
  });

  test("rejects unsupported versions without changes", async ({ page, signedIn }) => {
    const file = { format: "spotreba", version: 2, exported_at: "2026-03-01T00:00:00Z", account: {}, vehicles: [] };
    await importFile(page, JSON.stringify(file));
    await expect(page.getByText("Import zlyhal: version: nepodporovaný formát alebo verzia.")).toBeVisible();
    expect(await count("vehicles", signedIn.id)).toBe(0);
  });

  test("rejects an empty upload", async ({ page, signedIn: _ }) => {
    await importFile(page, "");
    await expect(page.getByText("Import: vyberte JSON súbor do 10 MiB.")).toBeVisible();
  });
});
