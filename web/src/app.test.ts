import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./chart", () => ({ renderChart: vi.fn(), destroyChart: vi.fn() }));

import "./app";
import { dashboard, jsonResponse as response, noContent, reminder, stubApi, stubSignedIn, stubSignedOut, user, vehicles } from "./testing";

async function mount(path: string, ready = "main form") {
  history.replaceState(null, "", path);
  const element = document.createElement("spotreba-app");
  document.body.append(element);
  await vi.waitFor(() => expect(element.querySelector(ready)).not.toBeNull());
  return element;
}

describe("application forms", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      if (url === "/api/me") return response({ user: { id: 1, email_address: "driver@example.com" } });
      if (url === "/api/vehicles") return response({ vehicles });
      if (url === "/api/refuelings/new") return response({ vehicle_id: 1, refueled_on: "2026-11-04", unit: "L", vehicles });
      if (url === "/api/additional_costs/new") return response({ vehicle_id: 1, occurred_on: "2026-11-04", vehicles });
      return response({});
    }));
  });
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("renders an accessible login form without requiring authentication", async () => {
    vi.mocked(fetch).mockResolvedValue(response({ error: "Prihláste sa." }, 401));
    const app = await mount("/session/new");
    expect(app.querySelector('label[for="email_address"]')?.textContent).toBe("E-mail");
    expect(app.querySelector('input[type="password"]')).not.toBeNull();
    expect(app.querySelector('a[href="/passwords/new"]')?.textContent).toBe("Zabudli ste heslo?");
  });

  it("provides authenticated navigation and a vehicle form", async () => {
    const app = await mount("/vehicles/new");
    expect(app.querySelector('a[href="/maintenance_reminders"]')).not.toBeNull();
    expect(app.querySelector('label[for="name"]')?.textContent).toBe("Názov vozidla");
    expect(app.querySelectorAll('select[name="fuel_type"] option')).toHaveLength(3);
  });

  it("offers insurance and service check kinds in the cost and reminder forms, keeping Iné last", async () => {
    const expected = [
      ["oil_change", "Výmena oleja"],
      ["fuel_filter_change", "Výmena palivového filtra"],
      ["technical_inspection", "Technická kontrola"],
      ["emissions_inspection", "Emisná kontrola"],
      ["tire_change", "Prezutie pneumatík"],
      ["repair", "Oprava"],
      ["insurance", "Poistenie"],
      ["service_check", "Servisná prehliadka"],
      ["other", "Iné"],
    ];
    for (const path of ["/additional_costs/new", "/maintenance_reminders/new"]) {
      const app = await mount(path);
      const options = [...app.querySelectorAll<HTMLOptionElement>('select[name="kind"] option')].map((option) => [option.value, option.textContent]);
      expect(options, path).toEqual(expected);
      app.remove();
    }
  });

  it("updates the refueling unit when another vehicle is selected", async () => {
    const app = await mount("/refuelings/new");
    expect(app.querySelector('[data-unit]')?.textContent).toBe("L");
    const select = app.querySelector('select[name="vehicle_id"]') as HTMLSelectElement;
    select.value = "2";
    select.dispatchEvent(new Event("change", { bubbles: true }));
    await vi.waitFor(() => expect(app.querySelector('[data-unit]')?.textContent).toBe("kWh"));
  });

  it("redirects an invalid reset link back to the request form with Rails' message", async () => {
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      if (url === "/api/me") return response({ error: "Prihláste sa." }, 401);
      if (url === "/api/passwords/invalid-token") return response({ error: "Odkaz na obnovenie hesla je neplatný alebo vypršal." }, 422);
      return response({});
    }));

    const app = await mount("/passwords/invalid-token/edit");
    await vi.waitFor(() => expect(location.pathname).toBe("/passwords/new"));
    expect(app.textContent).toContain("Odkaz na obnovenie hesla je neplatný alebo vypršal.");
  });
});

