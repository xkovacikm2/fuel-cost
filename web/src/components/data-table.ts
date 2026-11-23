import { html } from "lit";

export type Column<Row> = { header: string; cell: (row: Row) => unknown };

export function actionsColumn<Row>(actions: (row: Row) => unknown): Column<Row> {
  return { header: "Akcie", cell: (row) => html`<div class="actions">${actions(row)}</div>` };
}

// Every cell carries its column header as data-label; the stacked mobile layout prints it before the value.
export function dataTable<Row>({ columns, rows, empty }: { columns: Column<Row>[]; rows: Row[]; empty: string }) {
  if (!rows.length) return html`<p class="muted">${empty}</p>`;
  return html`<table><thead><tr>${columns.map((column) => html`<th>${column.header}</th>`)}</tr></thead><tbody>${rows.map((row) => html`<tr>${columns.map((column) => html`<td data-label=${column.header}>${column.cell(row)}</td>`)}</tr>`)}</tbody></table>`;
}
