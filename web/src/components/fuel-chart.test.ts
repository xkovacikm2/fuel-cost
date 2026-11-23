import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../chart", () => ({ renderChart: vi.fn(), destroyChart: vi.fn() }));

import { destroyChart, renderChart, type ChartPayload } from "../chart";
import "./fuel-chart";

function payload(overrides: Partial<ChartPayload> = {}): ChartPayload {
  return { title: "Spotreba paliva", labels: ["2026-10-01", "2026-10-20"], datasets: [], additional_costs: [], ...overrides };
}

async function mountChart(chart: ChartPayload) {
  const element = document.createElement("spotreba-fuel-chart");
  element.chart = chart;
  document.body.append(element);
  await element.updateComplete;
  return element;
}

describe("<spotreba-fuel-chart>", () => {
  afterEach(() => document.body.replaceChildren());

  it("titles the card and draws the chart on its canvas", async () => {
    const chart = payload();
    const element = await mountChart(chart);
    expect(element.querySelector("h3")?.textContent).toBe("Spotreba paliva");
    const canvas = element.querySelector("canvas");
    expect(canvas?.getAttribute("aria-label")).toBe("Graf spotreby paliva");
    expect(renderChart).toHaveBeenCalledExactlyOnceWith(canvas, chart);
  });

  it("explains an empty period instead of drawing a chart", async () => {
    const element = await mountChart(payload({ labels: [] }));
    expect(element.querySelector("canvas")).toBeNull();
    expect(element.textContent).toContain("V tomto období nie sú žiadne údaje o tankovaní ani dodatočných nákladoch.");
    expect(renderChart).not.toHaveBeenCalled();
  });

  it("mentions the dashed cost lines only when there are additional costs", async () => {
    const cost = { occurred_on: "2026-10-05", kind_label: "Oprava", vehicle_name: "Fabia", cost: "120" };
    expect((await mountChart(payload())).textContent).not.toContain("Zvislé prerušované čiary");
    expect((await mountChart(payload({ additional_costs: [cost] }))).textContent).toContain("Zvislé prerušované čiary označujú dodatočné náklady.");
  });

  it("redraws when it receives a new payload", async () => {
    const element = await mountChart(payload());
    const canvas = element.querySelector("canvas");
    const next = payload({ labels: ["2026-11-01"] });
    element.chart = next;
    await element.updateComplete;
    expect(destroyChart).toHaveBeenCalledWith(canvas);
    expect(renderChart).toHaveBeenLastCalledWith(canvas, next);
  });

  it("destroys the chart when it leaves the page", async () => {
    const element = await mountChart(payload());
    const canvas = element.querySelector("canvas");
    element.remove();
    expect(destroyChart).toHaveBeenCalledWith(canvas);
  });

  it("draws again after being moved within the page", async () => {
    const element = await mountChart(payload());
    const canvas = element.querySelector("canvas");
    document.body.append(element);
    await element.updateComplete;
    expect(destroyChart).toHaveBeenCalledWith(canvas);
    expect(renderChart).toHaveBeenCalledTimes(2);
  });
});
