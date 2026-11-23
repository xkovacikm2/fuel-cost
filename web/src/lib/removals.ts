import type { Redirect } from "./events";

// What deleting a record asks, calls and where it leads afterwards.
export type Removal = { url: string; confirmation: string; next: Redirect };

export const removeVehicle = (id: number): Removal => ({
  url: `/api/vehicles/${id}`,
  confirmation: "Vymazať toto vozidlo a všetky súvisiace tankovania?",
  next: { path: "/vehicles", notice: "Vozidlo bolo úspešne odstránené." },
});

export const removeReminder = (id: number): Removal => ({
  url: `/api/maintenance_reminders/${id}`,
  confirmation: "Vymazať túto pripomienku?",
  next: { path: "/maintenance_reminders", notice: "Pripomienka údržby bola úspešne odstránená." },
});

export const removeRefueling = (id: number): Removal => ({
  url: `/api/refuelings/${id}`,
  confirmation: "Vymazať tento záznam tankovania?",
  next: { path: "/", notice: "Tankovanie bolo úspešne odstránené." },
});

export const removeAdditionalCost = (id: number): Removal => ({
  url: `/api/additional_costs/${id}`,
  confirmation: "Vymazať tento dodatočný náklad?",
  next: { path: "/", notice: "Dodatočný náklad bol úspešne odstránený." },
});
