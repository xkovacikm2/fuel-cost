export type RouteName =
  | "dashboard"
  | "login"
  | "registration"
  | "password-request"
  | "password-edit"
  | "vehicles"
  | "vehicle-new"
  | "vehicle-show"
  | "vehicle-edit"
  | "refueling-new"
  | "refueling-edit"
  | "cost-new"
  | "cost-edit"
  | "reminders"
  | "reminder-new"
  | "reminder-edit";

export type ParsedRoute = {
  name: RouteName;
  id?: number;
  token?: string;
  query: Record<string, string>;
};

export function parseRoute(input: string): ParsedRoute {
  const url = new URL(input, window.location.origin);
  const path = url.pathname.replace(/\/+$/, "") || "/";
  const query = Object.fromEntries(url.searchParams.entries());
  const matchers: [RegExp, RouteName, ("id" | "token")?][] = [
    [/^\/passwords\/([^/]+)\/edit$/, "password-edit", "token"],
    [/^\/vehicles\/(\d+)\/edit$/, "vehicle-edit", "id"],
    [/^\/vehicles\/(\d+)$/, "vehicle-show", "id"],
    [/^\/refuelings\/(\d+)\/edit$/, "refueling-edit", "id"],
    [/^\/additional_costs\/(\d+)\/edit$/, "cost-edit", "id"],
    [/^\/maintenance_reminders\/(\d+)\/edit$/, "reminder-edit", "id"],
  ];
  for (const [pattern, name, key] of matchers) {
    const match = path.match(pattern);
    if (match) return { name, [key!]: key === "id" ? Number(match[1]) : match[1], query };
  }
  const fixed: Record<string, RouteName> = {
    "/": "dashboard",
    "/dashboard": "dashboard",
    "/session/new": "login",
    "/sign_up": "registration",
    "/passwords/new": "password-request",
    "/vehicles": "vehicles",
    "/vehicles/new": "vehicle-new",
    "/refuelings/new": "refueling-new",
    "/additional_costs/new": "cost-new",
    "/maintenance_reminders": "reminders",
    "/maintenance_reminders/new": "reminder-new",
  };
  return { name: fixed[path] ?? "dashboard", query };
}

export function formatDecimal(value: string | number, precision: number): string {
  const number = typeof value === "number" ? value : Number(value);
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
    useGrouping: true,
  }).format(number);
}

export function formatCurrency(value: string | number): string {
  return `${formatDecimal(value, 2)} €`;
}

export function fuelUnit(fuelType: string): string {
  return fuelType === "electricity" ? "kWh" : fuelType === "petrol" || fuelType === "diesel" ? "L" : "";
}

export function selectedVehicleId<T extends { id: number }>(vehicles: T[], mostRecentVehicleId: number | null): number | null {
  if (mostRecentVehicleId !== null && vehicles.some((vehicle) => vehicle.id === mostRecentVehicleId)) {
    return mostRecentVehicleId;
  }
  return vehicles[0]?.id ?? null;
}

export function daysFromTime(value: number, unit: string): number {
  const daysPerUnit: Record<string, number> = { days: 1, weeks: 7, months: 30, years: 365 };
  return Math.trunc(value) * (daysPerUnit[unit] ?? 1);
}
