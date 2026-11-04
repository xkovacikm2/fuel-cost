import { type Page } from "@playwright/test";
import { count, createAdditionalCost, createVehicle, query } from "../support/db";
import { daysAgo, today } from "../support/dates";
import { disableBrowserValidation, expect, newAccount, test } from "../support/fixtures";

const KIND_LABELS = [
  "Výmena oleja",
  "Výmena palivového filtra",
  "Technická kontrola",
  "Emisná kontrola",
  "Prezutie pneumatík",
  "Oprava",
  "Iné",
];

async function setVehicleValue(page: Page, id: number) {
  await page
    .getByLabel("Vozidlo")
    .evaluate((select: HTMLSelectElement, v) => (select.options[select.selectedIndex].value = v), String(id));
}

test.describe("additional costs", () => {
  test("redirects to vehicle creation when the user has no vehicle", async ({ page, signedIn: _ }) => {
    await page.goto("/additional_costs/new");
    await expect(page).toHaveURL(/\/vehicles\/new$/);
    await expect(page.getByText("Najprv vytvorte vozidlo.")).toBeVisible();
  });

  test("offers all cost kinds and defaults to today and the first vehicle", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Zeta");
    const alfa = await createVehicle(signedIn.id, "Alfa");
    await page.goto("/");
    await page.getByRole("link", { name: "Dodatočné náklady" }).click();
    await expect(page).toHaveURL(/\/additional_costs\/new$/);
    await expect(page.getByLabel("Vozidlo")).toHaveValue(String(alfa));
    await expect(page.getByLabel("Dátum nákladu")).toHaveValue(today());
    await expect(page.getByLabel("Typ nákladu").locator("option")).toHaveText(KIND_LABELS);
  });

  test("creates a cost with a trimmed comment", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await page.goto("/additional_costs/new");
    await page.getByLabel("Dátum nákladu").fill(daysAgo(4));
    await page.getByLabel("Typ nákladu").selectOption({ label: "Technická kontrola" });
    await page.getByLabel("Poznámka").fill("  STK Bratislava  ");
    await page.getByLabel("Náklady").fill("85.5");
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Dodatočný náklad bol úspešne vytvorený.")).toBeVisible();
    await expect(page).toHaveURL(/\/$/);
    const rows = await query(
      "SELECT to_char(occurred_on, 'YYYY-MM-DD') AS on, kind, comment, cost FROM additional_costs WHERE vehicle_id = $1",
      [id],
    );
    expect(rows).toEqual([{ on: daysAgo(4), kind: 2, comment: "STK Bratislava", cost: "85.50" }]);
  });

  test("stores a blank comment as empty", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await page.goto("/additional_costs/new");
    await page.getByLabel("Poznámka").fill("   ");
    await page.getByLabel("Náklady").fill("10");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Dodatočný náklad bol úspešne vytvorený.")).toBeVisible();
    expect(await query("SELECT comment FROM additional_costs WHERE vehicle_id = $1", [id])).toEqual([
      { comment: null },
    ]);
  });

  test("limits the comment to 255 characters", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/additional_costs/new");
    await expect(page.getByLabel("Poznámka")).toHaveAttribute("maxlength", "255");
    await disableBrowserValidation(page);
    await page.getByLabel("Poznámka").evaluate((input: HTMLInputElement) => {
      input.removeAttribute("maxlength");
      input.value = "x".repeat(256);
    });
    await page.getByLabel("Náklady").fill("10");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Dodatočný náklad sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("additional_costs", signedIn.id)).toBe(0);
  });

  test("requires a positive cost", async ({ page, signedIn }) => {
    await createVehicle(signedIn.id, "Fabia");
    await page.goto("/additional_costs/new");
    await disableBrowserValidation(page);
    await page.getByLabel("Náklady").fill("0");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Dodatočný náklad sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("additional_costs", signedIn.id)).toBe(0);
  });

  test("edits a cost from the dashboard", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    const cost = await createAdditionalCost(id, { on: daysAgo(3), kind: "repair", cost: 120, comment: "Brzdy" });
    await page.goto("/");
    await page.getByRole("link", { name: "Upraviť dodatočný náklad" }).click();
    await expect(page).toHaveURL(new RegExp(`/additional_costs/${cost}/edit$`));
    await expect(page.getByLabel("Poznámka")).toHaveValue("Brzdy");
    await expect(page.getByLabel("Typ nákladu")).toHaveValue("repair");
    await page.getByLabel("Typ nákladu").selectOption({ label: "Iné" });
    await page.getByLabel("Náklady").fill("130");
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Dodatočný náklad bol úspešne upravený.")).toBeVisible();
    expect(await query("SELECT kind, cost FROM additional_costs WHERE id = $1", [cost])).toEqual([
      { kind: 6, cost: "130.00" },
    ]);
  });

  test("deletes a cost after confirmation", async ({ page, signedIn, dialogs }) => {
    const id = await createVehicle(signedIn.id, "Fabia");
    await createAdditionalCost(id, { on: daysAgo(3), kind: "repair", cost: 120 });
    await page.goto("/");
    await page.getByRole("button", { name: "Vymazať dodatočný náklad" }).click();

    await expect(page.getByText("Dodatočný náklad bol úspešne odstránený.")).toBeVisible();
    expect(dialogs).toEqual(["Vymazať tento dodatočný náklad?"]);
    expect(await count("additional_costs", signedIn.id)).toBe(0);
  });

  test("refuses another user's vehicle on create and update", async ({ page, signedIn }) => {
    const mine = await createVehicle(signedIn.id, "Moje");
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");

    await page.goto("/additional_costs/new");
    await setVehicleValue(page, foreign);
    await page.getByLabel("Náklady").fill("10");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Nemôžete vytvárať náklady pre iného používateľa.")).toBeVisible();
    expect(await count("additional_costs", other.id)).toBe(0);

    const cost = await createAdditionalCost(mine, { on: daysAgo(3), kind: "repair", cost: 120 });
    await page.goto(`/additional_costs/${cost}/edit`);
    await setVehicleValue(page, foreign);
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Nemôžete upravovať náklady iného používateľa.")).toBeVisible();
    expect(await query("SELECT vehicle_id FROM additional_costs WHERE id = $1", [cost])).toEqual([
      { vehicle_id: String(mine) },
    ]);
  });

  test("does not expose another user's cost", async ({ page, signedIn: _ }) => {
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    const cost = await createAdditionalCost(foreign, { on: daysAgo(3), kind: "repair", cost: 120, comment: "Tajné" });
    await page.goto(`/additional_costs/${cost}/edit`);
    await expect(page.getByText("Cudzie")).toHaveCount(0);
    await expect(page.locator('input[value="Tajné"]')).toHaveCount(0);
  });
});
