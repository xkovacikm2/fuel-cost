import { render } from "lit";
import { describe, expect, it, vi } from "vitest";
import { formActions, inputField, selectField, timeField, timeUnitSelect, vehicleField } from "./form-fields";

function draw(template: Parameters<typeof render>[0]) {
  const host = document.createElement("div");
  render(template, host);
  return host;
}

describe("inputField", () => {
  it("pairs a label with its input and carries the validation attributes", () => {
    const host = draw(inputField({ id: "amount", label: "Množstvo", type: "number", step: "0.01", min: "0.01", required: true, value: "35.5" }));
    const input = host.querySelector("input")!;
    expect(host.querySelector("label")?.getAttribute("for")).toBe("amount");
    expect(host.querySelector("label")?.textContent).toBe("Množstvo");
    expect([input.id, input.name, input.type, input.step, input.min, input.required, input.value]).toEqual(["amount", "amount", "number", "0.01", "0.01", true, "35.5"]);
  });

  it("leaves out attributes that were not asked for", () => {
    const input = draw(inputField({ id: "name", label: "Názov vozidla" })).querySelector("input")!;
    for (const attribute of ["type", "required", "maxlength", "autocomplete", "step", "min"]) expect(input.hasAttribute(attribute), attribute).toBe(false);
    expect(input.value).toBe("");
  });
});

describe("selectField", () => {
  it("renders the options and marks the selected one", () => {
    const host = draw(selectField({ id: "kind", label: "Typ", options: [["repair", "Oprava"], ["other", "Iné"]], selected: "other" }));
    expect([...host.querySelectorAll("option")].map((option) => [option.value, option.textContent, option.selected])).toEqual([["repair", "Oprava", false], ["other", "Iné", true]]);
    expect(host.querySelector("select")?.name).toBe("kind");
  });

  it("compares numeric ids with the selected value", () => {
    const host = draw(vehicleField([{ id: 1, name: "Fabia" }, { id: 2, name: "Zoe" }], 2));
    expect(host.querySelector("select")?.id).toBe("vehicle_id");
    expect(host.querySelector("label")?.textContent).toBe("Vozidlo");
    expect(host.querySelector<HTMLSelectElement>("select")?.value).toBe("2");
  });

  it("reports changes to the caller", () => {
    const onChange = vi.fn();
    const select = draw(vehicleField([{ id: 1, name: "Fabia" }], 1, onChange)).querySelector("select")!;
    select.dispatchEvent(new Event("change"));
    expect(onChange).toHaveBeenCalledOnce();
  });
});

describe("timeUnitSelect", () => {
  it("offers days, weeks, months and years with the given unit selected", () => {
    const select = draw(timeUnitSelect("interval_time_unit", "months")).querySelector("select")!;
    expect([...select.options].map((option) => option.value)).toEqual(["days", "weeks", "months", "years"]);
    expect(select.name).toBe("interval_time_unit");
    expect(select.getAttribute("aria-label")).toBe("Časová jednotka");
    expect(select.value).toBe("months");
  });
});

describe("timeField", () => {
  it("combines a labelled amount with its unit", () => {
    const host = draw(timeField({ id: "lead_time_0", label: "Čas vopred", unitName: "lead_unit_0", value: "2", unit: "weeks" }));
    const input = host.querySelector("input")!;
    expect(host.querySelector("label")?.getAttribute("for")).toBe("lead_time_0");
    expect([input.id, input.name, input.type, input.min, input.step, input.value]).toEqual(["lead_time_0", "lead_time_0", "number", "1", "1", "2"]);
    expect(host.querySelector<HTMLSelectElement>(".input-with-unit select")).toMatchObject({ name: "lead_unit_0", value: "weeks" });
  });

  it("starts empty and in days", () => {
    const host = draw(timeField({ id: "interval_time_value", label: "Časový interval", unitName: "interval_time_unit" }));
    expect(host.querySelector("input")?.value).toBe("");
    expect(host.querySelector("select")?.value).toBe("days");
  });
});

describe("formActions", () => {
  it("pairs the labelled submit button with an icon link back", () => {
    const host = draw(formActions("/vehicles"));
    expect(host.querySelector('button[type="submit"]')?.textContent).toBe("Uložiť");
    expect(host.querySelector('a.icon-button[aria-label="Späť"]')?.getAttribute("href")).toBe("/vehicles");
  });
});
