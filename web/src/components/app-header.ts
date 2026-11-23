import { LitElement, html, nothing, type PropertyValues } from "lit";
import { messageOf, request } from "../lib/api";
import { emit } from "../lib/events";
import type { User } from "../lib/types";
import type { ParsedRoute } from "../lib/view";
import { icon } from "./icons";

const sections = [
  ["/", "Prehľad", "home"],
  ["/refuelings/new", "Nové tankovanie", "fuel"],
  ["/additional_costs/new", "Dodatočné náklady", "receipt"],
  ["/maintenance_reminders", "Pripomienky", "bell"],
  ["/vehicles", "Vozidlá", "car"],
] as const;

// The top bar: brand, section links and the user menu. It owns its open/closed state and reports
// sign-out and data import to the app shell as events.
export class AppHeader extends LitElement {
  static properties = {
    user: { attribute: false }, route: { attribute: false },
    navOpen: { state: true }, menuOpen: { state: true },
  };

  declare user: User | null;
  declare route: ParsedRoute;
  declare navOpen: boolean;
  declare menuOpen: boolean;

  constructor() {
    super();
    this.user = null;
    this.navOpen = this.menuOpen = false;
  }

  createRenderRoot() { return this; }

  // Following a link, or going back, always produces a new route object.
  protected willUpdate(changed: PropertyValues<this>) {
    if (changed.has("route")) this.navOpen = this.menuOpen = false;
  }

  private async signOut() {
    try {
      await request("/api/session", "DELETE");
      emit(this, "spotreba:signed-out");
    } catch (error) {
      emit(this, "spotreba:flash", { error: messageOf(error) });
    }
  }

  private async importData(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files?.length) return;
    const form = new FormData();
    form.set("file", input.files[0]);
    try {
      const response = await fetch("/api/user_data/import", { method: "POST", body: form, credentials: "same-origin" });
      const data = await response.json();
      emit(this, "spotreba:navigate", { path: "/", notice: response.ok ? data.notice : "", error: response.ok ? "" : data.error });
    } catch (error) {
      emit(this, "spotreba:flash", { error: messageOf(error) });
    } finally {
      input.value = "";
    }
  }

  private renderMemberNav() {
    return html`${sections.map(([href, label, name]) => html`<a class="nav-link" href=${href} title=${label}>${icon(name)}<span>${label}</span></a>`)}
      <div class="user-menu"><button type="button" class="link-button nav-link" title="Používateľ" aria-expanded=${this.menuOpen} @click=${() => { this.menuOpen = !this.menuOpen; }}>${icon("user")}<span>Používateľ</span></button>
      <div class="user-menu-panel" ?hidden=${!this.menuOpen}>
        <a class="nav-link" data-external href="/api/user_data/export" title="Exportovať JSON">${icon("download")}<span>Exportovať JSON</span></a>
        <input id="import-file" type="file" accept=".json,application/json" hidden aria-label="JSON súbor na import" @change=${this.importData}>
        <button type="button" class="link-button nav-link" title="Importovať JSON" @click=${() => this.querySelector<HTMLInputElement>("#import-file")!.click()}>${icon("upload")}<span>Importovať JSON</span></button>
        <button type="button" class="link-button nav-link" title="Odhlásiť sa" @click=${this.signOut}>${icon("logout")}<span>Odhlásiť sa</span></button>
      </div></div>`;
  }

  render() {
    return html`<header class="topbar"><div class="container topbar-inner">
      ${this.user ? html`<button class="nav-toggle" type="button" aria-label="Otvoriť navigáciu" aria-expanded=${this.navOpen} @click=${() => { this.navOpen = !this.navOpen; }}>${icon("menu")}</button>` : nothing}
      <h1 class="brand"><a href="/">Spotreba</a></h1>
      <nav class=${`main-nav ${this.navOpen ? "is-visible is-expanded" : ""}`}>
        ${this.user ? this.renderMemberNav() : html`<a href="/session/new">Prihlásiť sa</a><a href="/sign_up">Vytvoriť účet</a>`}
      </nav></div></header>`;
  }
}

customElements.define("spotreba-header", AppHeader);

declare global {
  interface HTMLElementTagNameMap { "spotreba-header": AppHeader }
}
