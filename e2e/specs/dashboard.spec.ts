import { type Page } from "@playwright/test";
import { createAdditionalCost, createRefueling, createVehicle } from "../support/db";
import { addDays, daysAgo, today } from "../support/dates";
import { expect, newAccount, stat, test } from "../support/fixtures";

function window(offset = 0) {
  const end = addDays(today(), -offset);
  return `${addDays(end, -364)} - ${end}`;
}

function sectionHeading(page: Page, prefix: string) {
  return page.getByRole("heading", { name: new RegExp(`^${prefix}`) });
}

async function seedTwoVehicles(userId: number) {
  const a = await createVehicle(userId, "Auto A", "petrol");
  const b = await createVehicle(userId, "Auto B", "electricity");
  await createRefueling(b, { on: daysAgo(40), distance: 100, amount: 15, cost: 4.5 });
  await createRefueling(a, { on: daysAgo(30), distance: 500, amount: 35, cost: 56 });
  await createRefueling(a, { on: daysAgo(20), distance: 400, amount: 30, cost: 48.6 });
  await createRefueling(a, { on: daysAgo(10), distance: 600, amount: 39, cost: 62.4 });
  await createAdditionalCost(a, { on: daysAgo(15), kind: "repair", cost: "1000.50", comment: "Spojka" });
  return { a, b };
}

