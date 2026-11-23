import type { RouteName } from "../lib/view";
import { CostFormPage } from "./cost-form-page";
import { DashboardPage } from "./dashboard-page";
import { LoginPage } from "./login-page";
import type { SpotrebaPage } from "./page";
import { PasswordRequestPage } from "./password-request-page";
import { PasswordResetPage } from "./password-reset-page";
import { ReminderFormPage } from "./reminder-form-page";
import { RemindersPage } from "./reminders-page";
import { RefuelingFormPage } from "./refueling-form-page";
import { RegistrationPage } from "./registration-page";
import { VehicleFormPage } from "./vehicle-form-page";
import { VehiclePage } from "./vehicle-page";
import { VehiclesPage } from "./vehicles-page";

type PageEntry = {
  page: new () => SpotrebaPage;
  /** Reachable without signing in. */
  public?: boolean;
};

// Typed over every route name, so a route without a page does not compile.
export const pages: Record<RouteName, PageEntry> = {
  dashboard: { page: DashboardPage },
  login: { page: LoginPage, public: true },
  registration: { page: RegistrationPage, public: true },
  "password-request": { page: PasswordRequestPage, public: true },
  "password-edit": { page: PasswordResetPage, public: true },
  vehicles: { page: VehiclesPage },
  "vehicle-new": { page: VehicleFormPage },
  "vehicle-show": { page: VehiclePage },
  "vehicle-edit": { page: VehicleFormPage },
  "refueling-new": { page: RefuelingFormPage },
  "refueling-edit": { page: RefuelingFormPage },
  "cost-new": { page: CostFormPage },
  "cost-edit": { page: CostFormPage },
  reminders: { page: RemindersPage },
  "reminder-new": { page: ReminderFormPage },
  "reminder-edit": { page: ReminderFormPage },
};
