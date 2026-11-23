import { LitElement, nothing, type PropertyDeclarations, type TemplateResult } from "lit";
import { messageOf, request } from "../lib/api";
import { emit, type Redirect, type SpotrebaEvents } from "../lib/events";
import type { Removal } from "../lib/removals";
import type { ParsedRoute } from "../lib/view";

export type { Redirect };

// One screen of the app. The app shell creates a fresh page for every visit, hands it its route and
// inserts it; the page loads what it needs, renders itself and talks back to the shell with events.
export abstract class SpotrebaPage extends LitElement {
  // Typed loosely so that pages can declare their own reactive state; Lit merges the declarations of every class in the chain.
  static properties: PropertyDeclarations = { route: { attribute: false }, ready: { state: true } };

  declare route: ParsedRoute;
  declare ready: boolean;
  private started = false;

  constructor() {
    super();
    this.ready = false;
  }

  createRenderRoot() { return this; }

  connectedCallback() {
    super.connectedCallback();
    // The page element only wraps its content; see the .route-view rule in the stylesheet.
    this.classList.add("route-view");
    if (this.started) return;
    this.started = true;
    void this.start();
  }

  /** Fetches the data the page needs. Resolve with a redirect to leave instead of showing the page. */
  protected async load(): Promise<void | Redirect> {}

  protected abstract renderPage(): TemplateResult;

  render() {
    return this.ready ? this.renderPage() : nothing;
  }

  private async start() {
    try {
      const redirect = await this.load();
      // The user has already moved on; whatever this page found out is no longer of interest.
      if (!this.isConnected) return;
      if (redirect) this.navigate(redirect);
      else this.ready = true;
    } catch (error) {
      if (this.isConnected) this.flash({ error: messageOf(error) });
    }
  }

  protected navigate(destination: Redirect) {
    emit(this, "spotreba:navigate", destination);
  }

  protected flash(messages: SpotrebaEvents["spotreba:flash"]) {
    emit(this, "spotreba:flash", messages);
  }

  /** Runs a form's action with its data. A failure is shown as a flash error; resolves to whether it succeeded. */
  protected async submit(event: SubmitEvent, action: (form: FormData) => Promise<void>): Promise<boolean> {
    event.preventDefault();
    this.flash({ error: "" });
    try {
      await action(new FormData(event.currentTarget as HTMLFormElement));
      return true;
    } catch (error) {
      this.flash({ error: messageOf(error) });
      return false;
    }
  }

  /**
   * Saves a record. A record that belongs to another user sends the visitor on to `next` with the
   * server's explanation (or `foreign`); any other failure is reported as `failure`.
   */
  protected async persist(save: () => Promise<unknown>, outcome: { next: string; notice: string; failure: string; foreign?: string }) {
    try {
      await save();
    } catch (error) {
      if (error instanceof Error && error.message.includes("iného používateľa")) {
        this.navigate({ path: outcome.next, error: outcome.foreign ?? error.message });
        return;
      }
      throw new Error(outcome.failure);
    }
    this.navigate({ path: outcome.next, notice: outcome.notice });
  }

  protected async deleteRecord({ url, confirmation, next }: Removal) {
    if (!confirm(confirmation)) return;
    try {
      await request(url, "DELETE");
      this.navigate(next);
    } catch (error) {
      this.flash({ error: messageOf(error) });
    }
  }
}
