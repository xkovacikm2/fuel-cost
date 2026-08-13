import { Controller } from "@hotwired/stimulus"

const eventPositions = (chart, events) => {
    const labels = chart.data.labels
    const xScale = chart.scales.x

    return events.map((event) => {
      const targetTime = Date.parse(event.date)
      const exactIndex = labels.findIndex((label) => label === event.date)
      if (exactIndex >= 0) return xScale.getPixelForValue(exactIndex)

      for (let index = 1; index < labels.length; index += 1) {
        const previousTime = Date.parse(labels[index - 1])
        const nextTime = Date.parse(labels[index])
        if (targetTime < previousTime || targetTime > nextTime) continue

        const fraction = (targetTime - previousTime) / (nextTime - previousTime)
        const previousX = xScale.getPixelForValue(index - 1)
        const nextX = xScale.getPixelForValue(index)
        return previousX + (nextX - previousX) * fraction
      }

      return null
    })
}

const additionalCostsPlugin = {
  id: "additionalCosts",
  afterDatasetsDraw(chart, _args, options) {
    const events = options.events || []
    const { ctx, chartArea, scales } = chart
    const positions = eventPositions(chart, events)
    const hoveredIndex = chart.$additionalCostMouseX === null
      ? null
      : positions.findIndex((x) => x !== null && Math.abs(x - chart.$additionalCostMouseX) <= 7)
    chart.$additionalCostHover = hoveredIndex

    ctx.save()
    ctx.font = "11px Segoe UI, sans-serif"
    ctx.textAlign = "left"
    ctx.textBaseline = "middle"

    events.forEach((event, index) => {
      const x = positions[index]
      if (x === null) return
      const color = index % 2 === 0 ? "#b42318" : "#9c2c77"
      ctx.strokeStyle = color
      ctx.setLineDash([5, 4])
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.moveTo(x, chartArea.top)
      ctx.lineTo(x, chartArea.bottom)
      ctx.stroke()

      ctx.fillStyle = color
      ctx.translate(x + 5, chartArea.top + 8 + (index % 3) * 16)
      ctx.rotate(-Math.PI / 2)
      ctx.fillText(`${event.label} (${event.vehicle})`, 0, 0)
      ctx.setTransform(1, 0, 0, 1, 0, 0)
    })

    const hoveredEvent = events[chart.$additionalCostHover]
    if (hoveredEvent) {
      const x = positions[chart.$additionalCostHover]
      const text = `${hoveredEvent.label}: ${Number(hoveredEvent.cost).toFixed(2)} €`
      const padding = 8
      const width = ctx.measureText(text).width + padding * 2
      const height = 30
      const tooltipX = Math.min(Math.max(x - width / 2, chartArea.left), chartArea.right - width)
      const tooltipY = chartArea.top + 8

      ctx.fillStyle = "rgba(31, 41, 55, 0.95)"
      ctx.fillRect(tooltipX, tooltipY, width, height)
      ctx.fillStyle = "#ffffff"
      ctx.textAlign = "center"
      ctx.fillText(text, tooltipX + width / 2, tooltipY + height / 2)
    }

    ctx.restore()
  }
}

export default class extends Controller {
  static targets = ["canvas"]
  static values = { payload: Object }

  connect() {
    if (!this.hasCanvasTarget || !this.hasPayloadValue) {
      return
    }

    this.handleMouseMove = (event) => {
      if (!this.chart) return

      const rect = this.canvasTarget.getBoundingClientRect()
      this.chart.$additionalCostMouseX = event.clientX - rect.left
      this.chart.update("none")
    }
    this.handleMouseLeave = () => {
      if (!this.chart) return

      this.chart.$additionalCostMouseX = null
      this.chart.update("none")
    }
    this.canvasTarget.addEventListener("mousemove", this.handleMouseMove)
    this.canvasTarget.addEventListener("mouseleave", this.handleMouseLeave)
    this.tryRender()
  }

  disconnect() {
    this.canvasTarget.removeEventListener("mousemove", this.handleMouseMove)
    this.canvasTarget.removeEventListener("mouseleave", this.handleMouseLeave)
    this.chart?.destroy()
  }

  tryRender(retries = 20) {
    if (!window.Chart) {
      if (retries <= 0) return

      setTimeout(() => this.tryRender(retries - 1), 50)
      return
    }

    const payload = this.payloadValue
    window.Chart.register(additionalCostsPlugin)

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
          },
          additionalCosts: {
            events: payload.additional_costs || []
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
    this.chart.$additionalCostMouseX = null
  }
}
