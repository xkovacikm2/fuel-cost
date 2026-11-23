import { html } from "lit";
import { fetchVehicles } from "../lib/api";
import { removeVehicle } from "../lib/removals";
import type { Vehicle } from "../lib/types";
import { actionsColumn, dataTable, type Column } from "../components/data-table";
import { iconButton, iconLink } from "../components/icons";
import { sectionHeading } from "../components/section-heading";
import { SpotrebaPage } from "./page";

export class VehiclesPage extends SpotrebaPage {
  private vehicles: Vehicle[] = [];

  private columns: Column<Vehicle>[] = [
    { header: "Názov", cell: (vehicle) => vehicle.name },
    { header: "Typ paliva", cell: (vehicle) => vehicle.fuel_type_label },
    { header: "Jednotka", cell: (vehicle) => vehicle.unit },
    actionsColumn((vehicle) => html`${iconLink(`/vehicles/${vehicle.id}`, "Zobraziť vozidlo", "eye")}${iconLink(`/vehicles/${vehicle.id}/edit`, "Upraviť vozidlo", "edit")}${iconButton("Vymazať vozidlo", "trash", () => this.deleteRecord(removeVehicle(vehicle.id)), true)}`),
  ];

  protected async load() {
    this.vehicles = await fetchVehicles();
  }

  protected renderPage() {
    return html`<section class="card">${sectionHeading("Vozidlá", iconLink("/vehicles/new", "Nové vozidlo", "plus"))}
      ${dataTable({ columns: this.columns, rows: this.vehicles, empty: "Zatiaľ žiadne vozidlá." })}
    </section>`;
  }
}

customElements.define("spotreba-vehicles-page", VehiclesPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-vehicles-page": VehiclesPage }
}