describe("action buttons", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      if (url === "/api/me") return response({ user: { id: 1, email_address: "driver@example.com" } });
      if (url === "/api/vehicles") return response({ vehicles });
      if (url === "/api/vehicles/1") return response({ vehicle: vehicles[0] });
      if (url === "/api/maintenance_reminders") return response({ reminders: [reminder] });
      if (url === "/api/maintenance_reminders/7") return response({ reminder });
      return response({});
    }));
  });
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it.each<[string, string, string[]]>([
    ["/vehicles", "main table", ["Nové vozidlo", "Zobraziť vozidlo", "Upraviť vozidlo", "Vymazať vozidlo", "Zobraziť vozidlo", "Upraviť vozidlo", "Vymazať vozidlo"]],
    ["/vehicles/1", "main h2", ["Upraviť vozidlo", "Späť", "Vymazať vozidlo"]],
    ["/vehicles/new", "main form", ["Späť"]],
    ["/maintenance_reminders", "main table", ["Nová pripomienka", "Upraviť pripomienku", "Vymazať pripomienku"]],
    ["/maintenance_reminders/7/edit", "main form", ["Odstrániť", "Odstrániť", "Pridať pripomienku", "Späť"]],
  ])("offers every action on %s as a labelled icon button", async (path, ready, labels) => {
    const app = await mount(path, ready);
    const actions = [...app.querySelectorAll<HTMLElement>("main button, main a.icon-button")]
      .filter((control) => !(control instanceof HTMLButtonElement && control.type === "submit" && control.form));
    expect(actions.map((control) => control.getAttribute("aria-label"))).toEqual(labels);
    for (const control of actions) {
      expect(control.classList.contains("icon-button"), control.outerHTML).toBe(true);
      expect(control.textContent?.trim(), control.outerHTML).toBe("");
      expect(control.querySelector(".icon"), control.outerHTML).not.toBeNull();
    }
  });
});

describe("mobile table labels", () => {
  beforeEach(() => {
    stubSignedIn({ "/api/maintenance_reminders": { reminders: [reminder] } });
  });
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  // On narrow screens every cell shows its column header from data-label (td::before { content: attr(data-label) }).
  it.each([["/vehicles"], ["/maintenance_reminders"]])("labels every %s cell with its column header", async (path) => {
    const app = await mount(path, "main table");
    const headers = [...app.querySelectorAll("main thead th")].map((th) => th.textContent);
    const rows = [...app.querySelectorAll("main tbody tr")];
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect([...row.children].map((cell) => cell.getAttribute("data-label"))).toEqual(headers);
    }
  });
});

