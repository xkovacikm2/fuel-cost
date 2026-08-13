import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["toggle", "menu", "expand"]

  toggle() {
    const isOpen = this.menuTarget.classList.toggle("is-visible")

    if (!isOpen) {
      this.menuTarget.classList.remove("is-expanded")
      this.expandTarget.setAttribute("aria-expanded", "false")
    }

    this.toggleTarget.setAttribute("aria-expanded", isOpen)
    this.toggleTarget.setAttribute("aria-label", isOpen ? "Zavrieť navigáciu" : "Otvoriť navigáciu")
    this.toggleTarget.setAttribute("title", isOpen ? "Zavrieť navigáciu" : "Otvoriť navigáciu")
  }

  expand() {
    const isExpanded = this.menuTarget.classList.toggle("is-expanded")
    const icon = this.expandTarget.querySelector("img")

    this.expandTarget.setAttribute("aria-expanded", isExpanded)
    this.expandTarget.setAttribute("aria-label", isExpanded ? "Zúžiť navigáciu" : "Rozšíriť navigáciu")
    this.expandTarget.setAttribute("title", isExpanded ? "Zúžiť navigáciu" : "Rozšíriť navigáciu")
    this.expandTarget.querySelector("span").textContent = isExpanded ? "Zúžiť navigáciu" : "Rozšíriť navigáciu"
    icon.src = isExpanded ? "/icons/chevron-left.svg" : "/icons/chevron-right.svg"
  }
}