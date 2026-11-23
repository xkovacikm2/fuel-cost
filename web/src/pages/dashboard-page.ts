import { html, nothing } from "lit";
import { fetchVehicles, request } from "../lib/api";
import { removeAdditionalCost, removeRefueling } from "../lib/removals";
import type { CostRow, Dashboard, RefuelingRow, Vehicle } from "../lib/types";
import { formatCurrency, formatDecimal } from "../lib/view";
import { actionsColumn, dataTable, type Column } from "../components/data-table";
import { selectField } from "../components/form-fields";
import { glyph, iconButton, iconLink } from "../components/icons";
import { pagination } from "../components/pagination";
import { sectionHeading } from "../components/section-heading";
import { statCard } from "../components/stat-card";
import "../components/fuel-chart";
import { SpotrebaPage } from "./page";

const YEAR = 365;

export class DashboardPage extends SpotrebaPage {
  private vehicles: Vehicle[] = [];
  private data!: Dashboard;

  private refuelingColumns: Column<RefuelingRow>[] = [
    { header: "Dátum", cell: (row) => row.refueled_on.slice(0, 10) },
    { header: "Vozidlo", cell: (row) => row.vehicle_name },
    { header: "Vzdialenosť (km)", cell: (row) => formatDecimal(row.distance_km!, 2) },
    { header: "Množstvo", cell: (row) => `${formatDecimal(row.amount!, 2)} ${row.unit}` },
    { header: "Náklady", cell: (row) => formatDecimal(row.cost!, 2) },
    { header: "Spotreba / 100 km", cell: (row) => formatDecimal(row.consumption_per_100km, 2) },
    { header: "Náklady / 100 km", cell: (row) => formatDecimal(row.cost_per_100km, 2) },
    actionsColumn((row) => html`${iconLink(`/refuelings/${row.id}/edit`, "Upraviť tankovanie", "edit")}${iconButton("Vymazať tankovanie", "trash", () => this.deleteRecord(removeRefueling(row.id)), true)}`),
  ];

  private costColumns: Column<CostRow>[] = [
    { header: "Dátum", cell: (row) => row.occurred_on.slice(0, 10) },
    { header: "Vozidlo", cell: (row) => row.vehicle_name },
    { header: "Typ", cell: (row) => row.kind_label },
    { header: "Poznámka", cell: (row) => row.comment },
    { header: "Náklady", cell: (row) => formatCurrency(row.cost) },
    actionsColumn((row) => html`${iconLink(`/additional_costs/${row.id}/edit`, "Upraviť dodatočný náklad", "edit")}${iconButton("Vymazať dodatočný náklad", "trash", () => this.deleteRecord(removeAdditionalCost(row.id)), true)}`),
  ];

  protected async load() {
    [this.vehicles, this.data] = await Promise.all([fetchVehicles(), request<Dashboard>(`/api/dashboard${location.search}`)]);
  }

  // Links keep the current scope, vehicle and period unless told otherwise.
  private url(overrides: Record<string, number> = {}) {
    const { scope, offset, selected_vehicle_id: vehicleId } = this.data;
    const params = new URLSearchParams({
      scope, offset: String(offset), ...(vehicleId ? { vehicle_id: String(vehicleId) } : {}),
      ...Object.fromEntries(Object.entries(overrides).map(([key, value]) => [key, String(value)])),
    });
    return `/?${params}`;
  }

  private applyFilter(event: SubmitEvent) {
    event.preventDefault();
    const params = new URLSearchParams({ ...Object.fromEntries(new FormData(event.currentTarget as HTMLFormElement)) as Record<string, string>, offset: String(this.data.offset) });
    this.navigate({ path: `/?${params}` });
  }

  private renderStats() {
    const { stats } = this.data;
    return html`<section class="grid grid-4">
      ${statCard("Dni od posledného tankovania", stats.days_since_last_refueling ?? "—")}
      ${statCard("Priemerná spotreba / 100 km", stats.average_consumption_per_100km === null ? "—" : formatDecimal(stats.average_consumption_per_100km, 1))}
      ${statCard("Celkové náklady", formatCurrency(stats.total_cost_paid))}
      ${statCard("Priemerný počet dní medzi tankovaniami", stats.average_days_between_refuelings === null ? "—" : formatDecimal(stats.average_days_between_refuelings, 1))}
    </section>`;
  }

  private renderRefuelings() {
    const { scope, selected_vehicle_id: vehicleId, stats, refuelings } = this.data;
    const unit = scope === "combined" ? "zmiešané jednotky" : this.vehicles.find((vehicle) => vehicle.id === vehicleId)?.unit ?? "L";
    return html`<section class="card">${sectionHeading(`Záznamy tankovaní: ${formatDecimal(stats.total_fuel_refilled, 2)} ${unit} / ${formatCurrency(stats.total_fuel_cost_paid)}`, iconLink("/refuelings/new", "Pridať tankovanie", "plus"), 3)}
      ${dataTable({ columns: this.refuelingColumns, rows: refuelings.items, empty: "Zatiaľ tu nie sú žiadne záznamy tankovaní." })}
      ${refuelings.items.length ? pagination({ page: refuelings.page, totalPages: refuelings.total_pages, shown: refuelings.items.length, total: refuelings.total_count, href: (page) => this.url({ table_page: page }) }) : nothing}
    </section>`;
  }

  private renderCosts() {
    return html`<section class="card">${sectionHeading(`Dodatočné náklady: ${formatCurrency(this.data.stats.total_additional_cost_paid)}`, iconLink("/additional_costs/new", "Pridať dodatočný náklad", "plus"), 3)}
      ${dataTable({ columns: this.costColumns, rows: this.data.additional_costs, empty: "Zatiaľ tu nie sú žiadne dodatočné náklady." })}
    </section>`;
  }

  private renderFilter() {
    const { scope, offset, selected_vehicle_id: vehicleId } = this.data;
    const vehicleOptions = this.vehicles.length ? this.vehicles.map((vehicle) => [vehicle.id, vehicle.name] as const) : [["", "Zatiaľ žiadne vozidlá"] as const];
    return html`<section class="card"><h2>Prehľad spotreby paliva</h2><form @submit=${this.applyFilter}><div class="grid grid-3">
      ${selectField({ id: "scope", label: "Rozsah grafu", options: [["vehicle", "Podľa vozidla"], ["combined", "Spolu"]], selected: scope })}
      ${selectField({ id: "dashboard_vehicle", name: "vehicle_id", label: "Vozidlo (v režime podľa vozidla)", options: vehicleOptions, selected: vehicleId, disabled: !this.vehicles.length })}
      <div class="field actions"><button type="submit">${glyph("check")}Použiť</button></div></div></form><div class="actions">${iconLink(this.url({ offset: offset + YEAR }), "Predchádzajúcich 365 dní", "chevron-left")}${offset > 0 ? iconLink(this.url({ offset: Math.max(offset - YEAR, 0) }), "Nasledujúcich 365 dní", "chevron-right") : nothing}</div>
    </section>`;
  }

  protected renderPage() {
    return html`<spotreba-fuel-chart .chart=${this.data.chart}></spotreba-fuel-chart>
      ${this.renderStats()}${this.renderRefuelings()}${this.renderCosts()}${this.renderFilter()}`;
  }
}

customElements.define("spotreba-dashboard-page", DashboardPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-dashboard-page": DashboardPage }
}
