import { LitElement, html, nothing } from "lit";
import { daysFromTime, formatCurrency, formatDecimal, parseRoute, type ParsedRoute } from "./lib/view";
import { renderChart, destroyChart, type ChartPayload } from "./chart";

type Vehicle = { id: number; name: string; fuel_type: string; fuel_type_label: string; unit: string };
type Refueling = { id?: number; vehicle_id: number | null; refueled_on: string; distance_km?: string; amount?: string; cost?: string };
type User = { id: number; email_address: string };
type Cost = { id?: number; vehicle_id: number | null; occurred_on: string; kind: string; comment: string | null; cost: string };
type Lead = { id?: number; days_before: number | null; kilometres_before: string | null; time_value?: string; time_unit?: string };
type Reminder = { id?: number; vehicle_id: number | null; vehicle_name?: string; kind: string; kind_label?: string; interval_days: number | null; interval_km: string | null; interval_time_value?: string; interval_time_unit?: string; active: boolean; leads: Lead[]; status?: string };
const kinds = [["oil_change", "Výmena oleja"], ["fuel_filter_change", "Výmena palivového filtra"], ["technical_inspection", "Technická kontrola"], ["emissions_inspection", "Emisná kontrola"], ["tire_change", "Prezutie pneumatík"], ["repair", "Oprava"], ["other", "Iné"]];
type Dashboard = {
  scope: string; offset: number; selected_vehicle_id: number | null;
  chart: ChartPayload;
  stats: { days_since_last_refueling: number | null; average_consumption_per_100km: string | null; average_days_between_refuelings: string | null; total_cost_paid: string; total_fuel_refilled: string; total_fuel_cost_paid: string; total_additional_cost_paid: string };
  refuelings: { page: number; total_pages: number; total_count: number; items: (Refueling & { id: number; vehicle_name: string; unit: string; consumption_per_100km: string; cost_per_100km: string })[] };
  additional_costs: (Cost & { id: number; vehicle_name: string; kind_label: string })[];
};

class RequestError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

async function request<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json();
  if (!response.ok) throw new RequestError(data.error ?? "Požiadavka zlyhala.", response.status);
  return data as T;
}

export class SpotrebaApp extends LitElement {
  static properties = {
    route: { state: true }, user: { state: true }, loading: { state: true },
    notice: { state: true }, error: { state: true }, menuOpen: { state: true },
    navOpen: { state: true }, vehicles: { state: true }, vehicle: { state: true },
    refueling: { state: true }, selectedUnit: { state: true },
    cost: { state: true }, reminder: { state: true }, reminders: { state: true },
    dashboard: { state: true },
  };

  declare route: ParsedRoute;
  declare user: User | null;
  declare loading: boolean;
  declare notice: string;
  declare error: string;
  declare menuOpen: boolean;
  declare navOpen: boolean;
  declare vehicles: Vehicle[];
  declare vehicle: Partial<Vehicle>;
  declare refueling: Refueling;
  declare selectedUnit: string;
  declare cost: Cost;
  declare reminder: Reminder;
  declare reminders: Reminder[];
  declare dashboard: Dashboard | null;
  private returnTo = "/";
  private generation = 0;
  private onPopState = () => { void this.load(); };

  constructor() {
    super();
    this.route = parseRoute(location.href);
    this.user = null;
    this.loading = true;
    this.notice = this.error = "";
    this.menuOpen = this.navOpen = false;
    this.vehicles = [];
    this.vehicle = {};
    this.refueling = { vehicle_id: null, refueled_on: "" };
    this.selectedUnit = "";
    this.cost = { vehicle_id: null, occurred_on: "", kind: "oil_change", comment: null, cost: "" };
    this.reminder = { vehicle_id: null, kind: "oil_change", interval_days: null, interval_km: null, active: true, leads: [] };
    this.reminders = [];
    this.dashboard = null;
  }

  createRenderRoot() { return this; }

  connectedCallback() {
    super.connectedCallback();
    window.addEventListener("popstate", this.onPopState);
    this.addEventListener("click", this.onLink);
    void this.load();
  }

  disconnectedCallback() {
    const canvas = this.querySelector("canvas");
    if (canvas) destroyChart(canvas);
    window.removeEventListener("popstate", this.onPopState);
    this.removeEventListener("click", this.onLink);
    super.disconnectedCallback();
  }

  private onLink = (event: MouseEvent) => {
    const anchor = (event.target as Element).closest("a");
    if (!anchor || anchor.hasAttribute("download") || anchor.dataset.external !== undefined || event.ctrlKey || event.metaKey || event.shiftKey || event.button !== 0) return;
    const url = new URL(anchor.href);
    if (url.origin !== location.origin) return;
    event.preventDefault();
    this.navigate(url.pathname + url.search);
  };

  private navigate(path: string, notice = "", error = "") {
    history.pushState(null, "", path);
    this.notice = notice;
    this.error = error;
    this.menuOpen = this.navOpen = false;
    void this.load();
  }

