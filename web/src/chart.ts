import { Chart, registerables, type ChartDataset, type Plugin } from "chart.js";

export type ChartPayload = {
  title: string; labels: string[];
  datasets: { label: string; data: (string | null)[]; yAxisID: string; borderColor: string; backgroundColor: string; borderDash?: number[]; tension: number }[];
  additional_costs: { occurred_on: string; kind_label: string; vehicle_name: string; cost: string }[];
};

const payloads = new WeakMap<Chart, ChartPayload>();
const plugin: Plugin = {
  id: "additionalCosts",
  afterDatasetsDraw(chart) {
    const payload = payloads.get(chart);
    if (!payload) return;
    const { ctx, chartArea, scales } = chart;
    payload.additional_costs.forEach((cost, index) => {
      const target = Date.parse(cost.occurred_on);
      let position: number | null = null;
      const exact = payload.labels.indexOf(cost.occurred_on);
      if (exact >= 0) position = scales.x.getPixelForValue(exact);
      for (let label = 1; position === null && label < payload.labels.length; label++) {
        const previous = Date.parse(payload.labels[label - 1]);
        const next = Date.parse(payload.labels[label]);
        if (target >= previous && target <= next && next !== previous) position = scales.x.getPixelForValue(label - 1) + (scales.x.getPixelForValue(label) - scales.x.getPixelForValue(label - 1)) * (target - previous) / (next - previous);
      }
      if (position === null) return;
      ctx.save();
      ctx.strokeStyle = index % 2 ? "#9c2c77" : "#b42318";
      ctx.lineWidth = 2;
      ctx.setLineDash([5, 4]);
      ctx.beginPath(); ctx.moveTo(position, chartArea.top); ctx.lineTo(position, chartArea.bottom); ctx.stroke();
      ctx.fillStyle = ctx.strokeStyle;
      ctx.translate(position + 5, chartArea.top + 8 + (index % 3) * 16);
      ctx.rotate(-Math.PI / 2);
      ctx.fillText(`${cost.kind_label} (${cost.vehicle_name})`, 0, 0);
      ctx.restore();
    });
  },
};
Chart.register(...registerables, plugin);
(window as Window & { Chart?: typeof Chart }).Chart = Chart;

export function destroyChart(canvas: HTMLCanvasElement) { Chart.getChart(canvas)?.destroy(); }

export function renderChart(canvas: HTMLCanvasElement, payload: ChartPayload) {
  if (Chart.getChart(canvas)) return;
  const chart = new Chart(canvas, {
    type: "line",
    data: { labels: payload.labels, datasets: payload.datasets as unknown as ChartDataset<"line">[] },
    options: {
      responsive: true, maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: { legend: { position: "bottom" } },
      scales: { yConsumption: { type: "linear", position: "left" }, yCost: { type: "linear", position: "right", grid: { drawOnChartArea: false } } },
    },
  });
  payloads.set(chart, payload);
  chart.update("none");
}