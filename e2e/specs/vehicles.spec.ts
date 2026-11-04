import { count, createRefueling, createVehicle, query } from "../support/db";
import { daysAgo } from "../support/dates";
import { expect, newAccount, test } from "../support/fixtures";

test.describe("vehicles", () => {
  test("lists only the signed-in user's vehicles sorted by name", async ({ page, signedIn }) => {
    const other = await newAccount("other");
    await createVehicle(signedIn.id, "Zeta", "diesel");
    await createVehicle(signedIn.id, "Alfa", "electricity");
    await createVehicle(other.id, "Cudzie auto");

    await page.goto("/vehicles");
    const rows = page.locator("tbody tr");
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0)).toContainText("Alfa");
    await expect(rows.nth(0)).toContainText("Elektrina");
    await expect(rows.nth(0)).toContainText("kWh");
    await expect(rows.nth(1)).toContainText("Zeta");
    await expect(rows.nth(1)).toContainText("Nafta");
    await expect(rows.nth(1)).toContainText("L");
    await expect(page.getByText("Cudzie auto")).toHaveCount(0);
  });

  test("shows an empty state", async ({ page, signedIn: _ }) => {
    await page.goto("/vehicles");
    await expect(page.getByText("Zatiaľ žiadne vozidlá.")).toBeVisible();
  });

  test("creates a vehicle", async ({ page, signedIn }) => {
    await page.goto("/vehicles");
    await page.getByRole("link", { name: "Nové vozidlo" }).click();
    await expect(page).toHaveURL(/\/vehicles\/new$/);
    await page.getByLabel("Názov vozidla").fill("Škoda Octavia");
    await page.getByLabel("Typ paliva").selectOption({ label: "Nafta" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Vozidlo bolo úspešne vytvorené.")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Škoda Octavia" })).toBeVisible();
    await expect(page.getByText("Typ paliva: Nafta")).toBeVisible();
    await expect(page.getByText("Jednotka: L")).toBeVisible();
    const rows = await query("SELECT name, fuel_type FROM vehicles WHERE user_id = $1", [signedIn.id]);
    expect(rows).toEqual([{ name: "Škoda Octavia", fuel_type: 1 }]);
  });

  test("requires a name", async ({ page, signedIn }) => {
    await page.goto("/vehicles/new");
    await page.getByRole("button", { name: "Uložiť" }).click();
    await expect(page.getByText("Toto vozidlo sa nepodarilo uložiť:")).toBeVisible();
    expect(await count("vehicles", signedIn.id)).toBe(0);
  });

  test("edits a vehicle", async ({ page, signedIn }) => {
    const id = await createVehicle(signedIn.id, "Starý názov", "petrol");
    await page.goto("/vehicles");
    await page.getByRole("link", { name: "Upraviť" }).click();
    await expect(page).toHaveURL(new RegExp(`/vehicles/${id}/edit$`));
    await expect(page.getByLabel("Názov vozidla")).toHaveValue("Starý názov");
    await page.getByLabel("Názov vozidla").fill("Nový názov");
    await page.getByLabel("Typ paliva").selectOption({ label: "Elektrina" });
    await page.getByRole("button", { name: "Uložiť" }).click();

    await expect(page.getByText("Vozidlo bolo úspešne upravené.")).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/vehicles/${id}$`));
    await expect(page.getByText("Jednotka: kWh")).toBeVisible();
    const rows = await query("SELECT name, fuel_type FROM vehicles WHERE id = $1", [id]);
    expect(rows).toEqual([{ name: "Nový názov", fuel_type: 2 }]);
  });

  test("deletes a vehicle with its refuelings after confirmation", async ({ page, signedIn, dialogs }) => {
    const id = await createVehicle(signedIn.id, "Na zmazanie");
    await createRefueling(id, { on: daysAgo(1), distance: 100, amount: 7, cost: 10 });
    await page.goto(`/vehicles/${id}`);
    await page.getByRole("button", { name: "Vymazať" }).click();

    await expect(page.getByText("Vozidlo bolo úspešne odstránené.")).toBeVisible();
    await expect(page).toHaveURL(/\/vehicles$/);
    expect(dialogs).toEqual(["Vymazať toto vozidlo a všetky súvisiace tankovania?"]);
    expect(await count("vehicles", signedIn.id)).toBe(0);
    expect(await query("SELECT id FROM refuelings WHERE vehicle_id = $1", [id])).toHaveLength(0);
  });

  test("does not expose another user's vehicle", async ({ page, signedIn: _ }) => {
    const other = await newAccount("other");
    const id = await createVehicle(other.id, "Tajné auto");
    for (const path of [`/vehicles/${id}`, `/vehicles/${id}/edit`]) {
      await page.goto(path);
      await expect(page.getByText("Tajné auto")).toHaveCount(0);
      await expect(page.locator(`input[value="Tajné auto"]`)).toHaveCount(0);
    }
    expect(await count("vehicles", other.id)).toBe(1);
  });
});
