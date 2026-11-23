import { html } from "lit";

export function icon(name: string) {
  return html`<img src=${`/icons/${name}.svg`} alt="" aria-hidden="true">`;
}

// Masked rather than an <img>, so the glyph takes the text colour of the button it sits in.
export function glyph(name: string) {
  return html`<span class="icon" style=${`--icon: url(/icons/${name}.svg)`} aria-hidden="true"></span>`;
}

export function iconLink(href: string, label: string, name: string) {
  return html`<a class="icon-button" href=${href} aria-label=${label} title=${label}>${glyph(name)}</a>`;
}

export function iconButton(label: string, name: string, onClick: () => void, danger = false) {
  return html`<button type="button" class=${danger ? "icon-button danger" : "icon-button"} aria-label=${label} title=${label} @click=${onClick}>${glyph(name)}</button>`;
}
