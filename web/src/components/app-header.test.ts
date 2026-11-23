import { afterEach, describe, expect, it, vi } from "vitest";
import { parseRoute } from "../lib/view";
import { noContent, stubApi, user } from "../testing";
import "./app-header";

async function mountHeader(signedIn = true) {
  const header = document.createElement("spotreba-header");
  header.user = signedIn ? user : null;
  header.route = parseRoute("/");
  document.body.append(header);
  await header.updateComplete;
  return header;
}

describe("<spotreba-header>", () => {
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("offers sign-in and registration to visitors", async () => {
    const header = await mountHeader(false);
    expect([...header.querySelectorAll(".main-nav a")].map((link) => link.getAttribute("href"))).toEqual(["/session/new", "/sign_up"]);
    expect(header.querySelector(".nav-toggle")).toBeNull();
  });

  it("links the main sections and the user menu for a signed-in user", async () => {
    const header = await mountHeader();
    const sections = [...header.querySelectorAll(".main-nav > a.nav-link")].map((link) => link.getAttribute("href"));
    expect(sections).toEqual(["/", "/refuelings/new", "/additional_costs/new", "/maintenance_reminders", "/vehicles"]);
    expect(header.querySelector(".user-menu > button")?.textContent).toContain("Používateľ");
  });

  it("opens and closes the navigation drawer used on small screens", async () => {
    const header = await mountHeader();
    const toggle = header.querySelector<HTMLButtonElement>(".nav-toggle")!;
    const nav = header.querySelector(".main-nav")!;
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    toggle.click();
    await header.updateComplete;
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(nav.classList.contains("is-expanded")).toBe(true);
    toggle.click();
    await header.updateComplete;
    expect(nav.classList.contains("is-expanded")).toBe(false);
  });

  it("closes the drawer and the user menu once the route changes", async () => {
    const header = await mountHeader();
    header.querySelector<HTMLButtonElement>(".nav-toggle")!.click();
    header.querySelector<HTMLButtonElement>(".user-menu > button")!.click();
    await header.updateComplete;
    const panel = header.querySelector<HTMLElement>(".user-menu-panel")!;
    expect(panel.hidden).toBe(false);

    header.route = parseRoute("/vehicles");
    await header.updateComplete;
    expect(panel.hidden).toBe(true);
    expect(header.querySelector(".main-nav")!.classList.contains("is-expanded")).toBe(false);
  });

  it("signs out through the API and announces it", async () => {
    const fetchMock = stubApi({ "DELETE /api/session": noContent() });
    const header = await mountHeader();
    const signedOut = vi.fn();
    header.addEventListener("spotreba:signed-out", signedOut);

    header.querySelector<HTMLButtonElement>('.user-menu-panel button[title="Odhlásiť sa"]')!.click();

    await vi.waitFor(() => expect(signedOut).toHaveBeenCalledOnce());
    expect(fetchMock).toHaveBeenCalledWith("/api/session", expect.objectContaining({ method: "DELETE" }));
  });
});
