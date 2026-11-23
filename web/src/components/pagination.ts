import { html, nothing } from "lit";
import { iconLink } from "./icons";

type PaginationOptions = { page: number; totalPages: number; shown: number; total: number; href: (page: number) => string };

export function pagination({ page, totalPages, shown, total, href }: PaginationOptions) {
  return html`<div class="section-heading"><p class="muted">Zobrazuje sa ${shown} z ${total} záznamov</p><div class="actions">${page > 1 ? iconLink(href(page - 1), "Predchádzajúca strana", "chevron-left") : nothing}<span class="muted">Strana ${page} / ${totalPages}</span>${page < totalPages ? iconLink(href(page + 1), "Nasledujúca strana", "chevron-right") : nothing}</div></div>`;
}
