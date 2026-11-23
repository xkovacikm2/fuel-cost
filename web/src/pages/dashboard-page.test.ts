import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../chart", () => ({ renderChart: vi.fn(), destroyChart: vi.fn() }));

import { parseRoute } from "../lib/view";
import { dashboard, noContent, stubSignedIn } from "../testing";
import "./dashboard-page";

async function mountPage(data = dashboard(), search = "") {
  history.replaceState(null, "", `/${search}`);
  stubSignedIn({ [`/api/dashboard${search}`]: data, "DELETE /api/refuelings/5": noContent(), "DELETE /api/additional_costs/6": noContent() });
  const page = document.createElement("spotreba-dashboard-page");
  page.route = parseRoute("/");
  const navigate = vi.fn<(event: CustomEvent) => void>();
  page.addEventListener("spotreba:navigate", navigate);
  document.body.append(page);
  await vi.waitFor(() => expect(page.querySelector(".stat")).not.toBeNull());
  return { page, navigate };
}

describe("dashboard page", () => {
  beforeEach(() => history.replaceState(null, "", "/"));
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("asks the API for the query string it was opened with", async () => {
    await mountPage(dashboard(), "?scope=combined&offset=365");
    expect(fetch).toHaveBeenCalledWith("/api/dashboard?scope=combined&offset=365", expect.anything());
  });

  it("shows the chart, the summary cards and both record tables", async () => {
    const { page } = await mountPage();
    expect(page.querySelector("spotreba-fuel-chart")?.chart.title).toBe("Spotreba paliva");
    expect([...page.querySelectorAll(".stat")].map((stat) => stat.textContent)).toEqual(["3", "6.4", "345.50 €", "19.0"]);
    const [refuelings, costs] = [...page.querySelectorAll("table")];
    expect([...refuelings.querySelectorAll("tbody td")].map((cell) => cell.getAttribute("data-label"))).toEqual(
      ["Dátum", "Vozidlo", "Vzdialenosť (km)", "Množstvo", "Náklady", "Spotreba / 100 km", "Náklady / 100 km", "Akcie"],
    );
    expect(refuelings.querySelector("tbody td")?.textContent).toBe("2026-10-20");
    expect(costs.querySelector("tbody tr")?.textContent).toContain("Brzdy");
    expect(page.textContent).toContain("Záznamy tankovaní: 70.00 L / 100.00 €");
    expect(page.textContent).toContain("Dodatočné náklady: 245.50 €");
  });

  it("keeps scope, vehicle and offset in the pagination and year links", async () => {
    const { page } = await mountPage(dashboard({ offset: 365 }));
    const href = (label: string) => page.querySelector(`a[aria-label="${label}"]`)?.getAttribute("href");
    expect(href("Predchádzajúca strana")).toBe("/?scope=vehicle&offset=365&vehicle_id=1&table_page=1");
    expect(href("Nasledujúca strana")).toBe("/?scope=vehicle&offset=365&vehicle_id=1&table_page=3");
    expect(href("Predchádzajúcich 365 dní")).toBe("/?scope=vehicle&offset=730&vehicle_id=1");
    expect(href("Nasledujúcich 365 dní")).toBe("/?scope=vehicle&offset=0&vehicle_id=1");
  });

  it("applies the filter by navigating with the chosen scope and vehicle", async () => {
    const { page, navigate } = await mountPage();
    page.querySelector<HTMLSelectElement>("#scope")!.value = "combined";
    page.querySelector<HTMLSelectElement>("#dashboard_vehicle")!.value = "2";
    page.querySelector("form")!.dispatchEvent(new SubmitEvent("submit", { cancelable: true, bubbles: true }));
    expect(navigate.mock.calls[0][0].detail).toEqual({ path: "/?scope=combined&vehicle_id=2&offset=0" });
  });

  it("deletes a refueling after confirmation and reloads the dashboard with a notice", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const { page, navigate } = await mountPage();
    page.querySelector<HTMLButtonElement>('button[aria-label="Vymazať tankovanie"]')!.click();
    await vi.waitFor(() => expect(navigate).toHaveBeenCalledOnce());
    expect(confirm).toHaveBeenCalledWith("Vymazať tento záznam tankovania?");
    expect(navigate.mock.calls[0][0].detail).toEqual({ path: "/", notice: "Tankovanie bolo úspešne odstránené." });
  });
});