  private async load() {
    const generation = ++this.generation;
    this.loading = true;
    const canvas = this.querySelector("canvas");
    if (canvas) destroyChart(canvas);
    this.route = parseRoute(location.href);
    try {
      try { this.user = (await request<{ user: User }>("/api/me")).user; }
      catch (error) {
        if (!(error instanceof RequestError) || error.status !== 401) throw error;
        this.user = null;
      }
      if (generation !== this.generation) return;
      if (!this.user && !["login", "registration", "password-request", "password-edit"].includes(this.route.name)) {
        this.returnTo = location.pathname + location.search;
        this.navigate("/session/new");
        return;
      }
      if (!this.user && this.route.name === "password-edit") {
        try {
          await request<{ valid: boolean }>(`/api/passwords/${encodeURIComponent(this.route.token!)}`);
        } catch (error) {
          if (error instanceof RequestError && error.status === 422) {
            this.navigate("/passwords/new", "", error.message);
            return;
          }
          throw error;
        }
      }
      if (this.user) {
        this.vehicles = (await request<{ vehicles: Vehicle[] }>("/api/vehicles")).vehicles;
        if (this.route.name === "dashboard") this.dashboard = await request<Dashboard>(`/api/dashboard${location.search}`);
        if (this.route.name === "vehicle-new") this.vehicle = { name: "", fuel_type: "petrol" };
        if (["vehicle-edit", "vehicle-show"].includes(this.route.name)) {
          this.vehicle = (await request<{ vehicle: Vehicle }>(`/api/vehicles/${this.route.id}`)).vehicle;
        }
        if (this.route.name === "refueling-new") {
          const defaults = await request<Refueling & { unit: string }>("/api/refuelings/new");
          if (!this.vehicles.length) { this.navigate("/vehicles/new", "", "Najprv vytvorte vozidlo."); return; }
          this.refueling = defaults;
          this.selectedUnit = defaults.unit;
        }
        if (this.route.name === "refueling-edit") {
          this.refueling = (await request<{ refueling: Refueling }>(`/api/refuelings/${this.route.id}`)).refueling;
          this.selectedUnit = this.vehicles.find((vehicle) => vehicle.id === this.refueling.vehicle_id)?.unit ?? "";
        }
        if (this.route.name === "cost-new") {
          const defaults = await request<{ vehicle_id: number | null; occurred_on: string }>("/api/additional_costs/new");
          if (!this.vehicles.length) { this.navigate("/vehicles/new", "", "Najprv vytvorte vozidlo."); return; }
          this.cost = { ...defaults, kind: "oil_change", comment: null, cost: "" };
        }
        if (this.route.name === "cost-edit") this.cost = (await request<{ additional_cost: Cost }>(`/api/additional_costs/${this.route.id}`)).additional_cost;
        if (this.route.name === "reminders") this.reminders = (await request<{ reminders: Reminder[] }>("/api/maintenance_reminders")).reminders;
        if (this.route.name === "reminder-new") this.reminder = { vehicle_id: this.vehicles[0]?.id ?? null, kind: "oil_change", interval_days: null, interval_km: null, active: true, leads: [{ days_before: null, kilometres_before: null }] };
        if (this.route.name === "reminder-edit") {
          this.reminder = (await request<{ reminder: Reminder }>(`/api/maintenance_reminders/${this.route.id}`)).reminder;
          if (!this.reminder.leads.length) this.reminder.leads = [{ days_before: null, kilometres_before: null }];
        }
      }
    } catch (error) {
      this.error = error instanceof Error ? error.message : "Požiadavka zlyhala.";
    } finally {
      if (generation === this.generation) this.loading = false;
    }
  }

  protected updated() {
    if (!this.loading && this.route.name === "dashboard" && this.dashboard) {
      const canvas = this.querySelector("canvas");
      if (canvas) renderChart(canvas, this.dashboard.chart);
    }
  }

  private async submit(event: SubmitEvent, action: (form: FormData) => Promise<void>) {
    event.preventDefault();
    this.error = "";
    try { await action(new FormData(event.currentTarget as HTMLFormElement)); }
    catch (error) { this.error = error instanceof Error ? error.message : "Požiadavka zlyhala."; }
  }

  private async login(form: FormData) {
    this.user = (await request<{ user: User }>("/api/session", "POST", Object.fromEntries(form))).user;
    this.navigate(this.returnTo);
  }

  private async register(form: FormData) {
    this.user = (await request<{ user: User }>("/api/registrations", "POST", Object.fromEntries(form))).user;
    this.navigate("/", "Vitajte! Váš účet bol vytvorený.");
  }

