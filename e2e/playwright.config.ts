import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  globalSetup: "./support/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: process.env.CI ? 2 : 4,
  reporter: [["list"], ["html", { open: "never" }]],
  timeout: 30_000,
  expect: { timeout: 5_000 },
  use: {
    baseURL: process.env.BASE_URL ?? "http://localhost:8080",
    locale: "sk-SK",
    timezoneId: "Europe/Bratislava",
    trace: "retain-on-failure",
    // The service worker precaches "/" and would race with page requests; PWA is covered separately.
    serviceWorkers: "block",
    // Without GPU libraries the software rasterizer stalls requestAnimationFrame.
    launchOptions: { args: ["--disable-gpu", "--disable-software-rasterizer"] },
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
