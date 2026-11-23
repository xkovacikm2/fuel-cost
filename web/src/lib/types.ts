import type { ChartPayload } from "../chart";

export type Vehicle = { id: number; name: string; fuel_type: string; fuel_type_label: string; unit: string };
export type Refueling = { id?: number; vehicle_id: number | null; refueled_on: string; distance_km?: string; amount?: string; cost?: string };
export type User = { id: number; email_address: string };
export type Cost = { id?: number; vehicle_id: number | null; occurred_on: string; kind: string; comment: string | null; cost: string };
export type Lead = { id?: number; days_before: number | null; kilometres_before: string | null; time_value?: string; time_unit?: string };
export type Reminder = {
  id?: number; vehicle_id: number | null; vehicle_name?: string; kind: string; kind_label?: string;
  interval_days: number | null; interval_km: string | null; interval_time_value?: string; interval_time_unit?: string;
  active: boolean; leads: Lead[]; status?: string;
};

export type RefuelingRow = Refueling & { id: number; vehicle_name: string; unit: string; consumption_per_100km: string; cost_per_100km: string };
export type CostRow = Cost & { id: number; vehicle_name: string; kind_label: string };

export type Dashboard = {
  scope: string; offset: number; selected_vehicle_id: number | null;
  chart: ChartPayload;
  stats: {
    days_since_last_refueling: number | null; average_consumption_per_100km: string | null; average_days_between_refuelings: string | null;
    total_cost_paid: string; total_fuel_refilled: string; total_fuel_cost_paid: string; total_additional_cost_paid: string;
  };
  refuelings: { page: number; total_pages: number; total_count: number; items: RefuelingRow[] };
  additional_costs: CostRow[];
};
