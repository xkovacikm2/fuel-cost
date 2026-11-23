import { render } from "lit";
import { describe, expect, it } from "vitest";
import { pagination } from "./pagination";

function draw(page: number, totalPages: number) {
  const host = document.createElement("div");
  render(pagination({ page, totalPages, shown: 10, total: 25, href: (target) => `/?table_page=${target}` }), host);
  return host;
}

describe("pagination", () => {
  it("summarises the visible records and the current page", () => {
    const host = draw(2, 3);
    expect(host.textContent).toContain("Zobrazuje sa 10 z 25 záznamov");
    expect(host.textContent).toContain("Strana 2 / 3");
  });

  it("links to both neighbouring pages in the middle", () => {
    const host = draw(2, 3);
    expect(host.querySelector('a[aria-label="Predchádzajúca strana"]')?.getAttribute("href")).toBe("/?table_page=1");
    expect(host.querySelector('a[aria-label="Nasledujúca strana"]')?.getAttribute("href")).toBe("/?table_page=3");
  });

  it("has no previous link on the first page and no next link on the last", () => {
    const first = draw(1, 3);
    expect(first.querySelector('a[aria-label="Predchádzajúca strana"]')).toBeNull();
    expect(first.querySelector('a[aria-label="Nasledujúca strana"]')).not.toBeNull();
    const last = draw(3, 3);
    expect(last.querySelector('a[aria-label="Predchádzajúca strana"]')).not.toBeNull();
    expect(last.querySelector('a[aria-label="Nasledujúca strana"]')).toBeNull();
  });
});
