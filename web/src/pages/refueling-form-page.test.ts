import { afterEach, describe, expect, it, vi } from "vitest";
import { parseRoute } from "../lib/view";
import { stubSignedIn, vehicles } from "../testing";
import "./refueling-form-page";

function mountPage(path: string) {
  const page = document.createElement("spotreba-refueling-form-page");
  page.route = parseRoute(path);
  const navigate = vi.fn<(event: CustomEvent) => void>();
  page.addEventListener("spotreba:navigate", navigate);
  document.body.append(page);
  return { page, navigate };
}

describe("refueling form page", () => {
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("sends the user to create a vehicle first when there is none", async () => {
    stubSignedIn({ "/api/vehicles": { vehicles: [] }, "/api/refuelings/new": { vehicle_id: null, refueled_on: "2026-11-04", unit: "" } });
    const { page, navigate } = mountPage("/refuelings/new");
    await vi.waitFor(() => expect(navigate).toHaveBeenCalledOnce());
    expect(navigate.mock.calls[0][0].detail).toEqual({ path: "/vehicles/new", error: "Najprv vytvorte vozidlo." });
    expect(page.querySelector("form")).toBeNull();
  });

  it("fills the form from the refueling being edited and shows its vehicle's unit", async () => {
    const refueling = { id: 5, vehicle_id: 2, refueled_on: "2026-10-20T00:00:00Z", distance_km: "500.00", amount: "35.50", cost: "52.00" };
    stubSignedIn({ "/api/refuelings/5": { refueling } });
    const { page } = mountPage("/refuelings/5/edit");
    await vi.waitFor(() => expect(page.querySelector("form")).not.toBeNull());
    expect(page.querySelector("h2")?.textContent).toBe("Upraviť tankovanie");
    expect(page.querySelector<HTMLSelectElement>("#vehicle_id")?.value).toBe(String(vehicles[1].id));
    expect(page.querySelector<HTMLInputElement>("#refueled_on")?.value).toBe("2026-10-20");
    expect(page.querySelector<HTMLInputElement>("#amount")?.value).toBe("35.50");
    expect(page.querySelector("[data-unit]")?.textContent).toBe("kWh");
  });

  it("explains a rejected save without the API detail and stays on the form", async () => {
    stubSignedIn({
      "/api/refuelings/new": { vehicle_id: 1, refueled_on: "2026-11-04", unit: "L" },
      "POST /api/refuelings": new Response(JSON.stringify({ error: "Množstvo musí byť väčšie ako 0." }), { status: 422 }),
    });
    const { page, navigate } = mountPage("/refuelings/new");
    const flash = vi.fn<(event: CustomEvent) => void>();
    page.addEventListener("spotreba:flash", flash);
    await vi.waitFor(() => expect(page.querySelector("form")).not.toBeNull());

    page.querySelector("form")!.dispatchEvent(new SubmitEvent("submit", { cancelable: true, bubbles: true }));

    await vi.waitFor(() => expect(flash).toHaveBeenCalledTimes(2));
    expect(flash.mock.calls.map(([event]) => event.detail)).toEqual([{ error: "" }, { error: "Toto tankovanie sa nepodarilo uložiť:" }]);
    expect(navigate).not.toHaveBeenCalled();
  });
});