describe("page navigation", () => {
  const refueling = { id: 5, vehicle_id: 1, refueled_on: "2026-10-20T00:00:00Z", distance_km: "500", amount: "35", cost: "52" };
  const cost = { id: 6, vehicle_id: 1, occurred_on: "2026-10-01T00:00:00Z", kind: "repair", comment: "Brzdy", cost: "245.5" };

  const signedInApi = () => stubSignedIn({
    "/api/dashboard": dashboard(),
    "/api/vehicles/1": { vehicle: vehicles[0] },
    "/api/refuelings/new": { vehicle_id: 1, refueled_on: "2026-11-04", unit: "L" },
    "/api/refuelings/5": { refueling },
    "/api/additional_costs/new": { vehicle_id: 1, occurred_on: "2026-11-04" },
    "/api/additional_costs/6": { additional_cost: cost },
    "/api/maintenance_reminders": { reminders: [reminder] },
    "/api/maintenance_reminders/7": { reminder },
    "/api/passwords/abc": { valid: true },
  });

  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it.each<[string, string]>([
    ["/", "spotreba-dashboard-page"],
    ["/session/new", "spotreba-login-page"],
    ["/sign_up", "spotreba-registration-page"],
    ["/passwords/new", "spotreba-password-request-page"],
    ["/passwords/abc/edit", "spotreba-password-reset-page"],
    ["/vehicles", "spotreba-vehicles-page"],
    ["/vehicles/new", "spotreba-vehicle-form-page"],
    ["/vehicles/1", "spotreba-vehicle-page"],
    ["/vehicles/1/edit", "spotreba-vehicle-form-page"],
    ["/refuelings/new", "spotreba-refueling-form-page"],
    ["/refuelings/5/edit", "spotreba-refueling-form-page"],
    ["/additional_costs/new", "spotreba-cost-form-page"],
    ["/additional_costs/6/edit", "spotreba-cost-form-page"],
    ["/maintenance_reminders", "spotreba-reminders-page"],
    ["/maintenance_reminders/new", "spotreba-reminder-form-page"],
    ["/maintenance_reminders/7/edit", "spotreba-reminder-form-page"],
  ])("opens %s in its own page element, %s", async (path, tag) => {
    signedInApi();
    const app = await mount(path, `${tag} section`);
    expect([...app.querySelectorAll("main > :not(.flash)")].map((element) => element.localName)).toEqual([tag]);
  });

  it("swaps the page element when a navigation link is followed", async () => {
    signedInApi();
    const app = await mount("/vehicles", "spotreba-vehicles-page table");
    const first = app.querySelector("spotreba-vehicles-page");

    app.querySelector<HTMLAnchorElement>('.main-nav a[href="/maintenance_reminders"]')!.click();

    await vi.waitFor(() => expect(app.querySelector("spotreba-reminders-page table")).not.toBeNull());
    expect(location.pathname).toBe("/maintenance_reminders");
    expect(first?.isConnected).toBe(false);
    expect(app.querySelector("spotreba-vehicles-page")).toBeNull();
  });

  it("creates a fresh page for every visit, even to the same route", async () => {
    signedInApi();
    const app = await mount("/vehicles", "spotreba-vehicles-page table");
    const first = app.querySelector("spotreba-vehicles-page");

    app.querySelector<HTMLAnchorElement>('.main-nav a[href="/vehicles"]')!.click();

    await vi.waitFor(() => expect(first?.isConnected).toBe(false));
    await vi.waitFor(() => expect(app.querySelector("spotreba-vehicles-page table")).not.toBeNull());
    expect(app.querySelector("spotreba-vehicles-page")).not.toBe(first);
  });

  it.each(["/session/new", "/sign_up", "/passwords/new", "/passwords/abc/edit"])("lets visitors open the public page %s", async (path) => {
    stubSignedOut({ "/api/passwords/abc": { valid: true } });
    const app = await mount(path, "main form");
    expect(location.pathname).toBe(path);
    expect(app.querySelector(".main-nav")?.textContent).toContain("Prihlásiť sa");
  });

  it("sends a visitor to the login page and back to the page they asked for once signed in", async () => {
    let signedIn = false;
    vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
      if (url === "/api/me") return signedIn ? response({ user }) : response({ error: "Prihláste sa." }, 401);
      if (url === "/api/session" && init?.method === "POST") { signedIn = true; return response({ user }); }
      if (url === "/api/vehicles") return response({ vehicles });
      return response({});
    }));

    const app = await mount("/vehicles?sort=name", "spotreba-login-page form");
    expect(location.pathname).toBe("/session/new");

    app.querySelector("form")!.dispatchEvent(new SubmitEvent("submit", { cancelable: true, bubbles: true }));

    await vi.waitFor(() => expect(app.querySelector("spotreba-vehicles-page table")).not.toBeNull());
    expect(location.pathname + location.search).toBe("/vehicles?sort=name");
    expect(app.querySelector(".main-nav")?.textContent).toContain("Používateľ");
  });

  it("shows flash messages sent by a page and clears them on the next navigation", async () => {
    signedInApi();
    const app = await mount("/vehicles/new", "spotreba-vehicle-form-page form");
    const page = app.querySelector("spotreba-vehicle-form-page")!;

    page.dispatchEvent(new CustomEvent("spotreba:flash", { detail: { error: "Toto vozidlo sa nepodarilo uložiť:" }, bubbles: true, composed: true }));
    await vi.waitFor(() => expect(app.querySelector("main > .flash.flash-alert")?.textContent).toBe("Toto vozidlo sa nepodarilo uložiť:"));

    page.dispatchEvent(new CustomEvent("spotreba:navigate", { detail: { path: "/vehicles", notice: "Vozidlo bolo úspešne vytvorené." }, bubbles: true, composed: true }));
    await vi.waitFor(() => expect(app.querySelector("main > .flash.flash-notice")?.textContent).toBe("Vozidlo bolo úspešne vytvorené."));
    expect(app.querySelector(".flash-alert")).toBeNull();
    expect(location.pathname).toBe("/vehicles");
  });

  it("signs out when the header announces it and returns to the login page", async () => {
    stubSignedIn({ "DELETE /api/session": noContent() });
    const app = await mount("/vehicles", "spotreba-vehicles-page table");
    // From now on the session is gone.
    stubApi({ "/api/me": response({ error: "Prihláste sa." }, 401), "DELETE /api/session": noContent() });

    app.querySelector<HTMLButtonElement>('.user-menu-panel button[title="Odhlásiť sa"]')!.click();

    await vi.waitFor(() => expect(app.querySelector("spotreba-login-page form")).not.toBeNull());
    expect(location.pathname).toBe("/session/new");
    expect(app.querySelector(".main-nav")?.textContent).toContain("Prihlásiť sa");
  });
});