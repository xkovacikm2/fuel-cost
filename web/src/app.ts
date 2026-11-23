import { LitElement, html, nothing } from "lit";
import "./components/app-header";
import { RequestError, messageOf, request } from "./lib/api";
import type { Redirect, SpotrebaEvents } from "./lib/events";
import type { User } from "./lib/types";
import { parseRoute, type ParsedRoute } from "./lib/view";
import { pages } from "./pages";
import type { SpotrebaPage } from "./pages/page";

// The shell around the screens: it knows who is signed in, which route is open and which flash messages
// to show. It puts a fresh page element for the current route in <main> and reacts to the events the
// header and the pages send (navigate, flash, signed-in, signed-out).
export class SpotrebaApp extends LitElement {
  static properties = {
    user: { state: true }, route: { state: true }, notice: { state: true }, error: { state: true }, page: { state: true },
  };

  declare user: User | null;
  declare route: ParsedRoute;
  declare notice: string;
  declare error: string;
  declare page: SpotrebaPage | null;
  private returnTo = "/";
  private generation = 0;
  private onPopState = () => { void this.load(); };

  constructor() {
    super();
    this.user = null;
    this.route = parseRoute(location.href);
    this.notice = this.error = "";
    this.page = null;
    this.addEventListener("click", this.onLink);
    this.addEventListener("spotreba:navigate", (event) => this.navigate(event.detail));
    this.addEventListener("spotreba:flash", (event) => this.flash(event.detail));
    this.addEventListener("spotreba:signed-in", (event) => this.signedIn(event.detail));
    this.addEventListener("spotreba:signed-out", () => this.signedOut());
  }

  createRenderRoot() { return this; }

  connectedCallback() {
    super.connectedCallback();
    window.addEventListener("popstate", this.onPopState);
    void this.load();
  }

  disconnectedCallback() {
    window.removeEventListener("popstate", this.onPopState);
    super.disconnectedCallback();
  }

  private onLink = (event: MouseEvent) => {
    const anchor = (event.target as Element).closest("a");
    if (!anchor || anchor.hasAttribute("download") || anchor.dataset.external !== undefined || event.ctrlKey || event.metaKey || event.shiftKey || event.button !== 0) return;
    const url = new URL(anchor.href);
    if (url.origin !== location.origin) return;
    event.preventDefault();
    this.navigate({ path: url.pathname + url.search });
  };

  private navigate({ path, notice = "", error = "" }: Redirect) {
    history.pushState(null, "", path);
    this.notice = notice;
    this.error = error;
    void this.load();
  }

  private flash({ notice, error }: SpotrebaEvents["spotreba:flash"]) {
    if (notice !== undefined) this.notice = notice;
    if (error !== undefined) this.error = error;
  }

  private signedIn({ user, path, notice }: SpotrebaEvents["spotreba:signed-in"]) {
    this.user = user;
    this.navigate({ path: path ?? this.returnTo, notice });
  }

  private signedOut() {
    this.user = null;
    this.navigate({ path: "/session/new" });
  }

  private async currentUser() {
    try {
      return (await request<{ user: User }>("/api/me")).user;
    } catch (error) {
      if (error instanceof RequestError && error.status === 401) return null;
      throw error;
    }
  }

  private async load() {
    const generation = ++this.generation;
    this.page = null;
    this.route = parseRoute(location.href);
    try {
      this.user = await this.currentUser();
      if (generation !== this.generation) return;
      const entry = pages[this.route.name];
      if (!this.user && !entry.public) {
        this.returnTo = location.pathname + location.search;
        this.navigate({ path: "/session/new" });
        return;
      }
      const page = new entry.page();
      page.route = this.route;
      this.page = page;
    } catch (error) {
      if (generation === this.generation) this.error = messageOf(error);
    }
  }

  render() {
    return html`<spotreba-header .user=${this.user} .route=${this.route}></spotreba-header><main class="container page">${this.notice ? html`<p class="flash flash-notice">${this.notice}</p>` : nothing}${this.error ? html`<p class="flash flash-alert">${this.error}</p>` : nothing}${this.page}</main>`;
  }
}

customElements.define("spotreba-app", SpotrebaApp);
