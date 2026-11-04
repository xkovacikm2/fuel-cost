import { type Locator, type Page } from "@playwright/test";
import {
  count,
  createAdditionalCost,
  createLead,
  createRefueling,
  createRule,
  createVehicle,
  query,
} from "../support/db";
import { daysAgo } from "../support/dates";
import { expect, newAccount, test } from "../support/fixtures";

function intervalUnit(page: Page) {
  return page.locator(".field").filter({ has: page.getByLabel("Časový interval") }).locator("select");
}

function leads(page: Page) {
  return page.locator(".reminder-lead:visible");
}

async function fillLead(lead: Locator, l: { time?: string; unit?: string; km?: string }) {
  if (l.time !== undefined) await lead.getByLabel("Čas vopred").fill(l.time);
  if (l.unit) await lead.locator(".field", { hasText: "Čas vopred" }).locator("select").selectOption({ label: l.unit });
  if (l.km !== undefined) await lead.getByLabel("Kilometrov vopred").fill(l.km);
}

async function ruleRows(vehicleId: number) {
  return query(
    `SELECT kind, interval_days, interval_km, active FROM maintenance_reminder_rules WHERE vehicle_id = $1 ORDER BY id`,
    [vehicleId],
  );
}

async function leadRows(vehicleId: number) {
  return query(
    `SELECT l.days_before, l.kilometres_before FROM maintenance_reminder_leads l
     JOIN maintenance_reminder_rules r ON r.id = l.maintenance_reminder_rule_id
     WHERE r.vehicle_id = $1 ORDER BY l.days_before NULLS LAST, l.kilometres_before`,
    [vehicleId],
  );
}

