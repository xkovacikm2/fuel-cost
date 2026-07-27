import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["canvas"]
  static values = { payload: Object }

  connect() {
    if (!this.hasCanvasTarget || !this.hasPayloadValue) {
      return
    }

    this.tryRender()
  }

  disconnect() {
    this.chart?.destroy()
  }

  tryRender(retries = 20) {
    if (!window.Chart) {
      if (retries <= 0) return

      setTimeout(() => this.tryRender(retries - 1), 50)
      return
    }

    const payload = this.payloadValue

    this.chart = new window.Chart(this.canvasTarget.getContext("2d"), {
      type: "line",
      data: {
        labels: payload.labels,
        datasets: payload.datasets
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: "index",
          intersect: false
        },
        plugins: {
          legend: {
            position: "bottom"
          }
        },
        scales: {
          x: {
            title: {
              display: true,
              text: "Date"
            }
          },
          yConsumption: {
            type: "linear",
            display: true,
            position: "left",
            title: {
              display: true,
              text: "Consumption / 100 km"
            }
          },
          yCost: {
            type: "linear",
            display: true,
            position: "right",
            grid: {
              drawOnChartArea: false
            },
            title: {
              display: true,
              text: "Cost / 100 km"
            }
          }
        }
      }
    })
  }
}
