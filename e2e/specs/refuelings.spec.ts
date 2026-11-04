import { type Page } from "@playwright/test";
import { count, createRefueling, createVehicle, query } from "../support/db";
import { daysAgo, today } from "../support/dates";
import { disableBrowserValidation, expect, newAccount, test } from "../support/fixtures";

function unit(page: Page) {
  return page.locator(".field").filter({ has: page.locator("label", { hasText: /^Jednotka$/ }) }).locator("p");
}

async function fillRefueling(page: Page, r: { date?: string; distance: string; amount: string; cost: string }) {
  if (r.date) await page.getByLabel("Dátum tankovania").fill(r.date);
  await page.getByLabel("Vzdialenosť od posledného tankovania (km)").fill(r.distance);
  await page.getByLabel("Množstvo paliva do plnej nádrže").fill(r.amount);
  await page.getByLabel("Náklady na palivo").fill(r.cost);
}

test.describe("refuelings", () => {
  test("redirects to vehicle creation when the user has no vehicle", async ({ page, signedIn: _ }) => {
    await page.goto("/refuelings/new");
    await expect(page).toHaveURL(/\/vehicles\/new$/);
    await expect(page.getByText("Najprv vytvorte vozidlo.")).toBeVisible();
  });

  test("defaults to today and the vehicle of the most recent refueling and follows the unit", async ({
    page,
    signedIn,
  }) => {
    const tesla = await createVehicle(signedIn.id, "Auto A Tesla", "electricity");
    const golf = await createVehicle(signedIn.id, "Auto B Golf", "petrol");
    await createRefueling(tesla, { on: daysAgo(10), distance: 200, amount: 30, cost: 9 });
    await createRefueling(golf, { on: daysAgo(3), distance: 500, amount: 35, cost: 56 });

    await page.goto("/refuelings/new");
    await expect(page.getByLabel("Vozidlo")).toHaveValue(String(golf));
    await expect(page.getByLabel("Dátum tankovania")).toHaveValue(today());
    await expect(unit(page)).toHaveText("L");

    await page.getByLabel("Vozidlo").selectOption({ label: "Auto A Tesla" });
    await expect(unit(page)).toHaveText("kWh");
  });

  test("falls back to the first vehicle by name without previous refuelings", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Zeta", "petrol");
    const alfa = await createVehicle(signedIn.id, "Alfa", "electricity");
    await page.goto("/refuelings/new");
    await expect(page.getByLabel("Vozidlo")).toHaveValue(String(alfa));
    await expect(unit(page)).toHaveText("kWh");
  });

  test("creates a refueling from the navigation", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia", "petrol");
    await page.goto("/");
    await page.getByRole("link", { name: "Nové tankovanie" }).click();
    await expect(page).toHaveURL(/\/refuelings\/new$/);
    await fillRefueling(page, { date: daysAgo(2), distance: "612.4", amount: "41.25", cost: "66.8" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Tankovanie bolo úspešne vytvorené.")).toBeVisible();
    await expect(page).toHaveURL(/\/$/);
    const rows = await query(
      "SELECT to_char(refueled_on, 'YYYY-MM-DD') AS on, distance_km, amount, cost FROM refuelings WHERE vehicle_id = $1",
      [id],
    );
    expect(rows).toEqual([{ on: daysAgo(2), distance_km: "612.40", amount: "41.25", cost: "66.80" }]);
  });

  test("rejects non-positive values", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/refuelings/new");
    await fillRefueling(page, { distance: "0", amount: "-1", cost: "0" });
    await page.getByRole("button", { name: "Uložiť" }).click();
    await page.waitForLoadState("networkidle");
    expect(await count("refuelings", signedIn.id)).toBe(0);
  });

  test("shows validation errors for values the browser lets through", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/refuelings/new");
    await disableBrowserValidation(page);
    await fillRefueling(page, { distance: "0", amount: "10", cost: "10" });
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Toto tankovanie sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("refuelings", signedIn.id)).toBe(0);
  });

  test("edits a refueling from the dashboard", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    const refueling = await createRefueling(id, { on: daysAgo(5), distance: 500, amount: 35, cost: 56 });
    await page.goto("/");
    await page.getByRole("link", { name: "Upraviť tankovanie" }).click();
    await expect(page).toHaveURL(new RegExp(`/refuelings/${refueling}/edit$`));
    expect(Number(await page.getByLabel("Množstvo paliva do plnej nádrže").inputValue())).toBe(35);
    await page.getByLabel("Množstvo paliva do plnej nádrže").fill("36.5");
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Tankovanie bolo úspešne upravené.")).toBeVisible();
    const rows = await query("SELECT amount FROM refuelings WHERE id = $1", [refueling]);
    expect(rows).toEqual([{ amount: "36.50" }]);
  });

  test("deletes a refueling after confirmation", async ({ page, signedIn, dialogs }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await createRefueling(id, { on: daysAgo(5), distance: 500, amount: 35, cost: 56 });
    await page.goto("/");
    await page.getByRole("button", { name: "Vymazať tankovanie" }).click();

    await expect(page.getByText("Tankovanie bolo úspešne odstránené.")).toBeVisible();
    expect(dialogs).toEqual(["Vymazať tento záznam tankovania?"]);
    expect(await count("refuelings", signedIn.id)).toBe(0);
  });

  test("refuses to create a refueling for another user's vehicle", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Moje");
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    await page.goto("/refuelings/new");
    await page
      .getByLabel("Vozidlo")
      .evaluate((select: HTMLSelectElement, id) => (select.options[select.selectedIndex].value = id), String(foreign));
    await fillRefueling(page, { distance: "100", amount: "7", cost: "10" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Nemôžete vytvárať tankovania pre iného používateľa.")).toBeVisible();
    expect(await count("refuelings", other.id)).toBe(0);
  });

  test("refuses to move a refueling to another user's vehicle", async ({ page, signedIn }) => {
    const mine = await createVehicle(signedIn.id, "Moje");
    const refueling = await createRefueling(mine, { on: daysAgo(5), distance: 500, amount: 35, cost: 56 });
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    await page.goto(`/refuelings/${refueling}/edit`);
    await page
      .getByLabel("Vozidlo")
      .evaluate((select: HTMLSelectElement, id) => (select.options[select.selectedIndex].value = id), String(foreign));
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Nemôžete upravovať tankovania iného používateľa.")).toBeVisible();
    const rows = await query("SELECT vehicle_id FROM refuelings WHERE id = $1", [refueling]);
    expect(rows).toEqual([{ vehicle_id: String(mine) }]);
  });

  test("does not expose another user's refueling", async ({ page, signedIn: _ }) => {
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    const refueling = await createRefueling(foreign, { on: daysAgo(5), distance: 777, amount: 35, cost: 56 });
    await page.goto(`/refuelings/${refueling}/edit`);
    await expect(page.getByText("Cudzie")).toHaveCount(0);
    const distance = page.getByLabel("Vzdialenosť od posledného tankovania (km)");
    if (await distance.count()) expect(Number(await distance.inputValue())).not.toBe(777);
  });
});
