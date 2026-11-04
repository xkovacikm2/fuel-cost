import { describe, expect, it } from "vitest";
import { daysFromTime, formatCurrency, formatDecimal, fuelUnit, parseRoute, selectedVehicleId } from "./view";

describe("Rails URL router", () => {
  it.each([
    ["/", "dashboard"],
    ["/dashboard", "dashboard"],
    ["/session/new", "login"],
    ["/sign_up", "registration"],
    ["/passwords/new", "password-request"],
    ["/passwords/reset-token/edit", "password-edit"],
    ["/vehicles", "vehicles"],
    ["/vehicles/new", "vehicle-new"],
    ["/vehicles/42", "vehicle-show"],
    ["/vehicles/42/edit", "vehicle-edit"],
    ["/refuelings/new", "refueling-new"],
    ["/refuelings/42/edit", "refueling-edit"],
    ["/additional_costs/new", "cost-new"],
    ["/additional_costs/42/edit", "cost-edit"],
    ["/maintenance_reminders", "reminders"],
    ["/maintenance_reminders/new", "reminder-new"],
    ["/maintenance_reminders/42/edit", "reminder-edit"],
  ])("routes %s to %s", (path, name) => {
    expect(parseRoute(path).name).toBe(name);
  });

  it("keeps dashboard query parameters available to controls", () => {
    expect(parseRoute("/?scope=combined&offset=365&table_page=2")).toMatchObject({
      name: "dashboard",
      query: { scope: "combined", offset: "365", table_page: "2" },
    });
  });
});

describe("display and form helpers", () => {
  it("formats fixed precision and currency like the Rails dashboard", () => {
    expect(formatDecimal("6.9", 1)).toBe("6.9");
    expect(formatDecimal("104", 2)).toBe("104.00");
    expect(formatCurrency("1167.5")).toBe("1,167.50 €");
  });

  it("uses the fuel unit for the selected vehicle and defaults to the latest one", () => {
    const vehicles = [
      { id: 8, fuel_type: "petrol" },
      { id: 9, fuel_type: "electricity" },
    ];
    expect(fuelUnit("petrol")).toBe("L");
    expect(fuelUnit("diesel")).toBe("L");
    expect(fuelUnit("electricity")).toBe("kWh");
    expect(selectedVehicleId(vehicles, 9)).toBe(9);
    expect(selectedVehicleId(vehicles, null)).toBe(8);
  });

  it("converts reminder intervals using Rails' fixed time-unit lengths", () => {
    expect(daysFromTime(2, "weeks")).toBe(14);
    expect(daysFromTime(3, "months")).toBe(90);
    expect(daysFromTime(1, "years")).toBe(365);
  });
});
