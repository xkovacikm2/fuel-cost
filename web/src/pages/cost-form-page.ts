import { html } from "lit";
import { fetchVehicles, request } from "../lib/api";
import { kinds } from "../lib/kinds";
import type { Cost, Vehicle } from "../lib/types";
import { formActions, inputField, selectField, vehicleField } from "../components/form-fields";
import { SpotrebaPage, type Redirect } from "./page";

export class CostFormPage extends SpotrebaPage {
  private vehicles: Vehicle[] = [];
  private cost: Cost = { vehicle_id: null, occurred_on: "", kind: "oil_change", comment: null, cost: "" };

  private get editing() { return this.route.name === "cost-edit"; }

  protected async load(): Promise<void | Redirect> {
    this.vehicles = await fetchVehicles();
    if (this.editing) {
      this.cost = (await request<{ additional_cost: Cost }>(`/api/additional_costs/${this.route.id}`)).additional_cost;
      return;
    }
    const defaults = await request<{ vehicle_id: number | null; occurred_on: string }>("/api/additional_costs/new");
    if (!this.vehicles.length) return { path: "/vehicles/new", error: "Najprv vytvorte vozidlo." };
    this.cost = { ...defaults, kind: "oil_change", comment: null, cost: "" };
  }

  private save(form: FormData) {
    const editing = this.editing;
    const body = { ...Object.fromEntries(form), vehicle_id: Number(form.get("vehicle_id")) };
    return this.persist(() => request(`/api/additional_costs${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", body), {
      next: "/",
      notice: editing ? "Dodatočný náklad bol úspešne upravený." : "Dodatočný náklad bol úspešne vytvorený.",
      failure: "Dodatočný náklad sa nepodarilo uložiť:",
    });
  }

  protected renderPage() {
    const { cost } = this;
    return html`<section class="card"><h2>${this.editing ? "Upraviť dodatočný náklad" : "Nový dodatočný náklad"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.save(form))}>
        ${vehicleField(this.vehicles, cost.vehicle_id)}
        ${inputField({ id: "occurred_on", label: "Dátum nákladu", type: "date", value: cost.occurred_on.slice(0, 10) })}
        ${selectField({ id: "kind", label: "Typ nákladu", options: kinds, selected: cost.kind })}
        ${inputField({ id: "comment", label: "Poznámka", maxlength: 255, value: cost.comment })}
        ${inputField({ id: "cost", label: "Náklady", type: "number", step: "0.01", min: "0.01", value: cost.cost })}
        ${formActions("/")}
      </form></section>`;
  }
}

customElements.define("spotreba-cost-form-page", CostFormPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-cost-form-page": CostFormPage }
}
