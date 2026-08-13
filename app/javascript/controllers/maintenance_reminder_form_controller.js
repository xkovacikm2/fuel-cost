import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["leads", "template"]

  addLead(event) {
    event.preventDefault()
    const identifier = new Date().getTime()
    const fields = this.templateTarget.innerHTML.replace(/NEW_RECORD/g, identifier)

    this.leadsTarget.insertAdjacentHTML("beforeend", fields)
  }

  removeLead(event) {
    event.preventDefault()
    const lead = event.target.closest("[data-maintenance-reminder-form-target='lead']")
    const destroyInput = lead.querySelector("input[name*='[_destroy]']")

    if (lead.dataset.persisted === "true") {
      destroyInput.value = "1"
      lead.hidden = true
    } else {
      lead.remove()
    }
  }
}