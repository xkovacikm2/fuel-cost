import { html, render } from "lit";
import { describe, expect, it } from "vitest";
import { actionsColumn, dataTable, type Column } from "./data-table";

type Row = { name: string; fuel: string };

const columns: Column<Row>[] = [
  { header: "Názov", cell: (row) => row.name },
  { header: "Typ paliva", cell: (row) => html`<em>${row.fuel}</em>` },
  actionsColumn((row) => html`<a href=${`/vehicles/${row.name}`}>${row.name}</a>`),
];

function draw(rows: Row[], empty = "Nič tu nie je.") {
  const host = document.createElement("div");
  render(dataTable({ columns, rows, empty }), host);
  return host;
}

describe("dataTable", () => {
  const rows = [{ name: "Fabia", fuel: "Benzín" }, { name: "Zoe", fuel: "Elektrina" }];

  it("renders a header cell per column and a row per record", () => {
    const host = draw(rows);
    expect([...host.querySelectorAll("thead th")].map((th) => th.textContent)).toEqual(["Názov", "Typ paliva", "Akcie"]);
    expect(host.querySelectorAll("tbody tr")).toHaveLength(2);
    expect([...host.querySelectorAll("tbody tr")[1].children].slice(0, 2).map((td) => td.textContent)).toEqual(["Zoe", "Elektrina"]);
  });

  it("labels every cell with its column header for the stacked mobile layout", () => {
    const host = draw(rows);
    for (const row of host.querySelectorAll("tbody tr")) {
      expect([...row.children].map((td) => td.getAttribute("data-label"))).toEqual(["Názov", "Typ paliva", "Akcie"]);
    }
  });

  it("wraps the actions column in an .actions container", () => {
    const cell = draw(rows).querySelector("tbody tr td:last-child");
    expect(cell?.querySelector(":scope > .actions a")?.getAttribute("href")).toBe("/vehicles/Fabia");
  });

  it("shows the empty message instead of an empty table", () => {
    const host = draw([], "Zatiaľ žiadne vozidlá.");
    expect(host.querySelector("table")).toBeNull();
    expect(host.querySelector("p.muted")?.textContent).toBe("Zatiaľ žiadne vozidlá.");
  });
});