  private async saveVehicle(form: FormData) {
    const editing = this.route.name === "vehicle-edit";
    try {
      const { vehicle } = await request<{ vehicle: Vehicle }>(`/api/vehicles${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", Object.fromEntries(form));
      this.navigate(`/vehicles/${vehicle.id}`, editing ? "Vozidlo bolo úspešne upravené." : "Vozidlo bolo úspešne vytvorené.");
    } catch (error) {
      this.error = "Toto vozidlo sa nepodarilo uložiť:";
      throw new Error(this.error + " " + (error instanceof Error ? error.message : ""));
    }
  }

  private async saveRefueling(form: FormData) {
    const editing = this.route.name === "refueling-edit";
    const body = { ...Object.fromEntries(form), vehicle_id: Number(form.get("vehicle_id")) };
    try {
      await request(`/api/refuelings${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", body);
      this.navigate("/", editing ? "Tankovanie bolo úspešne upravené." : "Tankovanie bolo úspešne vytvorené.");
    } catch (error) {
      if (error instanceof Error && error.message.includes("iného používateľa")) {
        this.navigate("/", "", editing ? "Nemôžete upravovať tankovania iného používateľa." : error.message);
        return;
      }
      throw new Error("Toto tankovanie sa nepodarilo uložiť:");
    }
  }

  private async deleteVehicle(id: number) {
    if (!confirm("Vymazať toto vozidlo a všetky súvisiace tankovania?")) return;
    try { await request(`/api/vehicles/${id}`, "DELETE"); this.navigate("/vehicles", "Vozidlo bolo úspešne odstránené."); }
    catch (error) { this.error = (error as Error).message; }
  }

  private async resetRequest(form: FormData) {
    const result = await request<{ notice: string }>("/api/passwords", "POST", Object.fromEntries(form));
    this.navigate("/session/new", result.notice);
  }

  private async resetPassword(form: FormData) {
    const result = await request<{ notice: string }>(`/api/passwords/${encodeURIComponent(this.route.token!)}`, "POST", Object.fromEntries(form));
    this.navigate("/session/new", result.notice);
  }

  private async saveCost(form: FormData) {
    const editing = this.route.name === "cost-edit";
    try {
      await request(`/api/additional_costs${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", { ...Object.fromEntries(form), vehicle_id: Number(form.get("vehicle_id")) });
      this.navigate("/", editing ? "Dodatočný náklad bol úspešne upravený." : "Dodatočný náklad bol úspešne vytvorený.");
    } catch (error) {
      if (error instanceof Error && error.message.includes("iného používateľa")) { this.navigate("/", "", error.message); return; }
      throw new Error("Dodatočný náklad sa nepodarilo uložiť:");
    }
  }

  private async saveReminder(form: FormData) {
    const editing = this.route.name === "reminder-edit";
    const interval = String(form.get("interval_time_value") ?? "");
    const leads: Lead[] = [];
    for (let index = 0; index < this.reminder.leads.length; index++) {
      const old = this.reminder.leads[index];
      const time = String(form.get(`lead_time_${index}`) ?? "");
      const km = String(form.get(`lead_km_${index}`) ?? "");
      if (time) leads.push({ id: old.days_before ? old.id : undefined, days_before: daysFromTime(Number(time), String(form.get(`lead_unit_${index}`))), kilometres_before: null });
      if (km) leads.push({ id: old.kilometres_before ? old.id : undefined, days_before: null, kilometres_before: km });
    }
    const body = {
      vehicle_id: Number(form.get("vehicle_id")), kind: form.get("kind"),
      interval_days: interval ? daysFromTime(Number(interval), String(form.get("interval_time_unit"))) : null,
      interval_km: form.get("interval_km") || null, active: form.has("active"), leads,
    };
    try {
      await request(`/api/maintenance_reminders${editing ? `/${this.route.id}` : ""}`, editing ? "PATCH" : "POST", body);
      this.navigate("/maintenance_reminders", editing ? "Pripomienka údržby bola úspešne upravená." : "Pripomienka údržby bola úspešne vytvorená.");
    } catch (error) {
      if (error instanceof Error && error.message.includes("iného používateľa")) { this.navigate("/maintenance_reminders", "", error.message); return; }
      throw new Error("Pripomienku údržby sa nepodarilo uložiť:");
    }
  }

  private async deleteReminder(id: number) {
    if (!confirm("Vymazať túto pripomienku?")) return;
    try { await request(`/api/maintenance_reminders/${id}`, "DELETE"); this.navigate("/maintenance_reminders", "Pripomienka údržby bola úspešne odstránená."); }
    catch (error) { this.error = (error as Error).message; }
  }

  private async importData(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files?.length) return;
    const form = new FormData();
    form.set("file", input.files[0]);
    try {
      const response = await fetch("/api/user_data/import", { method: "POST", body: form, credentials: "same-origin" });
      const data = await response.json();
      this.navigate("/", response.ok ? data.notice : "", response.ok ? "" : data.error);
    } catch (error) { this.error = (error as Error).message; }
    finally { input.value = ""; }
  }

  private async deleteRecord(resource: "refuelings" | "additional_costs", id: number) {
    if (!confirm(resource === "refuelings" ? "Vymazať tento záznam tankovania?" : "Vymazať tento dodatočný náklad?")) return;
    try {
      await request(`/api/${resource}/${id}`, "DELETE");
      this.navigate("/", resource === "refuelings" ? "Tankovanie bolo úspešne odstránené." : "Dodatočný náklad bol úspešne odstránený.");
    } catch (error) { this.error = (error as Error).message; }
  }

  private icon(name: string) { return html`<img src=${`/icons/${name}.svg`} alt="" aria-hidden="true">`; }

  private renderHeader() {
    const links = [["/", "Prehľad", "home"], ["/refuelings/new", "Nové tankovanie", "fuel"], ["/additional_costs/new", "Dodatočné náklady", "receipt"], ["/maintenance_reminders", "Pripomienky", "bell"], ["/vehicles", "Vozidlá", "car"]];
    return html`<header class="topbar"><div class="container topbar-inner">
      ${this.user ? html`<button class="nav-toggle" type="button" aria-label="Otvoriť navigáciu" aria-expanded=${this.navOpen} @click=${() => { this.navOpen = !this.navOpen; }}>${this.icon("menu")}</button>` : nothing}
      <h1 class="brand"><a href="/">Spotreba</a></h1>
      <nav class=${`main-nav ${this.navOpen ? "is-visible is-expanded" : ""}`}>
        ${this.user ? html`${links.map(([href, label, icon]) => html`<a class="nav-link" href=${href} title=${label}>${this.icon(icon)}<span>${label}</span></a>`)}
          <div class="user-menu"><button type="button" class="link-button nav-link" title="Používateľ" aria-expanded=${this.menuOpen} @click=${() => { this.menuOpen = !this.menuOpen; }}>${this.icon("user")}<span>Používateľ</span></button>
          <div class="user-menu-panel" ?hidden=${!this.menuOpen}>
            <a class="nav-link" data-external href="/api/user_data/export" title="Exportovať JSON">${this.icon("download")}<span>Exportovať JSON</span></a>
            <input id="import-file" type="file" accept=".json,application/json" hidden aria-label="JSON súbor na import" @change=${this.importData}>
            <button type="button" class="link-button nav-link" title="Importovať JSON" @click=${() => (this.querySelector("#import-file") as HTMLInputElement).click()}>${this.icon("upload")}<span>Importovať JSON</span></button>
            <button type="button" class="link-button nav-link" title="Odhlásiť sa" @click=${async () => { await request("/api/session", "DELETE"); this.user = null; this.navigate("/session/new"); }}>${this.icon("logout")}<span>Odhlásiť sa</span></button>
          </div></div>` : html`<a href="/session/new">Prihlásiť sa</a><a href="/sign_up">Vytvoriť účet</a>`}
      </nav></div></header>`;
  }

  private renderLogin() {
    return html`<section class="card"><h2>Prihlásiť sa</h2><form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.login(form))}>
      <div class="field"><label for="email_address">E-mail</label><input id="email_address" name="email_address" type="email" required autocomplete="username"></div>
      <div class="field"><label for="password">Heslo</label><input id="password" name="password" type="password" required maxlength="72" autocomplete="current-password"></div>
      <div class="actions"><button type="submit">Prihlásiť sa</button><a class="button secondary" href="/sign_up">Vytvoriť účet</a><a class="button secondary" href="/passwords/new">Zabudli ste heslo?</a></div>
    </form></section>`;
  }

  private renderRegistration() {
    return html`<section class="card"><h2>Vytvoriť účet</h2>
      ${this.error ? html`<div class="errors"><strong>Pri vytváraní účtu sa vyskytli chyby:</strong></div>` : nothing}
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.register(form))}>
      <div class="field"><label for="email_address">E-mail</label><input id="email_address" name="email_address" type="email" required autocomplete="username"></div>
      <div class="field"><label for="password">Heslo</label><input id="password" name="password" type="password" required maxlength="72" autocomplete="new-password"></div>
      <div class="field"><label for="password_confirmation">Potvrdenie hesla</label><input id="password_confirmation" name="password_confirmation" type="password" required maxlength="72" autocomplete="new-password"></div>
      <div class="actions"><button type="submit">Vytvoriť účet</button><a class="button secondary" href="/session/new">Už máte účet? Prihláste sa.</a></div></form></section>`;
  }

  private renderPasswordForm() {
    const editing = this.route.name === "password-edit";
    return html`<section class="card"><h1>${editing ? "Obnoviť heslo" : "Zabudli ste heslo?"}</h1>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => editing ? this.resetPassword(form) : this.resetRequest(form))}>
        ${editing ? html`<div class="field"><input name="password" type="password" required maxlength="72" autocomplete="new-password" placeholder="Zadajte nové heslo" aria-label="Zadajte nové heslo"></div><div class="field"><input name="password_confirmation" type="password" required maxlength="72" autocomplete="new-password" placeholder="Zopakujte nové heslo" aria-label="Zopakujte nové heslo"></div>` : html`<div class="field"><input name="email_address" type="email" required autocomplete="username" placeholder="Zadajte svoju e-mailovú adresu" aria-label="Zadajte svoju e-mailovú adresu"></div>`}
        <button type="submit">${editing ? "Uložiť nové heslo" : "Odoslať pokyny na obnovenie hesla"}</button>
      </form></section>`;
  }

  private renderVehicles() {
    return html`<section class="card"><div class="actions"><h2>Vozidlá</h2><a class="button" href="/vehicles/new">Nové vozidlo</a></div>
      ${!this.vehicles.length ? html`<p class="muted">Zatiaľ žiadne vozidlá.</p>` : html`<table><thead><tr><th>Názov</th><th>Typ paliva</th><th>Jednotka</th><th>Akcie</th></tr></thead><tbody>${this.vehicles.map((vehicle) => html`<tr><td>${vehicle.name}</td><td>${vehicle.fuel_type_label}</td><td>${vehicle.unit}</td><td class="actions"><a class="button secondary" href=${`/vehicles/${vehicle.id}`}>Zobraziť</a><a class="button secondary" href=${`/vehicles/${vehicle.id}/edit`}>Upraviť</a><button class="danger" @click=${() => this.deleteVehicle(vehicle.id)}>Vymazať</button></td></tr>`)}</tbody></table>`}
    </section>`;
  }

  private renderVehicleForm() {
    return html`<section class="card"><h2>${this.route.name === "vehicle-new" ? "Nové vozidlo" : "Upraviť vozidlo"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.saveVehicle(form))}>
        <div class="field"><label for="name">Názov vozidla</label><input id="name" name="name" .value=${this.vehicle.name ?? ""}></div>
        <div class="field"><label for="fuel_type">Typ paliva</label><select id="fuel_type" name="fuel_type"><option value="petrol" ?selected=${this.vehicle.fuel_type === "petrol"}>Benzín</option><option value="diesel" ?selected=${this.vehicle.fuel_type === "diesel"}>Nafta</option><option value="electricity" ?selected=${this.vehicle.fuel_type === "electricity"}>Elektrina</option></select></div>
        <div class="actions"><button type="submit">Uložiť</button><a class="button secondary" href="/vehicles">Späť</a></div>
      </form></section>`;
  }

  private renderVehicle() {
    return html`<section class="card"><h2>${this.vehicle.name}</h2><p><strong>Typ paliva:</strong> ${this.vehicle.fuel_type_label}</p><p><strong>Jednotka:</strong> ${this.vehicle.unit}</p><div class="actions"><a class="button secondary" href=${`/vehicles/${this.vehicle.id}/edit`}>Upraviť</a><a class="button secondary" href="/vehicles">Späť</a><button class="danger" @click=${() => this.deleteVehicle(this.vehicle.id!)}>Vymazať</button></div></section>`;
  }

  private vehicleSelect(selected: number | null) {
    return html`<div class="field"><label for="vehicle_id">Vozidlo</label><select id="vehicle_id" name="vehicle_id" @change=${(event: Event) => {
      this.selectedUnit = this.vehicles.find((vehicle) => vehicle.id === Number((event.target as HTMLSelectElement).value))?.unit ?? "";
    }}>${this.vehicles.map((vehicle) => html`<option value=${vehicle.id} ?selected=${vehicle.id === selected}>${vehicle.name}</option>`)}</select></div>`;
  }

  private renderRefuelingForm() {
    return html`<section class="card"><h2>${this.route.name === "refueling-new" ? "Nové tankovanie" : "Upraviť tankovanie"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.saveRefueling(form))}>
        ${this.vehicleSelect(this.refueling.vehicle_id)}<div class="field"><label>Jednotka</label><p data-unit class="muted">${this.selectedUnit}</p></div>
        <div class="field"><label for="refueled_on">Dátum tankovania</label><input id="refueled_on" name="refueled_on" type="date" .value=${this.refueling.refueled_on.slice(0, 10)}></div>
        <div class="field"><label for="distance_km">Vzdialenosť od posledného tankovania (km)</label><input id="distance_km" name="distance_km" type="number" step="0.01" min="0.01" .value=${this.refueling.distance_km ?? ""}></div>
        <div class="field"><label for="amount">Množstvo paliva do plnej nádrže</label><input id="amount" name="amount" type="number" step="0.01" min="0.01" .value=${this.refueling.amount ?? ""}></div>
        <div class="field"><label for="cost">Náklady na palivo</label><input id="cost" name="cost" type="number" step="0.01" min="0.01" .value=${this.refueling.cost ?? ""}></div>
        <div class="actions"><button type="submit">Uložiť</button><a class="button secondary" href="/">Späť</a></div>
      </form></section>`;
  }

  private renderCostForm() {
    return html`<section class="card"><h2>${this.route.name === "cost-new" ? "Nový dodatočný náklad" : "Upraviť dodatočný náklad"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.saveCost(form))}>
        ${this.vehicleSelect(this.cost.vehicle_id)}
        <div class="field"><label for="occurred_on">Dátum nákladu</label><input id="occurred_on" name="occurred_on" type="date" .value=${this.cost.occurred_on.slice(0, 10)}></div>
        <div class="field"><label for="kind">Typ nákladu</label><select id="kind" name="kind">${kinds.map(([value, label]) => html`<option value=${value} ?selected=${value === this.cost.kind}>${label}</option>`)}</select></div>
        <div class="field"><label for="comment">Poznámka</label><input id="comment" name="comment" maxlength="255" .value=${this.cost.comment ?? ""}></div>
        <div class="field"><label for="cost">Náklady</label><input id="cost" name="cost" type="number" step="0.01" min="0.01" .value=${this.cost.cost}></div>
        <div class="actions"><button type="submit">Uložiť</button><a class="button secondary" href="/">Späť</a></div>
      </form></section>`;
  }

  private timeUnits(name: string, value = "days") {
    return html`<select name=${name} aria-label="Časová jednotka" .value=${value}><option value="days">Dni</option><option value="weeks">Týždne</option><option value="months">Mesiace</option><option value="years">Roky</option></select>`;
  }

  private captureReminderForm() {
    const form = this.querySelector("main form") as HTMLFormElement | null;
    if (!form) return;
    const values = new FormData(form);
    this.reminder = { ...this.reminder, vehicle_id: Number(values.get("vehicle_id")), kind: String(values.get("kind")), active: values.has("active"), interval_time_value: String(values.get("interval_time_value") ?? ""), interval_time_unit: String(values.get("interval_time_unit") ?? "days"), interval_days: values.get("interval_time_value") ? daysFromTime(Number(values.get("interval_time_value")), String(values.get("interval_time_unit"))) : null, interval_km: String(values.get("interval_km") ?? "") || null,
      leads: this.reminder.leads.map((lead, index) => ({ ...lead, time_value: String(values.get(`lead_time_${index}`) ?? ""), time_unit: String(values.get(`lead_unit_${index}`) ?? "days"), days_before: values.get(`lead_time_${index}`) ? daysFromTime(Number(values.get(`lead_time_${index}`)), String(values.get(`lead_unit_${index}`))) : null, kilometres_before: String(values.get(`lead_km_${index}`) ?? "") || null })),
    };
  }

  private renderReminderForm() {
    return html`<section class="card"><h2>${this.route.name === "reminder-new" ? "Nová pripomienka" : "Upraviť pripomienku"}</h2>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.saveReminder(form))}>
        <div class="grid grid-2">${this.vehicleSelect(this.reminder.vehicle_id)}<div class="field"><label for="kind">Typ údržby</label><select id="kind" name="kind">${kinds.map(([value, label]) => html`<option value=${value} ?selected=${value === this.reminder.kind}>${label}</option>`)}</select></div></div>
        <div class="grid grid-2"><div class="field"><label for="interval_time_value">Časový interval</label><div class="input-with-unit"><input id="interval_time_value" name="interval_time_value" type="number" min="1" step="1" .value=${this.reminder.interval_time_value ?? this.reminder.interval_days?.toString() ?? ""}>${this.timeUnits("interval_time_unit", this.reminder.interval_time_unit)}</div></div>
        <div class="field"><label for="interval_km">Interval v kilometroch</label><input id="interval_km" name="interval_km" type="number" min="0.01" step="0.01" .value=${this.reminder.interval_km ?? ""}></div></div>
        <div class="checkbox-field"><input id="active" name="active" type="checkbox" ?checked=${this.reminder.active}><label for="active">Aktívna pripomienka</label></div>
        <fieldset class="reminder-leads"><legend>Pokročilé pripomienky</legend>
          ${this.reminder.leads.map((lead, index) => html`<div class="reminder-lead"><div class="grid grid-2"><div class="field"><label for=${`lead_time_${index}`}>Čas vopred</label><div class="input-with-unit"><input id=${`lead_time_${index}`} name=${`lead_time_${index}`} type="number" min="1" step="1" .value=${lead.time_value ?? lead.days_before?.toString() ?? ""}>${this.timeUnits(`lead_unit_${index}`, lead.time_unit)}</div></div><div class="field"><label for=${`lead_km_${index}`}>Kilometrov vopred</label><input id=${`lead_km_${index}`} name=${`lead_km_${index}`} type="number" min="0.01" step="0.01" .value=${lead.kilometres_before ?? ""}></div></div><button type="button" class="secondary" @click=${() => { this.captureReminderForm(); this.reminder = { ...this.reminder, leads: this.reminder.leads.filter((_, position) => position !== index) }; }}>Odstrániť</button></div>`)}
          <button type="button" class="secondary" @click=${() => { this.captureReminderForm(); this.reminder = { ...this.reminder, leads: [...this.reminder.leads, { days_before: null, kilometres_before: null }] }; }}>Pridať pripomienku</button>
        </fieldset><div class="actions"><button type="submit">Uložiť</button><a class="button secondary" href="/maintenance_reminders">Späť</a></div>
      </form></section>`;
  }

  private renderReminders() {
    return html`<section class="card"><div class="section-heading"><h2>Pripomienky údržby</h2><a class="icon-button" href="/maintenance_reminders/new" aria-label="Nová pripomienka" title="Nová pripomienka">${this.icon("plus")}</a></div>
      ${!this.reminders.length ? html`<p class="muted">Zatiaľ nemáte nastavené žiadne pripomienky údržby.</p>` : html`<table><thead><tr><th>Vozidlo</th><th>Údržba</th><th>Limit</th><th>Stav</th><th>Akcie</th></tr></thead><tbody>${this.reminders.map((reminder) => html`<tr><td>${reminder.vehicle_name}</td><td>${reminder.kind_label}</td><td>${[reminder.interval_days ? `${reminder.interval_days} dní` : "", reminder.interval_km ? `${Number(reminder.interval_km)} km` : ""].filter(Boolean).join(" / ")}</td><td>${reminder.status}</td><td class="actions"><a class="button secondary" href=${`/maintenance_reminders/${reminder.id}/edit`}>Upraviť</a><button class="danger" @click=${() => this.deleteReminder(reminder.id!)}>Vymazať</button></td></tr>`)}</tbody></table>`}
    </section>`;
  }

  private dashboardURL(overrides: Record<string, string | number>) {
    const data = this.dashboard!;
    const params = new URLSearchParams({ scope: data.scope, offset: String(data.offset), ...(data.selected_vehicle_id ? { vehicle_id: String(data.selected_vehicle_id) } : {}), ...Object.fromEntries(Object.entries(overrides).map(([key, value]) => [key, String(value)])) });
    return `/?${params}`;
  }

  private actionLink(href: string, label: string, icon: string, secondary = false) {
    return html`<a class=${`icon-button${secondary ? " secondary" : ""}`} href=${href} aria-label=${label} title=${label}>${this.icon(icon)}<span class="sr-only">${label}</span></a>`;
  }

  private renderDashboard() {
    const data = this.dashboard;
    if (!data) return nothing;
    const stats = data.stats;
    const selected = this.vehicles.find((vehicle) => vehicle.id === data.selected_vehicle_id);
    const cards: [string, string | number][] = [
      ["Dni od posledného tankovania", stats.days_since_last_refueling ?? "—"],
      ["Priemerná spotreba / 100 km", stats.average_consumption_per_100km === null ? "—" : formatDecimal(stats.average_consumption_per_100km, 1)],
      ["Celkové náklady", formatCurrency(stats.total_cost_paid)],
      ["Priemerný počet dní medzi tankovaniami", stats.average_days_between_refuelings === null ? "—" : formatDecimal(stats.average_days_between_refuelings, 1)],
    ];
    return html`<section class="card"><h3>${data.chart.title}</h3>
      ${data.chart.labels.length ? html`<div class="chart-wrap"><canvas aria-label="Graf spotreby paliva"></canvas></div>${data.chart.additional_costs.length ? html`<p class="muted">Zvislé prerušované čiary označujú dodatočné náklady.</p>` : nothing}` : html`<p class="muted">V tomto období nie sú žiadne údaje o tankovaní ani dodatočných nákladoch.</p>`}
    </section><section class="grid grid-4">${cards.map(([label, value]) => html`<article class="card"><h3>${label}</h3><p class="stat">${value}</p></article>`)}</section>
    <section class="card"><div class="section-heading"><h3>Záznamy tankovaní: ${formatDecimal(stats.total_fuel_refilled, 2)} ${data.scope === "combined" ? "zmiešané jednotky" : selected?.unit ?? "L"} / ${formatCurrency(stats.total_fuel_cost_paid)}</h3>${this.actionLink("/refuelings/new", "Pridať tankovanie", "plus")}</div>
      ${!data.refuelings.items.length ? html`<p class="muted">Zatiaľ tu nie sú žiadne záznamy tankovaní.</p>` : html`<table><thead><tr><th>Dátum</th><th>Vozidlo</th><th>Vzdialenosť (km)</th><th>Množstvo</th><th>Náklady</th><th>Spotreba / 100 km</th><th>Náklady / 100 km</th><th>Akcie</th></tr></thead><tbody>${data.refuelings.items.map((row) => html`<tr><td data-label="Dátum">${row.refueled_on.slice(0, 10)}</td><td data-label="Vozidlo">${row.vehicle_name}</td><td data-label="Vzdialenosť (km)">${formatDecimal(row.distance_km!, 2)}</td><td data-label="Množstvo">${formatDecimal(row.amount!, 2)} ${row.unit}</td><td data-label="Náklady">${formatDecimal(row.cost!, 2)}</td><td data-label="Spotreba / 100 km">${formatDecimal(row.consumption_per_100km, 2)}</td><td data-label="Náklady / 100 km">${formatDecimal(row.cost_per_100km, 2)}</td><td class="actions" data-label="Akcie">${this.actionLink(`/refuelings/${row.id}/edit`, "Upraviť tankovanie", "edit", true)}<button class="icon-button danger" aria-label="Vymazať tankovanie" title="Vymazať tankovanie" @click=${() => this.deleteRecord("refuelings", row.id)}>${this.icon("trash")}</button></td></tr>`)}</tbody></table>
      <div class="section-heading"><p class="muted">Zobrazuje sa ${data.refuelings.items.length} z ${data.refuelings.total_count} záznamov</p><div class="actions">${data.refuelings.page > 1 ? this.actionLink(this.dashboardURL({ table_page: data.refuelings.page - 1 }), "Predchádzajúca strana", "chevron-left", true) : nothing}<span class="muted">Strana ${data.refuelings.page} / ${data.refuelings.total_pages}</span>${data.refuelings.page < data.refuelings.total_pages ? this.actionLink(this.dashboardURL({ table_page: data.refuelings.page + 1 }), "Nasledujúca strana", "chevron-right", true) : nothing}</div></div>`}
    </section>
    <section class="card"><div class="section-heading"><h3>Dodatočné náklady: ${formatCurrency(stats.total_additional_cost_paid)}</h3>${this.actionLink("/additional_costs/new", "Pridať dodatočný náklad", "plus")}</div>
      ${!data.additional_costs.length ? html`<p class="muted">Zatiaľ tu nie sú žiadne dodatočné náklady.</p>` : html`<table><thead><tr><th>Dátum</th><th>Vozidlo</th><th>Typ</th><th>Poznámka</th><th>Náklady</th><th>Akcie</th></tr></thead><tbody>${data.additional_costs.map((row) => html`<tr><td data-label="Dátum">${row.occurred_on.slice(0, 10)}</td><td data-label="Vozidlo">${row.vehicle_name}</td><td data-label="Typ">${row.kind_label}</td><td data-label="Poznámka">${row.comment}</td><td data-label="Náklady">${formatCurrency(row.cost)}</td><td data-label="Akcie" class="actions">${this.actionLink(`/additional_costs/${row.id}/edit`, "Upraviť dodatočný náklad", "edit", true)}<button class="icon-button danger" aria-label="Vymazať dodatočný náklad" title="Vymazať dodatočný náklad" @click=${() => this.deleteRecord("additional_costs", row.id)}>${this.icon("trash")}</button></td></tr>`)}</tbody></table>`}
    </section>
    <section class="card"><h2>Prehľad spotreby paliva</h2><form @submit=${(event: SubmitEvent) => { event.preventDefault(); const values = new FormData(event.currentTarget as HTMLFormElement); const params = new URLSearchParams({ ...Object.fromEntries(values) as Record<string, string>, offset: String(data.offset) }); this.navigate(`/?${params}`); }}><div class="grid grid-3">
      <div class="field"><label for="scope">Rozsah grafu</label><select id="scope" name="scope"><option value="vehicle" ?selected=${data.scope === "vehicle"}>Podľa vozidla</option><option value="combined" ?selected=${data.scope === "combined"}>Spolu</option></select></div>
      <div class="field"><label for="dashboard_vehicle">Vozidlo (v režime podľa vozidla)</label><select id="dashboard_vehicle" name="vehicle_id" ?disabled=${!this.vehicles.length}>${this.vehicles.length ? this.vehicles.map((vehicle) => html`<option value=${vehicle.id} ?selected=${vehicle.id === data.selected_vehicle_id}>${vehicle.name}</option>`) : html`<option>Zatiaľ žiadne vozidlá</option>`}</select></div>
      <div class="field actions"><button type="submit">Použiť</button></div></div></form><div class="actions"><a class="button secondary" href=${this.dashboardURL({ offset: data.offset + 365 })}>Predchádzajúcich 365 dní</a>${data.offset > 0 ? html`<a class="button secondary" href=${this.dashboardURL({ offset: Math.max(data.offset - 365, 0) })}>Nasledujúcich 365 dní</a>` : nothing}</div>
    </section>`;
  }

  private renderPage() {
    switch (this.route.name) {
      case "login": return this.renderLogin();
      case "registration": return this.renderRegistration();
      case "password-request": case "password-edit": return this.renderPasswordForm();
      case "vehicles": return this.renderVehicles();
      case "vehicle-new": case "vehicle-edit": return this.renderVehicleForm();
      case "vehicle-show": return this.renderVehicle();
      case "refueling-new": case "refueling-edit": return this.renderRefuelingForm();
      case "cost-new": case "cost-edit": return this.renderCostForm();
      case "reminders": return this.renderReminders();
      case "reminder-new": case "reminder-edit": return this.renderReminderForm();
      default: return this.renderDashboard();
    }
  }

  render() {
    return html`${this.renderHeader()}<main class="container page">${this.notice ? html`<p class="flash flash-notice">${this.notice}</p>` : nothing}${this.error ? html`<p class="flash flash-alert">${this.error}</p>` : nothing}${this.loading ? nothing : this.renderPage()}</main>`;
  }
}

customElements.define("spotreba-app", SpotrebaApp);