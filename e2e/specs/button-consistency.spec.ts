import { type Page } from "@playwright/test";
import { createAdditionalCost, createLead, createRefueling, createRule, createVehicle } from "../support/db";
import { daysAgo } from "../support/dates";
import { expect, test } from "../support/fixtures";

// All controls share one size. Only form submit buttons carry a text label; every other action is an icon button.
const CONTROL_SIZE = 40;

const VIEWPORTS = {
  desktop: { width: 1280, height: 900 },
  mobile: { width: 390, height: 844 },
} as const;

type Seed = { vehicle: number; refueling: number; cost: number; rule: number };

const PUBLIC_SCREENS: [string, string][] = [
  ["login", "/session/new"],
  ["registration", "/sign_up"],
  ["password request", "/passwords/new"],
  ["password reset", "/passwords/reset-token/edit"],
];

const SIGNED_IN_SCREENS: [string, (seed: Seed) => string][] = [
  ["dashboard", () => "/"],
  ["dashboard with both table page links", () => "/?table_page=2"],
  ["dashboard with both year links", () => "/?offset=365"],
  ["vehicles", () => "/vehicles"],
  ["new vehicle", () => "/vehicles/new"],
  ["vehicle", (seed) => `/vehicles/${seed.vehicle}`],
  ["edit vehicle", (seed) => `/vehicles/${seed.vehicle}/edit`],
  ["new refueling", () => "/refuelings/new"],
  ["edit refueling", (seed) => `/refuelings/${seed.refueling}/edit`],
  ["new cost", () => "/additional_costs/new"],
  ["edit cost", (seed) => `/additional_costs/${seed.cost}/edit`],
  ["reminders", () => "/maintenance_reminders"],
  ["new reminder", () => "/maintenance_reminders/new"],
  ["edit reminder", (seed) => `/maintenance_reminders/${seed.rule}/edit`],
];

async function seed(userId: number): Promise<Seed> {
  const vehicle = await createVehicle(userId, "Škoda Fabia");
  await createVehicle(userId, "Renault Zoe", "electricity");
  let refueling = 0;
  for (let index = 0; index < 25; index++) {
    refueling = await createRefueling(vehicle, { on: daysAgo(index * 10 + 1), distance: 500, amount: 35, cost: 52 });
  }
  const cost = await createAdditionalCost(vehicle, { on: daysAgo(20), kind: "repair", cost: 245, comment: "Brzdy" });
  const rule = await createRule(vehicle, { kind: "oil_change", intervalDays: 365, intervalKm: 15000 });
  await createLead(rule, { daysBefore: 30 });
  await createLead(rule, { kilometresBefore: 1000 });
  return { vehicle, refueling, cost, rule };
}

async function inspectControls(page: Page) {
  await expect(page.locator("main").getByRole("heading").first()).toBeVisible();
  return page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>("header .nav-toggle, main button, main a.icon-button")]
      .filter((element) => element.getClientRects().length > 0)
      .map((element) => {
        const box = element.getBoundingClientRect();
        return {
          label: element.getAttribute("aria-label") ?? element.textContent?.trim() ?? "",
          iconOnly: element.classList.contains("icon-button") || element.classList.contains("nav-toggle"),
          submit: element instanceof HTMLButtonElement && element.type === "submit" && element.form !== null,
          named: Boolean(element.getAttribute("aria-label")),
          width: Math.round(box.width * 10) / 10,
          height: Math.round(box.height * 10) / 10,
        };
      }),
  );
}

async function expectConsistentControls(page: Page) {
  const controls = await inspectControls(page);
  expect(controls.length).toBeGreaterThan(0);
  const problems = controls.flatMap((control) => {
    const found: string[] = [];
    if (control.height !== CONTROL_SIZE) found.push(`is ${control.height}px high, want ${CONTROL_SIZE}px`);
    if (control.iconOnly && control.width !== control.height) found.push(`is ${control.width}x${control.height}px, want a square`);
    if (control.iconOnly && !control.named) found.push("is an icon button without an aria-label");
    if (!control.iconOnly && !control.submit) found.push("is a text action, want an icon button");
    return found.map((problem) => `"${control.label}" ${problem}`);
  });
  expect(problems).toEqual([]);
}

for (const [viewportName, viewport] of Object.entries(VIEWPORTS)) {
  test.describe(`button consistency on ${viewportName}`, () => {
    for (const [name, path] of PUBLIC_SCREENS) {
      test(`${name} screen`, async ({ page }) => {
        await page.route("**/api/passwords/reset-token", (route) => route.fulfill({ json: { valid: true } }));
        await page.setViewportSize(viewport);
        await page.goto(path);
        await expectConsistentControls(page);
      });
    }

    for (const [name, pathFor] of SIGNED_IN_SCREENS) {
      test(`${name} screen`, async ({ page, signedIn }) => {
        const data = await seed(signedIn.id);
        // Signing in needs the desktop navigation, so only resize afterwards.
        await page.setViewportSize(viewport);
        await page.goto(pathFor(data));
        await expectConsistentControls(page);
      });
    }
  });
}
