import { html } from "lit";
import { request } from "../lib/api";
import { removeVehicle } from "../lib/removals";
import type { Vehicle } from "../lib/types";
import { iconButton, iconLink } from "../components/icons";
import { SpotrebaPage } from "./page";

export class VehiclePage extends SpotrebaPage {
  private vehicle!: Vehicle;

  protected async load() {
    this.vehicle = (await request<{ vehicle: Vehicle }>(`/api/vehicles/${this.route.id}`)).vehicle;
  }

  protected renderPage() {
    const { id, name, fuel_type_label: fuelType, unit } = this.vehicle;
    return html`<section class="card"><h2>${name}</h2><p><strong>Typ paliva:</strong> ${fuelType}</p><p><strong>Jednotka:</strong> ${unit}</p><div class="actions">${iconLink(`/vehicles/${id}/edit`, "Upraviť vozidlo", "edit")}${iconLink("/vehicles", "Späť", "arrow-left")}${iconButton("Vymazať vozidlo", "trash", () => this.deleteRecord(removeVehicle(id)), true)}</div></section>`;
  }
}

customElements.define("spotreba-vehicle-page", VehiclePage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-vehicle-page": VehiclePage }
}
