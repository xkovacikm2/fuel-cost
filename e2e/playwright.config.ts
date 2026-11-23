import { defineConfig, devices } from "@playwright/test";
import { env } from "./support/env";

const startServer = process.env.E2E_START_SERVER === "1";
const baseURL = process.env.BASE_URL ?? (startServer ? "http://127.0.0.1:8081" : "http://localhost:8080");
const serverURL = startServer ? new URL(baseURL) : undefined;

if (serverURL) {
  if (serverURL.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(serverURL.hostname)) {
    throw new Error("Automatic E2E server requires an HTTP loopback BASE_URL");
  }
  if (!decodeURIComponent(new URL(env.databaseUrl).pathname).endsWith("_e2e")) {
    throw new Error("Automatic E2E server requires an E2E_DATABASE_URL with a database name ending in _e2e");
  }
}

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
  webServer: serverURL ? {
    command: "go -C ../server run ./cmd/spotreba migrate && go -C ../server run ./cmd/spotreba serve",
    url: new URL("/up", serverURL).href,
    timeout: 120_000,
    reuseExistingServer: false,
    env: {
      DATABASE_URL: env.databaseUrl,
      LISTEN_ADDR: `${serverURL.hostname}:${serverURL.port || "80"}`,
      APP_HOST: serverURL.host,
      APP_TZ: process.env.APP_TZ ?? "Europe/Bratislava",
      SECRET_KEY: process.env.SECRET_KEY ?? "development-only-secret-key-change-me",
      SMTP_ADDRESS: process.env.SMTP_ADDRESS ?? new URL(env.mailpitUrl).hostname,
      SMTP_PORT: process.env.SMTP_PORT ?? "1025",
      MAILER_FROM: process.env.MAILER_FROM ?? "spotreba@localhost",
      TRUSTED_PROXY_CIDRS: "127.0.0.0/8,::1/128",
    },
  } : undefined,
  use: {
    baseURL,
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
