import { html } from "lit";

export function statCard(label: string, value: string | number) {
  return html`<article class="card"><h3>${label}</h3><p class="stat">${value}</p></article>`;
}
