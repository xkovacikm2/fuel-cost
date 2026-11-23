import { afterEach, describe, expect, it, vi } from "vitest";
import { parseRoute } from "../lib/view";
import { reminder, stubSignedIn } from "../testing";
import "./reminder-form-page";

async function mountPage(path: string) {
  const page = document.createElement("spotreba-reminder-form-page");
  page.route = parseRoute(path);
  const navigate = vi.fn<(event: CustomEvent) => void>();
  page.addEventListener("spotreba:navigate", navigate);
  document.body.append(page);
  await vi.waitFor(() => expect(page.querySelector("form")).not.toBeNull());
  return { page, navigate };
}

const leads = (page: HTMLElement) => page.querySelectorAll(".reminder-lead");
const input = (page: HTMLElement, name: string) => page.querySelector<HTMLInputElement>(`[name="${name}"]`)!;

describe("reminder form page", () => {
  afterEach(() => {
    document.body.replaceChildren();
    vi.unstubAllGlobals();
  });

  it("starts a new reminder with one empty lead", async () => {
    stubSignedIn();
    const { page } = await mountPage("/maintenance_reminders/new");
    expect(page.querySelector("h2")?.textContent).toBe("Nová pripomienka");
    expect(leads(page)).toHaveLength(1);
    expect(page.querySelector<HTMLSelectElement>("#vehicle_id")?.value).toBe("1");
  });

  it("keeps what was typed when a lead is added or removed", async () => {
    stubSignedIn();
    const { page } = await mountPage("/maintenance_reminders/new");
    input(page, "interval_time_value").value = "14";
    input(page, "lead_km_0").value = "500";

    page.querySelector<HTMLButtonElement>('button[aria-label="Pridať pripomienku"]')!.click();
    await vi.waitFor(() => expect(leads(page)).toHaveLength(2));
    expect(input(page, "interval_time_value").value).toBe("14");
    expect(input(page, "lead_km_0").value).toBe("500");

    leads(page)[1].querySelector<HTMLButtonElement>('button[aria-label="Odstrániť"]')!.click();
    await vi.waitFor(() => expect(leads(page)).toHaveLength(1));
    expect(input(page, "lead_km_0").value).toBe("500");
  });

  it("saves intervals and leads in days and goes back to the list with a notice", async () => {
    const fetchMock = stubSignedIn({
      "/api/maintenance_reminders/7": { reminder },
      "PATCH /api/maintenance_reminders/7": { reminder },
    });
    const { page, navigate } = await mountPage("/maintenance_reminders/7/edit");
    input(page, "interval_time_value").value = "2";
    page.querySelector<HTMLSelectElement>('select[name="interval_time_unit"]')!.value = "weeks";

    page.querySelector("form")!.dispatchEvent(new SubmitEvent("submit", { cancelable: true, bubbles: true }));

    await vi.waitFor(() => expect(navigate).toHaveBeenCalledOnce());
    const [, init] = fetchMock.mock.calls.find(([url, options]) => url === "/api/maintenance_reminders/7" && options?.method === "PATCH")!;
    expect(JSON.parse(String(init?.body))).toMatchObject({
      vehicle_id: 1, kind: "oil_change", interval_days: 14, active: true,
      leads: [{ id: 1, days_before: 30, kilometres_before: null }, { id: 2, days_before: null, kilometres_before: "1000" }],
    });
    expect(navigate.mock.calls[0][0].detail).toEqual({ path: "/maintenance_reminders", notice: "Pripomienka údržby bola úspešne upravená." });
  });
});
