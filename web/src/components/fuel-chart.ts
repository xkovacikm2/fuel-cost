import { LitElement, html, nothing, type PropertyValues } from "lit";
import { destroyChart, renderChart, type ChartPayload } from "../chart";

// Draws the consumption chart card. Chart.js needs a canvas that is in the document, so the chart is
// (re)drawn after every render and torn down together with the element.
export class FuelChart extends LitElement {
  static properties = { chart: { attribute: false } };

  declare chart: ChartPayload;
  private drawn: HTMLCanvasElement | null = null;

  createRenderRoot() { return this; }

  connectedCallback() {
    super.connectedCallback();
    if (this.hasUpdated) this.requestUpdate("chart");
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.release();
  }

  protected updated(changed: PropertyValues<this>) {
    if (!changed.has("chart") || !this.isConnected) return;
    const canvas = this.querySelector("canvas");
    if (this.drawn !== canvas) this.release();
    this.drawn = canvas;
    if (!canvas) return;
    destroyChart(canvas);
    renderChart(canvas, this.chart);
  }

  private release() {
    if (this.drawn) destroyChart(this.drawn);
    this.drawn = null;
  }

  render() {
    const { title, labels, additional_costs: costs } = this.chart;
    return html`<section class="card"><h3>${title}</h3>
      ${labels.length ? html`<div class="chart-wrap"><canvas aria-label="Graf spotreby paliva"></canvas></div>${costs.length ? html`<p class="muted">Zvislé prerušované čiary označujú dodatočné náklady.</p>` : nothing}` : html`<p class="muted">V tomto období nie sú žiadne údaje o tankovaní ani dodatočných nákladoch.</p>`}
    </section>`;
  }
}

customElements.define("spotreba-fuel-chart", FuelChart);

declare global {
  interface HTMLElementTagNameMap { "spotreba-fuel-chart": FuelChart }
}
