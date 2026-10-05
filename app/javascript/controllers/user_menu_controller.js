import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["toggle", "panel", "form", "file", "importButton"]

  connect() {
    this.reset()
  }

  toggle() {
    const isOpen = this.panelTarget.hidden
    this.panelTarget.hidden = !isOpen
    this.toggleTarget.setAttribute("aria-expanded", String(isOpen))
  }

  close() {
    this.panelTarget.hidden = true
    this.toggleTarget.setAttribute("aria-expanded", "false")
  }

  closeOutside(event) {
    if (!this.element.contains(event.target)) this.close()
  }

  escape(event) {
    if (this.panelTarget.hidden) return
    event.preventDefault()
    this.close()
    this.toggleTarget.focus()
  }

  chooseFile() {
    if (this.importButtonTarget.disabled) return
    this.fileTarget.value = ""
    this.fileTarget.click()
  }

  upload() {
    if (this.fileTarget.files.length && !this.importButtonTarget.disabled) this.formTarget.requestSubmit()
  }

  busy() {
    this.importButtonTarget.disabled = true
    this.formTarget.setAttribute("aria-busy", "true")
  }

  reset() {
    this.close()
    this.fileTarget.value = ""
    this.importButtonTarget.disabled = false
    this.formTarget.removeAttribute("aria-busy")
  }
}