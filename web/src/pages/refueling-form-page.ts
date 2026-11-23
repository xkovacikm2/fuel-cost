import { html } from "lit";
import { fetchVehicles, request } from "../lib/api";
import type { Refueling, Vehicle } from "../lib/types";
import { formActions, inputField, vehicleField } from "../components/form-fields";
import { SpotrebaPage, type Redirect } from "./page";

export class RefuelingFormPage extends SpotrebaPage {
  static properties = { selectedUnit: { state: true } };

  declare selectedUnit: string;
  private vehicles: Vehicle[] = [];
  private refueling: Refueling = { vehicle_id: null, refueled_on: "" };

  constructor() {
    super();
    this.selectedUnit = "";
  }

  private get editing() { return this.route.name === "refueling-edit"; }

  protected async load(): Promise<void | Redirect> {
    this.vehicles = await fetchVehicles();
    if (this.editing) {
      this.refueling = (await request<{ refueling: Refueling }>(`/api/refuelings/${this.route.id}`)).refueling;
      this.selectedUnit = this.unitOf(this.refueling.vehicle_id);
      return;
    }
    const defaults = await request<Refueling & { unit: string }>("/api/refuelings/new");
    if (!this.vehicles.length) return { path: "/vehicles/new", error: "Najprv vytvorte vozidlo." };
    this.refueling = defaults;
    this.selectedUnit = defaults.unit;
  }

  private unitOf(vehicleId: number | null) {
    return this.vehicles.find((vehicle) => vehicle.id === vehicleId)?.unit ?? "";
  }

  private save(form: FormData) {
    const editing = this.editing;
    const body = { ...Object.fromEntries(form), vehicle_id: Number(form.get("vehicle_id")) };
    return this.persist(() => request(`/api/refuelings${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", body), {
      next: "/",
      notice: editing ? "Tankovanie bolo úspešne upravené." : "Tankovanie bolo úspešne vytvorené.",
      failure: "Toto tankovanie sa nepodarilo uložiť:",
      foreign: editing ? "Nemôžete upravovať tankovania iného používateľa." : undefined,
    });
  }

  protected renderPage() {
    const { refueling } = this;
    return html`<section class="card"><h2>${this.editing ? "Upraviť tankovanie" : "Nové tankovanie"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.save(form))}>
        ${vehicleField(this.vehicles, refueling.vehicle_id, (event) => { this.selectedUnit = this.unitOf(Number((event.target as HTMLSelectElement).value)); })}<div class="field"><label>Jednotka</label><p data-unit class="muted">${this.selectedUnit}</p></div>
        ${inputField({ id: "refueled_on", label: "Dátum tankovania", type: "date", value: refueling.refueled_on.slice(0, 10) })}
        ${inputField({ id: "distance_km", label: "Vzdialenosť od posledného tankovania (km)", type: "number", step: "0.01", min: "0.01", value: refueling.distance_km })}
        ${inputField({ id: "amount", label: "Množstvo paliva do plnej nádrže", type: "number", step: "0.01", min: "0.01", value: refueling.amount })}
        ${inputField({ id: "cost", label: "Náklady na palivo", type: "number", step: "0.01", min: "0.01", value: refueling.cost })}
        ${formActions("/")}
      </form></section>`;
  }
}

customElements.define("spotreba-refueling-form-page", RefuelingFormPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-refueling-form-page": RefuelingFormPage }
}
