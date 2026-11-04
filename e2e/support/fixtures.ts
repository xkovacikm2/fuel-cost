import { test as base, expect, type Page } from "@playwright/test";
import { randomInt, randomUUID } from "node:crypto";
import { createUser } from "./db";

export type Account = { id: number; email: string; password: string };

export const PASSWORD = "Tajne-heslo-123";

export async function newAccount(prefix = "user"): Promise<Account> {
  const email = `${prefix}-${randomUUID()}@example.com`;
  const id = await createUser(email, PASSWORD);
  return { id, email, password: PASSWORD };
}

export async function login(page: Page, email: string, password: string) {
  await page.goto("/session/new");
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Heslo").fill(password);
  await page.getByRole("button", { name: "Prihlásiť sa" }).click();
  await expect(page.getByRole("button", { name: "Používateľ" }).or(page.locator(".flash"))).toBeAttached();
}

export async function openUserMenu(page: Page) {
  await page.getByRole("button", { name: "Používateľ" }).click();
}

export function stat(page: Page, title: string) {
  return page
    .getByRole("article")
    .filter({ has: page.getByRole("heading", { name: title, exact: true }) })
    .locator(".stat");
}

/** Lets server-side validation be exercised past the browser's own constraint checks. */
export async function disableBrowserValidation(page: Page, submitLabel = "Uložiť") {
  await page
    .getByRole("button", { name: submitLabel })
    .locator("xpath=ancestor::form")
    .evaluate((form: HTMLFormElement) => (form.noValidate = true));
}

type Fixtures = {
  dialogs: string[];
  account: Account;
  signedIn: Account;
};

export const test = base.extend<Fixtures>({
  // A unique client IP per test keeps per-IP rate limits from leaking between parallel tests.
  extraHTTPHeaders: async ({}, use) => {
    await use({ "X-Forwarded-For": `10.${randomInt(256)}.${randomInt(256)}.${randomInt(1, 255)}` });
  },
  dialogs: async ({ page }, use) => {
    const messages: string[] = [];
    page.on("dialog", async (dialog) => {
      messages.push(dialog.message());
      await dialog.accept();
    });
    await use(messages);
  },
  account: async ({}, use) => {
    await use(await newAccount());
  },
  signedIn: async ({ page, account, dialogs: _ }, use) => {
    await login(page, account.email, account.password);
    await expect(page.getByRole("button", { name: "Používateľ" })).toBeAttached();
    await use(account);
  },
});

export { expect };