test.describe("dashboard", () => {
  test("shows empty states for a new account", async ({ page, signedIn: _ }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: `Vozidlo (${window()})` })).toBeVisible();
    await expect(page.getByText("V tomto období nie sú žiadne údaje o tankovaní ani dodatočných nákladoch.")).toBeVisible();
    await expect(stat(page, "Dni od posledného tankovania")).toHaveText("—");
    await expect(stat(page, "Priemerná spotreba / 100 km")).toHaveText("—");
    await expect(stat(page, "Celkové náklady")).toHaveText("0.00 €");
    await expect(stat(page, "Priemerný počet dní medzi tankovaniami")).toHaveText("—");
    await expect(sectionHeading(page, "Záznamy tankovaní:")).toHaveText("Záznamy tankovaní: 0.00 L / 0.00 €");
    await expect(page.getByText("Zatiaľ tu nie sú žiadne záznamy tankovaní.")).toBeVisible();
    await expect(sectionHeading(page, "Dodatočné náklady:")).toHaveText("Dodatočné náklady: 0.00 €");
    await expect(page.getByText("Zatiaľ tu nie sú žiadne dodatočné náklady.")).toBeVisible();
    await expect(page.getByLabel("Vozidlo (v režime podľa vozidla)")).toBeDisabled();
    await expect(page.getByLabel("Vozidlo (v režime podľa vozidla)")).toContainText("Zatiaľ žiadne vozidlá");
    await expect(page.getByRole("link", { name: "Nasledujúcich 365 dní" })).toHaveCount(0);
  });

  test("defaults to the vehicle of the most recent refueling", async ({ page, signedIn }) => {
    await seedTwoVehicles(signedIn.id);
    await page.goto("/");

    await expect(page.getByRole("heading", { name: `Auto A (${window()})` })).toBeVisible();
    await expect(page.getByLabel("Graf spotreby paliva")).toBeVisible();
    await expect(page.getByText("Zvislé prerušované čiary označujú dodatočné náklady.")).toBeVisible();
    const chart = await page.locator("canvas").evaluate((canvas) => {
      const chartWindow = window as typeof window & {
        Chart?: {
          getChart: (canvas: HTMLCanvasElement) => {
            data: { labels: string[]; datasets: { label: string; data: (number | null)[] }[] };
          } | undefined;
          registry: { plugins: { get: (id: string) => unknown } };
        };
      };
      const instance = chartWindow.Chart?.getChart(canvas as HTMLCanvasElement);
      return instance && {
        labels: instance.data.labels,
        datasets: instance.data.datasets.map(({ label, data }) => ({ label, data })),
        hasAdditionalCostPlugin: Boolean(chartWindow.Chart?.registry.plugins.get("additionalCosts")),
      };
    });
    expect(chart).toEqual({
      labels: [daysAgo(30), daysAgo(20), daysAgo(10)],
      datasets: [
        { label: "Spotreba na 100 km (L)", data: ["7.0", "7.5", "6.5"] },
        { label: "Náklady na 100 km", data: ["11.2", "12.15", "10.4"] },
      ],
      hasAdditionalCostPlugin: true,
    });
    await expect(page.getByLabel("Rozsah grafu")).toHaveValue("vehicle");
    await expect(page.getByLabel("Vozidlo (v režime podľa vozidla)").locator("option:checked")).toHaveText("Auto A");

    await expect(stat(page, "Dni od posledného tankovania")).toHaveText("10");
    await expect(stat(page, "Priemerná spotreba / 100 km")).toHaveText("6.9");
    await expect(stat(page, "Celkové náklady")).toHaveText("1,167.50 €");
    await expect(stat(page, "Priemerný počet dní medzi tankovaniami")).toHaveText("10.0");
    await expect(sectionHeading(page, "Záznamy tankovaní:")).toHaveText("Záznamy tankovaní: 104.00 L / 167.00 €");
    await expect(sectionHeading(page, "Dodatočné náklady:")).toHaveText("Dodatočné náklady: 1,000.50 €");
  });

  test("lists all refuelings newest first with derived metrics", async ({ page, signedIn }) => {
    await seedTwoVehicles(signedIn.id);
    await page.goto("/");

    const table = page.locator("table").filter({ has: page.getByRole("columnheader", { name: "Spotreba / 100 km" }) });
    const rows = table.locator("tbody tr");
    await expect(rows).toHaveCount(4);
    const cells = async (i: number) => (await rows.nth(i).locator("td").allInnerTexts()).slice(0, 7).map((t) => t.trim());
    expect(await cells(0)).toEqual([daysAgo(10), "Auto A", "600.00", "39.00 L", "62.40", "6.50", "10.40"]);
    expect(await cells(3)).toEqual([daysAgo(40), "Auto B", "100.00", "15.00 kWh", "4.50", "15.00", "4.50"]);
    await expect(page.getByText("Zobrazuje sa 4 z 4 záznamov")).toBeVisible();
    await expect(page.getByText("Strana 1 / 1")).toBeVisible();

    const costs = page.locator("table").filter({ has: page.getByRole("columnheader", { name: "Poznámka" }) });
    expect((await costs.locator("tbody tr").first().locator("td").allInnerTexts()).slice(0, 5).map((t) => t.trim())).toEqual([
      daysAgo(15),
      "Auto A",
      "Oprava",
      "Spojka",
      "1,000.50 €",
    ]);
  });

  test("switches to another vehicle", async ({ page, signedIn }) => {
    await seedTwoVehicles(signedIn.id);
    await page.goto("/");
    await page.getByLabel("Vozidlo (v režime podľa vozidla)").selectOption({ label: "Auto B" });
    await page.getByRole("button", { name: "Použiť" }).click();

    await expect(page.getByRole("heading", { name: `Auto B (${window()})` })).toBeVisible();
    await expect(stat(page, "Dni od posledného tankovania")).toHaveText("40");
    await expect(stat(page, "Priemerná spotreba / 100 km")).toHaveText("15.0");
    await expect(stat(page, "Celkové náklady")).toHaveText("4.50 €");
    await expect(stat(page, "Priemerný počet dní medzi tankovaniami")).toHaveText("—");
    await expect(sectionHeading(page, "Záznamy tankovaní:")).toHaveText("Záznamy tankovaní: 15.00 kWh / 4.50 €");
    await expect(sectionHeading(page, "Dodatočné náklady:")).toHaveText("Dodatočné náklady: 0.00 €");
  });

  test("combines all of the user's vehicles", async ({ page, signedIn }) => {
    await seedTwoVehicles(signedIn.id);
    const other = await newAccount("other");
    const foreign = await createVehicle(other.id, "Cudzie");
    await createRefueling(foreign, { on: daysAgo(1), distance: 100, amount: 99, cost: 999 });
    await createAdditionalCost(foreign, { on: daysAgo(1), kind: "other", cost: 999 });

    await page.goto("/");
    await page.getByLabel("Rozsah grafu").selectOption({ label: "Spolu" });
    await page.getByRole("button", { name: "Použiť" }).click();

    await expect(page.getByRole("heading", { name: `Vozidlá spolu (${window()})` })).toBeVisible();
    await expect(stat(page, "Dni od posledného tankovania")).toHaveText("10");
    await expect(stat(page, "Priemerná spotreba / 100 km")).toHaveText("7.4");
    await expect(stat(page, "Celkové náklady")).toHaveText("1,172.00 €");
    await expect(stat(page, "Priemerný počet dní medzi tankovaniami")).toHaveText("10.0");
    await expect(sectionHeading(page, "Záznamy tankovaní:")).toHaveText(
      "Záznamy tankovaní: 119.00 zmiešané jednotky / 171.50 €",
    );
    await expect(page.getByText("Cudzie")).toHaveCount(0);
  });

  test("moves the 365-day window back and forward", async ({ page, signedIn }) => {
    await seedTwoVehicles(signedIn.id);
    await page.goto("/");
    await page.getByRole("link", { name: "Predchádzajúcich 365 dní" }).click();

    await expect(page.getByRole("heading", { name: `Auto A (${window(365)})` })).toBeVisible();
    await expect(page.getByText("V tomto období nie sú žiadne údaje o tankovaní ani dodatočných nákladoch.")).toBeVisible();
    await expect(stat(page, "Priemerná spotreba / 100 km")).toHaveText("—");
    await expect(stat(page, "Celkové náklady")).toHaveText("0.00 €");
    // Day counters ignore the window.
    await expect(stat(page, "Dni od posledného tankovania")).toHaveText("10");

    await page.getByRole("link", { name: "Nasledujúcich 365 dní" }).click();
    await expect(page.getByRole("heading", { name: `Auto A (${window()})` })).toBeVisible();
    await expect(page.getByRole("link", { name: "Nasledujúcich 365 dní" })).toHaveCount(0);
  });

  test("paginates the refuelings table by ten", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Auto A");
    for (let i = 1; i <= 12; i++) {
      await createRefueling(id, { on: daysAgo(i), distance: 100 + i, amount: 7, cost: 10 });
    }
    await page.goto("/");
    await expect(page.getByText("Zobrazuje sa 10 z 12 záznamov")).toBeVisible();
    await expect(page.getByText("Strana 1 / 2")).toBeVisible();
    await expect(page.getByRole("link", { name: "Predchádzajúca strana" })).toHaveCount(0);

    await page.getByRole("link", { name: "Nasledujúca strana" }).click();
    await expect(page.getByText("Zobrazuje sa 2 z 12 záznamov")).toBeVisible();
    await expect(page.getByText("Strana 2 / 2")).toBeVisible();
    await expect(page.getByRole("cell", { name: daysAgo(12) })).toBeVisible();
    await expect(page.getByRole("link", { name: "Nasledujúca strana" })).toHaveCount(0);

    await page.getByRole("link", { name: "Predchádzajúca strana" }).click();
    await expect(page.getByText("Strana 1 / 2")).toBeVisible();
  });
});
