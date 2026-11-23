import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { parseRoute } from "../lib/view";
import { noContent, stubSignedIn } from "../testing";
import "./vehicles-page";

async function mountPage() {
  const page = document.createElement("spotreba-vehicles-page");
  page.route = parseRoute("/vehicles");
  const navigate = vi.fn<(event: CustomEvent) => void>();
  const flash = vi.fn<(event: CustomEvent) => void>();
  page.addEventListener("spotreba:navigate", navigate);
  page.addEventListener("spotreba:flash", flash);
  document.body.append(page);
  await vi.waitFor(() => expect(page.querySelector("table")).not.toBeNull());
  return { page, navigate, flash };
}

const deleteButton = (page: HTMLElement) => page.querySelector<HTMLButtonElement>('button[aria-label="Vymazať vozidlo"]')!;

describe("vehicles page", () => {
  beforeEach(() => {
    stubSignedIn({ "DELETE /api/vehicles/1": noContent() });
  });
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("loads and lists the vehicles on its own", async () => {
    const { page } = await mountPage();
    expect([...page.querySelectorAll("tbody tr")].map((row) => row.children[0].textContent)).toEqual(["Fabia", "Zoe"]);
    expect(fetch).toHaveBeenCalledWith("/api/vehicles", expect.anything());
  });

  it("deletes a vehicle after confirmation and goes back to the list with a notice", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const { page, navigate } = await mountPage();
    deleteButton(page).click();
    await vi.waitFor(() => expect(navigate).toHaveBeenCalledOnce());
    expect(confirm).toHaveBeenCalledWith("Vymazať toto vozidlo a všetky súvisiace tankovania?");
    expect(fetch).toHaveBeenCalledWith("/api/vehicles/1", expect.objectContaining({ method: "DELETE" }));
    expect(navigate.mock.calls[0][0].detail).toEqual({ path: "/vehicles", notice: "Vozidlo bolo úspešne odstránené." });
  });

  it("keeps the vehicle when the confirmation is declined", async () => {
    vi.stubGlobal("confirm", vi.fn(() => false));
    const { page, navigate } = await mountPage();
    deleteButton(page).click();
    await new Promise((resolve) => setTimeout(resolve));
    expect(navigate).not.toHaveBeenCalled();
    expect(fetch).not.toHaveBeenCalledWith("/api/vehicles/1", expect.anything());
  });
});
