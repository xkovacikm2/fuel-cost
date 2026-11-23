import { html } from "lit";
import { messageOf, request } from "../lib/api";
import type { Vehicle } from "../lib/types";
import { formActions, inputField, selectField } from "../components/form-fields";
import { SpotrebaPage } from "./page";

export class VehicleFormPage extends SpotrebaPage {
  private vehicle: Partial<Vehicle> = { name: "", fuel_type: "petrol" };

  private get editing() { return this.route.name === "vehicle-edit"; }

  protected async load() {
    if (this.editing) this.vehicle = (await request<{ vehicle: Vehicle }>(`/api/vehicles/${this.route.id}`)).vehicle;
  }

  private async save(form: FormData) {
    const editing = this.editing;
    let vehicle: Vehicle;
    try {
      ({ vehicle } = await request<{ vehicle: Vehicle }>(`/api/vehicles${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", Object.fromEntries(form)));
    } catch (error) {
      throw new Error(`Toto vozidlo sa nepodarilo uložiť: ${messageOf(error)}`);
    }
    this.navigate({ path: `/vehicles/${vehicle.id}`, notice: editing ? "Vozidlo bolo úspešne upravené." : "Vozidlo bolo úspešne vytvorené." });
  }

  protected renderPage() {
    return html`<section class="card"><h2>${this.editing ? "Upraviť vozidlo" : "Nové vozidlo"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.save(form))}>
        ${inputField({ id: "name", label: "Názov vozidla", value: this.vehicle.name })}
        ${selectField({ id: "fuel_type", label: "Typ paliva", options: [["petrol", "Benzín"], ["diesel", "Nafta"], ["electricity", "Elektrina"]], selected: this.vehicle.fuel_type })}
        ${formActions("/vehicles")}
      </form></section>`;
  }
}

customElements.define("spotreba-vehicle-form-page", VehicleFormPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-vehicle-form-page": VehicleFormPage }
}
