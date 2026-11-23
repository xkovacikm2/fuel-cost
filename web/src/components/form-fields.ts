import { html } from "lit";
import { ifDefined } from "lit/directives/if-defined.js";
import type { Vehicle } from "../lib/types";
import { glyph, iconLink } from "./icons";

export type Option = readonly [value: string | number, label: string];

type InputOptions = {
  id: string; label: string; name?: string; type?: string; value?: string | number | null;
  required?: boolean; maxlength?: number; autocomplete?: string; step?: string; min?: string;
};

// The value goes last so it is applied after type, step and min, which decide how it is sanitised.
export function inputField({ id, label, name = id, type, value, required, maxlength, autocomplete, step, min }: InputOptions) {
  return html`<div class="field"><label for=${id}>${label}</label><input id=${id} name=${name} type=${ifDefined(type)} ?required=${required} maxlength=${ifDefined(maxlength)} autocomplete=${ifDefined(autocomplete)} step=${ifDefined(step)} min=${ifDefined(min)} .value=${value ?? ""}></div>`;
}

type SelectOptions = {
  id: string; label: string; options: readonly Option[]; name?: string;
  selected?: string | number | null; disabled?: boolean; onChange?: (event: Event) => void;
};

export function selectField({ id, label, options, name = id, selected, disabled, onChange }: SelectOptions) {
  return html`<div class="field"><label for=${id}>${label}</label><select id=${id} name=${name} ?disabled=${disabled} @change=${onChange}>${options.map(([value, text]) => html`<option value=${value} ?selected=${selected != null && String(value) === String(selected)}>${text}</option>`)}</select></div>`;
}

export function vehicleField(vehicles: Pick<Vehicle, "id" | "name">[], selected: number | null, onChange?: (event: Event) => void) {
  return selectField({ id: "vehicle_id", label: "Vozidlo", options: vehicles.map((vehicle) => [vehicle.id, vehicle.name]), selected, onChange });
}

export function timeUnitSelect(name: string, value = "days") {
  return html`<select name=${name} aria-label="Časová jednotka" .value=${value}><option value="days">Dni</option><option value="weeks">Týždne</option><option value="months">Mesiace</option><option value="years">Roky</option></select>`;
}

export function timeField({ id, label, unitName, value, unit }: { id: string; label: string; unitName: string; value?: string | null; unit?: string }) {
  return html`<div class="field"><label for=${id}>${label}</label><div class="input-with-unit"><input id=${id} name=${id} type="number" min="1" step="1" .value=${value ?? ""}>${timeUnitSelect(unitName, unit)}</div></div>`;
}

export function formActions(backHref: string) {
  return html`<div class="actions"><button type="submit">${glyph("check")}Uložiť</button>${iconLink(backHref, "Späť", "arrow-left")}</div>`;
}
