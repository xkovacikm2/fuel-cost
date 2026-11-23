import { html } from "lit";

export function sectionHeading(title: unknown, action: unknown, level: 2 | 3 = 2) {
  return level === 2
    ? html`<div class="section-heading"><h2>${title}</h2>${action}</div>`
    : html`<div class="section-heading"><h3>${title}</h3>${action}</div>`;
}
