import { html, nothing } from "lit";
import { request } from "../lib/api";
import { emit } from "../lib/events";
import type { User } from "../lib/types";
import { glyph } from "../components/icons";
import { inputField } from "../components/form-fields";
import { SpotrebaPage } from "./page";

export class RegistrationPage extends SpotrebaPage {
  static properties = { failed: { state: true } };

  declare failed: boolean;

  constructor() {
    super();
    this.failed = false;
  }

  private async register(form: FormData) {
    const { user } = await request<{ user: User }>("/api/registrations", "POST", Object.fromEntries(form));
    emit(this, "spotreba:signed-in", { user, path: "/", notice: "Vitajte! Váš účet bol vytvorený." });
  }

  private async onSubmit(event: SubmitEvent) {
    this.failed = false;
    this.failed = !(await this.submit(event, (form) => this.register(form)));
  }

  protected renderPage() {
    return html`<section class="card"><h2>Vytvoriť účet</h2>
      ${this.failed ? html`<div class="errors"><strong>Pri vytváraní účtu sa vyskytli chyby:</strong></div>` : nothing}
      <form @submit=${this.onSubmit}>
      ${inputField({ id: "email_address", label: "E-mail", type: "email", required: true, autocomplete: "username" })}
      ${inputField({ id: "password", label: "Heslo", type: "password", required: true, maxlength: 72, autocomplete: "new-password" })}
      ${inputField({ id: "password_confirmation", label: "Potvrdenie hesla", type: "password", required: true, maxlength: 72, autocomplete: "new-password" })}
      <div class="actions"><button type="submit">${glyph("user-plus")}Vytvoriť účet</button><a href="/session/new">Už máte účet? Prihláste sa.</a></div></form></section>`;
  }
}

customElements.define("spotreba-registration-page", RegistrationPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-registration-page": RegistrationPage }
}
