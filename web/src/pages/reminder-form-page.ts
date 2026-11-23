import { html } from "lit";
import { fetchVehicles, request } from "../lib/api";
import { kinds } from "../lib/kinds";
import type { Lead, Reminder, Vehicle } from "../lib/types";
import { daysFromTime } from "../lib/view";
import { formActions, inputField, selectField, timeField, vehicleField } from "../components/form-fields";
import { iconButton } from "../components/icons";
import { SpotrebaPage } from "./page";

const emptyLead = (): Lead => ({ days_before: null, kilometres_before: null });
const blankReminder = (): Reminder => ({ vehicle_id: null, kind: "oil_change", interval_days: null, interval_km: null, active: true, leads: [] });

export class ReminderFormPage extends SpotrebaPage {
  static properties = { reminder: { state: true } };

  declare reminder: Reminder;
  private vehicles: Vehicle[] = [];

  constructor() {
    super();
    this.reminder = blankReminder();
  }

  private get editing() { return this.route.name === "reminder-edit"; }

  protected async load() {
    this.vehicles = await fetchVehicles();
    if (!this.editing) {
      this.reminder = { ...blankReminder(), vehicle_id: this.vehicles[0]?.id ?? null, leads: [emptyLead()] };
      return;
    }
    const { reminder } = await request<{ reminder: Reminder }>(`/api/maintenance_reminders/${this.route.id}`);
    this.reminder = { ...reminder, leads: reminder.leads.length ? reminder.leads : [emptyLead()] };
  }

  // Adding or removing a lead redraws the form, so what has been typed so far is read back into the state first.
  private captureForm() {
    const form = this.querySelector("form");
    if (!form) return;
    const values = new FormData(form);
    const days = (amount: string, unit: string) => values.get(amount) ? daysFromTime(Number(values.get(amount)), String(values.get(unit))) : null;
    this.reminder = {
      ...this.reminder,
      vehicle_id: Number(values.get("vehicle_id")),
      kind: String(values.get("kind")),
      active: values.has("active"),
      interval_time_value: String(values.get("interval_time_value") ?? ""),
      interval_time_unit: String(values.get("interval_time_unit") ?? "days"),
      interval_days: days("interval_time_value", "interval_time_unit"),
      interval_km: String(values.get("interval_km") ?? "") || null,
      leads: this.reminder.leads.map((lead, index) => ({
        ...lead,
        time_value: String(values.get(`lead_time_${index}`) ?? ""),
        time_unit: String(values.get(`lead_unit_${index}`) ?? "days"),
        days_before: days(`lead_time_${index}`, `lead_unit_${index}`),
        kilometres_before: String(values.get(`lead_km_${index}`) ?? "") || null,
      })),
    };
  }

  private changeLeads(change: (leads: Lead[]) => Lead[]) {
    this.captureForm();
    this.reminder = { ...this.reminder, leads: change(this.reminder.leads) };
  }

  private save(form: FormData) {
    const editing = this.editing;
    const interval = String(form.get("interval_time_value") ?? "");
    const leads: Lead[] = [];
    this.reminder.leads.forEach((old, index) => {
      const time = String(form.get(`lead_time_${index}`) ?? "");
      const km = String(form.get(`lead_km_${index}`) ?? "");
      if (time) leads.push({ id: old.days_before ? old.id : undefined, days_before: daysFromTime(Number(time), String(form.get(`lead_unit_${index}`))), kilometres_before: null });
      if (km) leads.push({ id: old.kilometres_before ? old.id : undefined, days_before: null, kilometres_before: km });
    });
    const body = {
      vehicle_id: Number(form.get("vehicle_id")), kind: form.get("kind"),
      interval_days: interval ? daysFromTime(Number(interval), String(form.get("interval_time_unit"))) : null,
      interval_km: form.get("interval_km") || null, active: form.has("active"), leads,
    };
    return this.persist(() => request(`/api/maintenance_reminders${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", body), {
      next: "/maintenance_reminders",
      notice: editing ? "Pripomienka údržby bola úspešne upravená." : "Pripomienka údržby bola úspešne vytvorená.",
      failure: "Pripomienku údržby sa nepodarilo uložiť:",
    });
  }

  private renderLead(lead: Lead, index: number) {
    return html`<div class="reminder-lead"><div class="grid grid-2">${timeField({ id: `lead_time_${index}`, label: "Čas vopred", unitName: `lead_unit_${index}`, value: lead.time_value ?? lead.days_before?.toString(), unit: lead.time_unit })}${inputField({ id: `lead_km_${index}`, label: "Kilometrov vopred", type: "number", min: "0.01", step: "0.01", value: lead.kilometres_before })}</div>${iconButton("Odstrániť", "trash", () => this.changeLeads((leads) => leads.filter((_, position) => position !== index)), true)}</div>`;
  }

  protected renderPage() {
    const { reminder } = this;
    return html`<section class="card"><h2>${this.editing ? "Upraviť pripomienku" : "Nová pripomienka"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.save(form))}>
        <div class="grid grid-2">${vehicleField(this.vehicles, reminder.vehicle_id)}${selectField({ id: "kind", label: "Typ údržby", options: kinds, selected: reminder.kind })}</div>
        <div class="grid grid-2">${timeField({ id: "interval_time_value", label: "Časový interval", unitName: "interval_time_unit", value: reminder.interval_time_value ?? reminder.interval_days?.toString(), unit: reminder.interval_time_unit })}
        ${inputField({ id: "interval_km", label: "Interval v kilometroch", type: "number", min: "0.01", step: "0.01", value: reminder.interval_km })}</div>
        <div class="checkbox-field"><input id="active" name="active" type="checkbox" ?checked=${reminder.active}><label for="active">Aktívna pripomienka</label></div>
        <fieldset class="reminder-leads"><legend>Pokročilé pripomienky</legend>
          ${reminder.leads.map((lead, index) => this.renderLead(lead, index))}
          ${iconButton("Pridať pripomienku", "plus", () => this.changeLeads((leads) => [...leads, emptyLead()]))}
        </fieldset>${formActions("/maintenance_reminders")}
      </form></section>`;
  }
}

customElements.define("spotreba-reminder-form-page", ReminderFormPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-reminder-form-page": ReminderFormPage }
}
