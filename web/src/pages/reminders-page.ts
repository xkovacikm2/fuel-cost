import { html } from "lit";
import { request } from "../lib/api";
import { removeReminder } from "../lib/removals";
import type { Reminder } from "../lib/types";
import { actionsColumn, dataTable, type Column } from "../components/data-table";
import { iconButton, iconLink } from "../components/icons";
import { sectionHeading } from "../components/section-heading";
import { SpotrebaPage } from "./page";

export class RemindersPage extends SpotrebaPage {
  private reminders: Reminder[] = [];

  private columns: Column<Reminder>[] = [
    { header: "Vozidlo", cell: (reminder) => reminder.vehicle_name },
    { header: "Údržba", cell: (reminder) => reminder.kind_label },
    { header: "Limit", cell: (reminder) => [reminder.interval_days ? `${reminder.interval_days} dní` : "", reminder.interval_km ? `${Number(reminder.interval_km)} km` : ""].filter(Boolean).join(" / ") },
    { header: "Stav", cell: (reminder) => reminder.status },
    actionsColumn((reminder) => html`${iconLink(`/maintenance_reminders/${reminder.id}/edit`, "Upraviť pripomienku", "edit")}${iconButton("Vymazať pripomienku", "trash", () => this.deleteRecord(removeReminder(reminder.id!)), true)}`),
  ];

  protected async load() {
    this.reminders = (await request<{ reminders: Reminder[] }>("/api/maintenance_reminders")).reminders;
  }

  protected renderPage() {
    return html`<section class="card">${sectionHeading("Pripomienky údržby", iconLink("/maintenance_reminders/new", "Nová pripomienka", "plus"))}
      ${dataTable({ columns: this.columns, rows: this.reminders, empty: "Zatiaľ nemáte nastavené žiadne pripomienky údržby." })}
    </section>`;
  }
}

customElements.define("spotreba-reminders-page", RemindersPage);

declare global {
  interface HTMLElementTagNameMap { "spotreba-reminders-page": RemindersPage }
}
