import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["vehicle", "unit"]
  static values = { units: Object }

  connect() {
    this.updateUnit()
  }

  vehicleChanged() {
    this.updateUnit()
  }

  updateUnit() {
    if (!this.hasVehicleTarget || !this.hasUnitTarget) {
      return
    }

    const vehicleId = this.vehicleTarget.value
    this.unitTarget.textContent = this.unitsValue[vehicleId] || ""
  }
}
