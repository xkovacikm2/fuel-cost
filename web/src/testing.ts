import { vi } from "vitest";
import type { Dashboard, User, Vehicle } from "./lib/types";

export const user: User = { id: 1, email_address: "driver@example.com" };

export const vehicles: Vehicle[] = [
  { id: 1, name: "Fabia", fuel_type: "petrol", unit: "L", fuel_type_label: "Benzín" },
  { id: 2, name: "Zoe", fuel_type: "electricity", unit: "kWh", fuel_type_label: "Elektrina" },
];

export function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
}

/** Stubs fetch. Routes are keyed by "METHOD /path" or just "/path" for GET; anything unlisted answers {}. */
export function stubApi(routes: Record<string, unknown> = {}) {
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const hit = routes[`${init?.method ?? "GET"} ${url}`] ?? routes[url] ?? {};
    return hit instanceof Response ? hit.clone() : jsonResponse(hit);
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

export function stubSignedIn(routes: Record<string, unknown> = {}) {
  return stubApi({ "/api/me": { user }, "/api/vehicles": { vehicles }, ...routes });
}

export function stubSignedOut(routes: Record<string, unknown> = {}) {
  return stubApi({ "/api/me": jsonResponse({ error: "Prihláste sa." }, 401), ...routes });
}

export const noContent = () => new Response(null, { status: 204 });

export function dashboard(overrides: Partial<Dashboard> = {}): Dashboard {
  return {
    scope: "vehicle",
    offset: 0,
    selected_vehicle_id: 1,
    chart: { title: "Spotreba paliva", labels: ["2026-10-01", "2026-10-20"], datasets: [], additional_costs: [] },
    stats: {
      days_since_last_refueling: 3,
      average_consumption_per_100km: "6.4",
      average_days_between_refuelings: "19",
      total_cost_paid: "345.5",
      total_fuel_refilled: "70",
      total_fuel_cost_paid: "100",
      total_additional_cost_paid: "245.5",
    },
    refuelings: {
      page: 2,
      total_pages: 3,
      total_count: 25,
      items: [
        { id: 5, vehicle_id: 1, vehicle_name: "Fabia", unit: "L", refueled_on: "2026-10-20T00:00:00Z", distance_km: "500", amount: "35", cost: "52", consumption_per_100km: "7", cost_per_100km: "10.4" },
      ],
    },
    additional_costs: [
      { id: 6, vehicle_id: 1, vehicle_name: "Fabia", occurred_on: "2026-10-01T00:00:00Z", kind: "repair", kind_label: "Oprava", comment: "Brzdy", cost: "245.5" },
    ],
    ...overrides,
  };
}

export const reminder = {
  id: 7, vehicle_id: 1, vehicle_name: "Fabia", kind: "oil_change", kind_label: "Výmena oleja", interval_days: 365, interval_km: "15000", active: true, status: "Aktívna",
  leads: [{ id: 1, days_before: 30, kilometres_before: null }, { id: 2, days_before: null, kilometres_before: "1000" }],
};
