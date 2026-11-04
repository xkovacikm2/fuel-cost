import { count, query } from "../support/db";
import { PASSWORD, expect, login, newAccount, openUserMenu, test } from "../support/fixtures";
import { messageText, messagesFor, waitForMessages } from "../support/mail";

test.describe("registration", () => {
  test("creates an account, normalises the e-mail and signs in", async ({ page }) => {
    const local = `New-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    await page.goto("/sign_up");
    await page.getByLabel("E-mail").fill(`${local}@Example.COM`);
    await page.getByLabel("Heslo", { exact: true }).fill(PASSWORD);
    await page.getByLabel("Potvrdenie hesla").fill(PASSWORD);
    await page.getByRole("button", { name: "Vytvoriť účet" }).click();

    await expect(page.getByText("Vitajte! Váš účet bol vytvorený.")).toBeVisible();
    await expect(page).toHaveURL(/\/$/);
    const rows = await query("SELECT id FROM users WHERE email_address = $1", [`${local.toLowerCase()}@example.com`]);
    expect(rows).toHaveLength(1);
  });

  test("rejects mismatching password confirmation", async ({ page }) => {
    const email = `mismatch-${Date.now()}@example.com`;
    await page.goto("/sign_up");
    await page.getByLabel("E-mail").fill(email);
    await page.getByLabel("Heslo", { exact: true }).fill(PASSWORD);
    await page.getByLabel("Potvrdenie hesla").fill(`${PASSWORD}x`);
    await page.getByRole("button", { name: "Vytvoriť účet" }).click();

    await expect(page.getByText("Pri vytváraní účtu sa vyskytli chyby:")).toBeVisible();
    expect(await query("SELECT id FROM users WHERE email_address = $1", [email])).toHaveLength(0);
  });

  test("rejects an e-mail that is already registered", async ({ page, account }) => {
    await page.goto("/sign_up");
    await page.getByLabel("E-mail").fill(account.email.toUpperCase());
    await page.getByLabel("Heslo", { exact: true }).fill(PASSWORD);
    await page.getByLabel("Potvrdenie hesla").fill(PASSWORD);
    await page.getByRole("button", { name: "Vytvoriť účet" }).click();

    await expect(page.getByText("Pri vytváraní účtu sa vyskytli chyby:")).toBeVisible();
  });
});

test.describe("sign in and out", () => {
  test("signs in and lands on the dashboard", async ({ page, account }) => {
    await login(page, account.email, account.password);
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole("heading", { name: "Prehľad spotreby paliva" })).toBeVisible();
    expect(await count("sessions", account.id)).toBe(1);
  });

  test("accepts the e-mail in any case with surrounding spaces", async ({ page, account }) => {
    await login(page, `  ${account.email.toUpperCase()}  `, account.password);
    await expect(page.getByRole("heading", { name: "Prehľad spotreby paliva" })).toBeVisible();
  });

  test("rejects wrong credentials", async ({ page, account }) => {
    await login(page, account.email, "wrong-password");
    await expect(page.getByText("Skúste inú e-mailovú adresu alebo heslo.")).toBeVisible();
    await expect(page).toHaveURL(/\/session\/new$/);
    expect(await count("sessions", account.id)).toBe(0);
  });

  test("redirects anonymous visitors to sign in and back to the requested page", async ({ page, account }) => {
    await page.goto("/vehicles");
    await expect(page).toHaveURL(/\/session\/new$/);
    await page.getByLabel("E-mail").fill(account.email);
    await page.getByLabel("Heslo").fill(account.password);
    await page.getByRole("button", { name: "Prihlásiť sa" }).click();
    await expect(page).toHaveURL(/\/vehicles$/);
    await expect(page.getByRole("heading", { name: "Vozidlá", exact: true })).toBeVisible();
  });

  test("signs out and ends the session", async ({ page, signedIn }) => {
    await openUserMenu(page);
    await page.getByRole("button", { name: "Odhlásiť sa" }).click();
    await expect(page).toHaveURL(/\/session\/new$/);
    expect(await count("sessions", signedIn.id)).toBe(0);

    await page.goto("/");
    await expect(page).toHaveURL(/\/session\/new$/);
  });

  test("rate limits sign-in attempts per client", async ({ page, account }) => {
    for (let i = 0; i < 10; i++) {
      await login(page, account.email, "wrong-password");
      await expect(page.getByText("Skúste inú e-mailovú adresu alebo heslo.")).toBeVisible();
    }
    await login(page, account.email, account.password);
    await expect(page.getByText("Skúste to neskôr.")).toBeVisible();
    expect(await count("sessions", account.id)).toBe(0);
  });
});

test.describe("password reset", () => {
  const SENT = "Pokyny na obnovenie hesla boli odoslané, ak používateľ s touto e-mailovou adresou existuje.";

  async function requestReset(page: import("@playwright/test").Page, email: string) {
    await page.goto("/session/new");
    await page.getByRole("link", { name: "Zabudli ste heslo?" }).click();
    await expect(page).toHaveURL(/\/passwords\/new$/);
    await page.getByPlaceholder("Zadajte svoju e-mailovú adresu").fill(email);
    await page.getByRole("button", { name: "Odoslať pokyny na obnovenie hesla" }).click();
    await expect(page.getByText(SENT)).toBeVisible();
  }

  async function resetLink(email: string): Promise<string> {
    const [mail] = await waitForMessages(email, 1);
    expect(mail.Subject).toBe("Reset your password");
    const text = await messageText(mail.ID);
    const match = text.match(/https?:\/\/\S+(\/passwords\/\S+\/edit)/);
    expect(match, text).not.toBeNull();
    return match![1];
  }

  test("resets the password via the e-mailed link and ends other sessions", async ({ page, browser, account }) => {
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await login(otherPage, account.email, account.password);
    await expect(otherPage.getByRole("heading", { name: "Prehľad spotreby paliva" })).toBeVisible();

    await requestReset(page, account.email);
    const link = await resetLink(account.email);

    await page.goto(link);
    await expect(page.getByRole("heading", { name: "Obnoviť heslo" })).toBeVisible();
    await page.getByPlaceholder("Zadajte nové heslo").fill("Nove-heslo-456");
    await page.getByPlaceholder("Zopakujte nové heslo").fill("Nove-heslo-456");
    await page.getByRole("button", { name: "Uložiť nové heslo" }).click();
    await expect(page.getByText("Heslo bolo obnovené.")).toBeVisible();
    expect(await count("sessions", account.id)).toBe(0);

    await otherPage.goto("/vehicles");
    await expect(otherPage).toHaveURL(/\/session\/new$/);
    await other.close();

    await login(page, account.email, account.password);
    await expect(page.getByText("Skúste inú e-mailovú adresu alebo heslo.")).toBeVisible();
    await login(page, account.email, "Nove-heslo-456");
    await expect(page.getByRole("heading", { name: "Prehľad spotreby paliva" })).toBeVisible();
  });

  test("keeps the password when the confirmation does not match", async ({ page, account }) => {
    await requestReset(page, account.email);
    const link = await resetLink(account.email);

    await page.goto(link);
    await page.getByPlaceholder("Zadajte nové heslo").fill("Nove-heslo-456");
    await page.getByPlaceholder("Zopakujte nové heslo").fill("Ine-heslo-789");
    await page.getByRole("button", { name: "Uložiť nové heslo" }).click();
    await expect(page.getByText("Heslá sa nezhodujú.").first()).toBeVisible();

    await login(page, account.email, account.password);
    await expect(page.getByRole("heading", { name: "Prehľad spotreby paliva" })).toBeVisible();
  });

  test("the link stops working after the password changed", async ({ page, account }) => {
    await requestReset(page, account.email);
    const link = await resetLink(account.email);
    await page.goto(link);
    await page.getByPlaceholder("Zadajte nové heslo").fill("Nove-heslo-456");
    await page.getByPlaceholder("Zopakujte nové heslo").fill("Nove-heslo-456");
    await page.getByRole("button", { name: "Uložiť nové heslo" }).click();
    await expect(page.getByText("Heslo bolo obnovené.")).toBeVisible();

    await page.goto(link);
    await expect(page.getByText("Odkaz na obnovenie hesla je neplatný alebo vypršal.").first()).toBeVisible();
    await expect(page).toHaveURL(/\/passwords\/new$/);
  });

  test("rejects a forged token", async ({ page }) => {
    await page.goto("/passwords/not-a-real-token/edit");
    await expect(page.getByText("Odkaz na obnovenie hesla je neplatný alebo vypršal.").first()).toBeVisible();
    await expect(page).toHaveURL(/\/passwords\/new$/);
  });

  test("answers the same for unknown e-mails without sending mail", async ({ page }) => {
    const email = `nobody-${Date.now()}@example.com`;
    await requestReset(page, email);
    await page.waitForTimeout(1_000);
    expect(await messagesFor(email)).toHaveLength(0);
  });

  test("rate limits reset requests per client", async ({ page }) => {
    const target = await newAccount("ratelimit");
    for (let i = 0; i < 10; i++) {
      await requestReset(page, `nobody-${i}-${Date.now()}@example.com`);
    }
    await page.goto("/passwords/new");
    await page.getByPlaceholder("Zadajte svoju e-mailovú adresu").fill(target.email);
    await page.getByRole("button", { name: "Odoslať pokyny na obnovenie hesla" }).click();
    await expect(page.getByText("Skúste to neskôr.").first()).toBeVisible();
    await page.waitForTimeout(1_000);
    expect(await messagesFor(target.email)).toHaveLength(0);
  });
});
