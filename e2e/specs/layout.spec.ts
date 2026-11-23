import { createRule, createVehicle } from "../support/db";
import { expect, test } from "../support/fixtures";

test.describe("mobile table cards", () => {
  const screens: [string, string][] = [
    ["vehicles", "/vehicles"],
    ["maintenance reminders", "/maintenance_reminders"],
  ];

  for (const [name, path] of screens) {
    test(`${name} label every cell with its column header`, async ({ page, signedIn }) => {
      const vehicle = await createVehicle(signedIn.id, "Škoda Fabia");
      await createRule(vehicle, { kind: "oil_change", intervalDays: 365, intervalKm: 15000 });
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(path);
      await expect(page.locator("main tbody tr").first()).toBeVisible();

      // On narrow screens each cell shows its column header through td::before { content: attr(data-label) }.
      const cells = await page.evaluate(() => {
        const headers = [...document.querySelectorAll("main thead th")].map((th) => th.textContent?.trim() ?? "");
        return [...document.querySelectorAll("main tbody tr")].flatMap((row) =>
          [...row.children].map((cell, index) => ({
            header: headers[index],
            label: getComputedStyle(cell, "::before").content.replace(/^"|"$/g, ""),
          })),
        );
      });
      expect(cells.length).toBeGreaterThan(0);
      expect(cells.filter((cell) => cell.label !== cell.header)).toEqual([]);
    });
  }
});

test.describe("header", () => {
  test("navigation items, including the user menu, share one vertical centre", async ({ page, signedIn: _ }) => {
    await expect(page.getByRole("button", { name: "Používateľ" })).toBeVisible();
    const items = await page.evaluate(() =>
      [...document.querySelectorAll<HTMLElement>("header .main-nav .nav-link")]
        .filter((link) => link.getClientRects().length > 0)
        .map((link) => {
          const box = link.getBoundingClientRect();
          return { label: link.textContent?.trim() ?? "", centre: box.top + box.height / 2 };
        }),
    );
    expect(items.length).toBeGreaterThan(1);
    const centres = items.map((item) => item.centre);
    expect(Math.max(...centres) - Math.min(...centres), JSON.stringify(items)).toBeLessThanOrEqual(0.5);
  });
});
