import { html } from "lit";
import { afterEach, describe, expect, it, vi } from "vitest";
import { parseRoute } from "../lib/view";
import { SpotrebaPage, type Redirect } from "./page";

class ProbePage extends SpotrebaPage {
  loading: () => Promise<void | Redirect> = async () => {};
  protected load() { return this.loading(); }
  protected renderPage() { return html`<p>loaded</p>`; }
  submitting(event: SubmitEvent, action: (form: FormData) => Promise<void>) { return this.submit(event, action); }
}
customElements.define("spotreba-probe-page", ProbePage);
declare global {
  interface HTMLElementTagNameMap { "spotreba-probe-page": ProbePage }
}

function probe(loading: ProbePage["loading"]) {
  const page = document.createElement("spotreba-probe-page");
  page.route = parseRoute("/");
  page.loading = loading;
  const flash = vi.fn<(event: CustomEvent) => void>();
  const navigate = vi.fn<(event: CustomEvent) => void>();
  page.addEventListener("spotreba:flash", flash);
  page.addEventListener("spotreba:navigate", navigate);
  return { page, flash, navigate };
}

const details = (spy: { mock: { calls: [CustomEvent][] } }) => spy.mock.calls.map(([event]) => event.detail);

describe("SpotrebaPage", () => {
  afterEach(() => document.body.replaceChildren());

  it("renders nothing until its data has loaded, then its content", async () => {
    let finish!: () => void;
    const { page } = probe(() => new Promise<void>((resolve) => { finish = resolve; }));
    document.body.append(page);
    await page.updateComplete;
    expect(page.textContent).toBe("");
    finish();
    await vi.waitFor(() => expect(page.textContent).toBe("loaded"));
  });

  it("reports a failed load as a flash error and stays blank", async () => {
    const { page, flash } = probe(async () => { throw new Error("Vozidlo neexistuje."); });
    document.body.append(page);
    await vi.waitFor(() => expect(flash).toHaveBeenCalledOnce());
    expect(details(flash)).toEqual([{ error: "Vozidlo neexistuje." }]);
    await page.updateComplete;
    expect(page.textContent).toBe("");
  });

  it("navigates away instead of rendering when the load asks for a redirect", async () => {
    const { page, navigate } = probe(async () => ({ path: "/vehicles/new", error: "Najprv vytvorte vozidlo." }));
    document.body.append(page);
    await vi.waitFor(() => expect(navigate).toHaveBeenCalledOnce());
    expect(details(navigate)).toEqual([{ path: "/vehicles/new", error: "Najprv vytvorte vozidlo." }]);
    await page.updateComplete;
    expect(page.textContent).toBe("");
  });

  it("stays silent when it was replaced before its load finished", async () => {
    let fail!: (error: Error) => void;
    const { page, flash, navigate } = probe(() => new Promise<void>((_, reject) => { fail = reject; }));
    document.body.append(page);
    await page.updateComplete;
    page.remove();
    fail(new Error("Príliš neskoro."));
    await new Promise((resolve) => setTimeout(resolve));
    expect(flash).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
    expect(page.ready).toBe(false);
  });

  describe("submit", () => {
    async function submitForm(action: (form: FormData) => Promise<void>) {
      const { page, flash } = probe(async () => {});
      document.body.append(page);
      await vi.waitFor(() => expect(page.ready).toBe(true));
      const form = document.createElement("form");
      form.innerHTML = '<input name="name" value="Fabia">';
      let outcome: Promise<boolean> | undefined;
      form.addEventListener("submit", (event) => { outcome = page.submitting(event as SubmitEvent, action); });
      page.append(form);
      const event = new SubmitEvent("submit", { cancelable: true });
      form.dispatchEvent(event);
      return { event, flash, outcome: await outcome! };
    }

    it("clears the previous error, hands over the form data and reports success", async () => {
      const action = vi.fn(async (_form: FormData) => {});
      const { event, flash, outcome } = await submitForm(action);
      expect(event.defaultPrevented).toBe(true);
      expect(Object.fromEntries(action.mock.calls[0][0])).toEqual({ name: "Fabia" });
      expect(details(flash)).toEqual([{ error: "" }]);
      expect(outcome).toBe(true);
    });

    it("shows the error of a failing action", async () => {
      const { flash, outcome } = await submitForm(async () => { throw new Error("Toto vozidlo sa nepodarilo uložiť: Názov je povinný."); });
      expect(details(flash)).toEqual([{ error: "" }, { error: "Toto vozidlo sa nepodarilo uložiť: Názov je povinný." }]);
      expect(outcome).toBe(false);
    });
  });
});
