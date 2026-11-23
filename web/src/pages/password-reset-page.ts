import { html } from "lit";
import { RequestError, request } from "../lib/api";
import { glyph } from "../components/icons";
import { SpotrebaPage, type Redirect } from "./page";

export class PasswordResetPage extends SpotrebaPage {
  private get url() { return `/api/passwords/${encodeURIComponent(this.route.token!)}`; }

  // An expired or unknown token cannot be used, so ask for a fresh one right away.
  protected async load(): Promise<void | Redirect> {
    try {
      await request<{ valid: boolean }>(this.url);
    } catch (error) {
      if (error instanceof RequestError && error.status === 422) return { path: "/passwords/new", error: error.message };
      throw error;
    }
  }

  private async resetPassword(form: FormData) {
    const result = await request<{ notice: string }>(this.url, "POST", Object.fromEntries(form));
    this.navigate({ path: "/session/new", notice: result.notice });
  }

  protected renderPage() {
    return html`<section class="card"><h1>Obnoviť heslo</h1>
      <form @submit=${(event: SubmitEvent) => this.submit(event, (form) => this.resetPassword(form))}>
        <div class="field"><input name="password" type="password" required maxlength="72" autocomplete="new-password" placeholder="Zadajte nové heslo" aria-label="Zadajte nové heslo"></div><div class="field"><input name="password_confirmation" type="password" required maxlength="72" autocomplete="new-password" placeholder="Zopakujte nové heslo" aria-label="Zopakujte nové heslo"></div>
        <button type="submit">${glyph("check")}Uložiť nové heslo</button>
      </form></section>`;
  }
}

customElements.define("spotreba-password-reset-page", PasswordResetPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-password-reset-page": PasswordResetPage }
}