test.describe("maintenance reminders", () => {
  test("shows an empty state and navigates from the menu", async ({ page, signedIn: _ }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Pripomienky" }).click();
    await expect(page).toHaveURL(/\/maintenance_reminders$/);
    await expect(page.getByText("Zatiaľ nemáte nastavené žiadne pripomienky údržby.")).toBeVisible();
  });

  test("new form defaults to active with one empty lead", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/maintenance_reminders");
    await page.getByRole("link", { name: "Nová pripomienka" }).click();
    await expect(page).toHaveURL(/\/maintenance_reminders\/new$/);
    await expect(page.getByLabel("Aktívna pripomienka")).toBeChecked();
    await expect(leads(page)).toHaveCount(1);
    await expect(intervalUnit(page).locator("option")).toHaveText(["Dni", "Týždne", "Mesiace", "Roky"]);
  });

  test("creates a reminder, converting units and splitting a combined lead", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await page.goto("/maintenance_reminders/new");
    await page.getByLabel("Typ údržby").selectOption({ label: "Výmena oleja" });
    await page.getByLabel("Časový interval").fill("1");
    await intervalUnit(page).selectOption({ label: "Roky" });
    await page.getByLabel("Interval v kilometroch").fill("15000");
    await fillLead(leads(page).first(), { time: "1", unit: "Mesiace", km: "1000" });
    await page.getByRole("button", { name: "Pridať pripomienku" }).click();
    await expect(leads(page)).toHaveCount(2);
    await fillLead(leads(page).nth(1), { time: "2", unit: "Týždne" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Pripomienka údržby bola úspešne vytvorená.")).toBeVisible();
    await expect(page).toHaveURL(/\/maintenance_reminders$/);
    expect(await ruleRows(id)).toEqual([{ kind: 0, interval_days: 365, interval_km: "15000.00", active: true }]);
    expect(await leadRows(id)).toEqual([
      { days_before: 14, kilometres_before: null },
      { days_before: 30, kilometres_before: null },
      { days_before: null, kilometres_before: "1000.00" },
    ]);
    const row = page.locator("tbody tr");
    await expect(row).toHaveCount(1);
    await expect(row).toContainText("Fabia");
    await expect(row).toContainText("Výmena oleja");
    await expect(row).toContainText("365 dní / 15000 km");
    await expect(row).toContainText("Čaká na prvý záznam");
  });

  test("lists reminders by vehicle and kind with their current state", async ({ page, signedIn }) => {
    const other = await newAccount("other");
    await createRule(await createVehicle(other.id, "Cudzie"), { kind: "repair", intervalDays: 10 });

    const fabia = await createVehicle(signedIn.id, "Fabia");
    const alfa = await createVehicle(signedIn.id, "Alfa");
    await createRule(fabia, { kind: "repair", intervalDays: 90, active: false });
    await createRule(fabia, { kind: "tire_change", intervalKm: "1000.5" });
    await createRule(fabia, { kind: "oil_change", intervalDays: 365, intervalKm: 10000 });
    await createRule(alfa, { kind: "technical_inspection", intervalDays: 730 });

    await createAdditionalCost(fabia, { on: daysAgo(100), kind: "oil_change", cost: 80 });
    await createAdditionalCost(fabia, { on: daysAgo(30), kind: "tire_change", cost: 40 });
    await createRefueling(fabia, { on: daysAgo(100), distance: 999, amount: 50, cost: 80 });
    await createRefueling(fabia, { on: daysAgo(50), distance: 1000, amount: 50, cost: 80 });
    await createRefueling(fabia, { on: daysAgo(20), distance: 501, amount: 30, cost: 50 });
    await createRefueling(fabia, { on: daysAgo(10), distance: 500, amount: 30, cost: 50 });

    await page.goto("/maintenance_reminders");
    const rows = page.locator("tbody tr");
    await expect(rows).toHaveCount(4);
    const cells = async (i: number) => (await rows.nth(i).locator("td").allInnerTexts()).slice(0, 4).map((t) => t.trim());
    expect(await cells(0)).toEqual(["Alfa", "Technická kontrola", "730 dní", "Čaká na prvý záznam"]);
    expect(await cells(1)).toEqual(["Fabia", "Výmena oleja", "365 dní / 10000 km", "265 dní / 7999 km"]);
    expect(await cells(2)).toEqual(["Fabia", "Prezutie pneumatík", "1000.5 km", "Termín je potrebný"]);
    expect(await cells(3)).toEqual(["Fabia", "Oprava", "90 dní", "Neaktívna"]);
    await expect(page.getByText("Cudzie")).toHaveCount(0);
  });

  test("edits interval, removes a lead and adds another", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    const rule = await createRule(id, { kind: "oil_change", intervalDays: 365, intervalKm: 10000 });
    await createLead(rule, { daysBefore: 30 });
    await createLead(rule, { kilometresBefore: 500 });

    await page.goto("/maintenance_reminders");
    await page.getByRole("link", { name: "Upraviť" }).click();
    await expect(page).toHaveURL(new RegExp(`/maintenance_reminders/${rule}/edit$`));
    expect(Number(await page.getByLabel("Časový interval").inputValue())).toBe(365);
    await expect(leads(page)).toHaveCount(2);

    for (const lead of await leads(page).all()) {
      if (Number(await lead.getByLabel("Kilometrov vopred").inputValue()) === 500) {
        await lead.getByRole("button", { name: "Odstrániť" }).click();
      }
    }
    await expect(leads(page)).toHaveCount(1);
    await page.getByRole("button", { name: "Pridať pripomienku" }).click();
    await fillLead(leads(page).nth(1), { km: "200" });
    await page.getByLabel("Časový interval").fill("2");
    await intervalUnit(page).selectOption({ label: "Roky" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Pripomienka údržby bola úspešne upravená.")).toBeVisible();
    expect(await ruleRows(id)).toEqual([{ kind: 0, interval_days: 730, interval_km: "10000.00", active: true }]);
    expect(await leadRows(id)).toEqual([
      { days_before: 30, kilometres_before: null },
      { days_before: null, kilometres_before: "200.00" },
    ]);
  });

  test("deactivates a reminder", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    const rule = await createRule(id, { kind: "oil_change", intervalDays: 365 });
    await page.goto(`/maintenance_reminders/${rule}/edit`);
    await page.getByLabel("Aktívna pripomienka").uncheck();
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Pripomienka údržby bola úspešne upravená.")).toBeVisible();
    await expect(page.locator("tbody tr")).toContainText("Neaktívna");
    expect((await ruleRows(id))[0].active).toBe(false);
  });

  test("requires at least one interval", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/maintenance_reminders/new");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Pripomienku údržby sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("maintenance_reminder_rules", signedIn.id)).toBe(0);
  });

  test("allows only one reminder per vehicle and kind", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await createRule(id, { kind: "oil_change", intervalDays: 365 });
    await page.goto("/maintenance_reminders/new");
    await page.getByLabel("Typ údržby").selectOption({ label: "Výmena oleja" });
    await page.getByLabel("Časový interval").fill("100");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Pripomienku údržby sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("maintenance_reminder_rules", signedIn.id)).toBe(1);
  });

  test("rejects a lead whose dimension the reminder has no interval for", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/maintenance_reminders/new");
    await page.getByLabel("Interval v kilometroch").fill("10000");
    await fillLead(leads(page).first(), { time: "10" });
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Pripomienku údržby sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("maintenance_reminder_rules", signedIn.id)).toBe(0);
  });

  test("deletes a reminder after confirmation", async ({ page, signedIn, dialogs }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    const rule = await createRule(id, { kind: "oil_change", intervalDays: 365 });
    await createLead(rule, { daysBefore: 30 });
    await page.goto("/maintenance_reminders");
    await page.getByRole("button", { name: "Vymazať" }).click();
    await expect(page.getByText("Pripomienka údržby bola úspešne odstránená.")).toBeVisible();
    expect(dialogs).toEqual(["Vymazať túto pripomienku?"]);
    expect(await count("maintenance_reminder_rules", signedIn.id)).toBe(0);
    expect(await count("maintenance_reminder_leads", signedIn.id)).toBe(0);
  });

  test("refuses another user's vehicle and hides another user's reminder", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Moje");
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    const foreignRule = await createRule(foreign, { kind: "repair", intervalDays: 77 });

    await page.goto("/maintenance_reminders/new");
    await page
      .getByLabel("Vozidlo")
      .evaluate((select: HTMLSelectElement, v) => (select.options[select.selectedIndex].value = v), String(foreign));
    await page.getByLabel("Časový interval").fill("100");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Nemôžete vytvárať pripomienky pre iného používateľa.")).toBeVisible();
    expect(await count("maintenance_reminder_rules", other.id)).toBe(1);

    await page.goto(`/maintenance_reminders/${foreignRule}/edit`);
    await expect(page.getByText("Cudzie")).toHaveCount(0);
  });
});
