import { html } from "lit";
import { request } from "../lib/api";
import { glyph } from "../components/icons";
import { SpotrebaPage } from "./page";

export class PasswordRequestPage extends SpotrebaPage {
  private async requestReset(form: FormData) {
    const result = await request<{ notice: string }>("/api/passwords", "POST", Object.fromEntries(form));
    this.navigate({ path: "/session/new", notice: result.notice });
  }

  protected renderPage() {
    return html`<section class="card"><h1>Zabudli ste heslo?</h1>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.requestReset(form))}>
        <div class="field"><input name="email_address" type="email" required autocomplete="username" placeholder="Zadajte svoju e-mailovú adresu" aria-label="Zadajte svoju e-mailovú adresu"></div>
        <button type="submit">${glyph("mail")}Odoslať pokyny na obnovenie hesla</button>
      </form></section>`;
  }
}

customElements.define("spotreba-password-request-page", PasswordRequestPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-password-request-page": PasswordRequestPage }
}
