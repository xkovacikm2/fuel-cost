import { count, query } from "../support/db";
import { backend } from "../support/env";
import { expect, test } from "../support/fixtures";

test.describe("platform", () => {
  test("health check responds", async ({ request }) => {
    const res = await request.get("/up");
    expect(res.status()).toBe(200);
  });

  test("links a web app manifest", async ({ page, request }) => {
    await page.goto("/session/new");
    const href = await page.locator('link[rel="manifest"]').getAttribute("href");
    expect(href).toBeTruthy();
    const res = await request.get(href!);
    expect(res.status()).toBe(200);
    const manifest = await res.json();
    expect(manifest).toMatchObject({
      name: "Spotreba – sledovanie paliva",
      short_name: "Spotreba",
      start_url: "/",
      scope: "/",
      display: "standalone",
      theme_color: "#0b3d91",
      background_color: "#f5f7fb",
    });
    for (const icon of manifest.icons as { src: string }[]) {
      expect((await request.get(icon.src)).status()).toBe(200);
    }
    await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute("content", "#0b3d91");
  });

  test.describe("service worker", () => {
    test.use({ serviceWorkers: "allow" });

    test("registers a service worker", async ({ page, request }) => {
      await page.goto("/session/new");
      const scriptURL = await page.evaluate(async () => (await navigator.serviceWorker.ready).active!.scriptURL);
      const res = await request.get(new URL(scriptURL).pathname);
      expect(res.status()).toBe(200);
      expect(res.headers()["content-type"]).toContain("javascript");
    });
  });

  test("navigation toggles on small screens", async ({ page, signedIn: _ }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");
    const vehicles = page.getByRole("link", { name: "Vozidlá" });
    await expect(vehicles).toBeHidden();
    await page.getByRole("button", { name: "Otvoriť navigáciu" }).click();
    await expect(vehicles).toBeVisible();
    await vehicles.click();
    await expect(page).toHaveURL(/\/vehicles$/);
  });

  test("rejects cross-origin state changes", async ({ page, signedIn }) => {
    const probe = backend.createVehicle;
    const res = await page.request.post(probe.path, {
      headers: { Origin: "https://evil.example", "Sec-Fetch-Site": "cross-site" },
      maxRedirects: 0,
        data: probe.json,
    });
    expect(res.status()).toBeGreaterThanOrEqual(400);
    expect(await count("vehicles", signedIn.id)).toBe(0);
    expect(await query("SELECT id FROM vehicles WHERE name = 'Cudzie vozidlo'")).toHaveLength(0);
  });
});
