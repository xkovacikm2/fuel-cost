import { html } from "lit";
import { request } from "../lib/api";
import { emit } from "../lib/events";
import type { User } from "../lib/types";
import { glyph } from "../components/icons";
import { inputField } from "../components/form-fields";
import { SpotrebaPage } from "./page";

export class LoginPage extends SpotrebaPage {
  private async signIn(form: FormData) {
    const { user } = await request<{ user: User }>("/api/session", "POST", Object.fromEntries(form));
    emit(this, "spotreba:signed-in", { user });
  }

  protected renderPage() {
    return html`<section class="card"><h2>Prihlásiť sa</h2><form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.signIn(form))}>
      ${inputField({ id: "email_address", label: "E-mail", type: "email", required: true, autocomplete: "username" })}
      ${inputField({ id: "password", label: "Heslo", type: "password", required: true, maxlength: 72, autocomplete: "current-password" })}
      <div class="actions"><button type="submit">${glyph("login")}Prihlásiť sa</button><a href="/sign_up">Vytvoriť účet</a><a href="/passwords/new">Zabudli ste heslo?</a></div>
    </form></section>`;
  }
}

customElements.define("spotreba-login-page", LoginPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-login-page": LoginPage }
}
