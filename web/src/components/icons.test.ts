import { render } from "lit";
import { describe, expect, it, vi } from "vitest";
import { iconButton, iconLink } from "./icons";

function draw(template: Parameters<typeof render>[0]) {
  const host = document.createElement("div");
  render(template, host);
  return host;
}

describe("icon actions", () => {
  it("renders a link that is named by its label and shows only a glyph", () => {
    const link = draw(iconLink("/vehicles/new", "Nové vozidlo", "plus")).querySelector("a")!;
    expect(link.getAttribute("href")).toBe("/vehicles/new");
    expect([link.getAttribute("aria-label"), link.title, link.textContent?.trim()]).toEqual(["Nové vozidlo", "Nové vozidlo", ""]);
    expect(link.classList.contains("icon-button")).toBe(true);
    expect(link.querySelector(".icon")?.getAttribute("style")).toContain("/icons/plus.svg");
  });

  it("renders a button that runs its handler and can be styled as dangerous", () => {
    const onClick = vi.fn();
    const host = draw(iconButton("Vymazať vozidlo", "trash", onClick, true));
    const button = host.querySelector("button")!;
    expect(button.type).toBe("button");
    expect(button.className).toBe("icon-button danger");
    expect(button.getAttribute("aria-label")).toBe("Vymazať vozidlo");
    button.click();
    expect(onClick).toHaveBeenCalledOnce();
    expect(draw(iconButton("Pridať", "plus", onClick)).querySelector("button")?.className).toBe("icon-button");
  });
});
