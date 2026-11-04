import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "./app";

const vehicles = [
  { id: 1, name: "Fabia", fuel_type: "petrol", unit: "L", fuel_type_label: "Benzín" },
  { id: 2, name: "Zoe", fuel_type: "electricity", unit: "kWh", fuel_type_label: "Elektrina" },
];

function response(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
}

async function mount(path: string) {
  history.replaceState(null, "", path);
  const element = document.createElement("spotreba-app");
  document.body.append(element);
  await vi.waitFor(() => expect(element.querySelector("main form")).not.toBeNull());
  return element;
}

describe("application forms", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      if (url === "/api/me") return response({ user: { id: 1, email_address: "driver@example.com" } });
      if (url === "/api/vehicles") return response({ vehicles });
      if (url === "/api/refuelings/new") return response({ vehicle_id: 1, refueled_on: "2026-11-04", unit: "L", vehicles });
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