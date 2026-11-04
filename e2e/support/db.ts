import bcrypt from "bcryptjs";
import pg from "pg";
import { env } from "./env";

// Numeric columns come back as strings, keeping exact decimals.
export const pool = new pg.Pool({ connectionString: env.databaseUrl, max: 4 });

export const FUEL_TYPES = { petrol: 0, diesel: 1, electricity: 2 } as const;
export const KINDS = {
  oil_change: 0,
  fuel_filter_change: 1,
  technical_inspection: 2,
  emissions_inspection: 3,
  tire_change: 4,
  repair: 5,
  other: 6,
} as const;
export const NOTIFICATION_KINDS = { advance: 0, due: 1 } as const;
export const NOTIFICATION_STATUSES = { queued: 0, sent: 1, skipped: 2, failed: 3 } as const;

export type FuelType = keyof typeof FUEL_TYPES;
export type Kind = keyof typeof KINDS;

const NOW = "(now() at time zone 'utc')";

export async function query<T extends pg.QueryResultRow = pg.QueryResultRow>(sql: string, params: unknown[] = []) {
  return (await pool.query<T>(sql, params)).rows;
}

async function insert(sql: string, params: unknown[]): Promise<number> {
  const rows = await query<{ id: string }>(sql, params);
  return Number(rows[0].id);
}

export async function createUser(email: string, password: string): Promise<number> {
  const digest = bcrypt.hashSync(password, 4);
  return insert(
    `INSERT INTO users (email_address, password_digest, created_at, updated_at)
     VALUES ($1, $2, ${NOW}, ${NOW}) RETURNING id`,
    [email, digest],
  );
}

export async function createVehicle(userId: number, name: string, fuelType: FuelType = "petrol", createdAt?: string) {
  return insert(
    `INSERT INTO vehicles (user_id, name, fuel_type, created_at, updated_at)
     VALUES ($1, $2, $3, COALESCE($4::timestamp, ${NOW}), COALESCE($4::timestamp, ${NOW})) RETURNING id`,
    [userId, name, FUEL_TYPES[fuelType], createdAt ?? null],
  );
}

export async function createRefueling(
  vehicleId: number,
  r: { on: string; distance: string | number; amount: string | number; cost: string | number; createdAt?: string },
) {
  return insert(
    `INSERT INTO refuelings (vehicle_id, refueled_on, distance_km, amount, cost, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamp, ${NOW}), COALESCE($6::timestamp, ${NOW})) RETURNING id`,
    [vehicleId, r.on, r.distance, r.amount, r.cost, r.createdAt ?? null],
  );
}

export async function createAdditionalCost(
  vehicleId: number,
  c: { on: string; kind: Kind; cost: string | number; comment?: string | null; createdAt?: string },
) {
  return insert(
    `INSERT INTO additional_costs (vehicle_id, occurred_on, kind, cost, comment, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamp, ${NOW}), COALESCE($6::timestamp, ${NOW})) RETURNING id`,
    [vehicleId, c.on, KINDS[c.kind], c.cost, c.comment ?? null, c.createdAt ?? null],
  );
}

export async function createRule(
  vehicleId: number,
  r: { kind: Kind; intervalDays?: number | null; intervalKm?: string | number | null; active?: boolean; createdAt?: string },
) {
  return insert(
    `INSERT INTO maintenance_reminder_rules (vehicle_id, kind, interval_days, interval_km, active, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamp, ${NOW}), COALESCE($6::timestamp, ${NOW})) RETURNING id`,
    [vehicleId, KINDS[r.kind], r.intervalDays ?? null, r.intervalKm ?? null, r.active ?? true, r.createdAt ?? null],
  );
}

export async function createLead(
  ruleId: number,
  l: { daysBefore?: number | null; kilometresBefore?: string | number | null; createdAt?: string },
) {
  return insert(
    `INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id, days_before, kilometres_before, created_at, updated_at)
     VALUES ($1, $2, $3, COALESCE($4::timestamp, ${NOW}), COALESCE($4::timestamp, ${NOW})) RETURNING id`,
    [ruleId, l.daysBefore ?? null, l.kilometresBefore ?? null, l.createdAt ?? null],
  );
}

export async function createNotification(n: {
  ruleId: number;
  leadId?: number | null;
  costId: number;
  kind: keyof typeof NOTIFICATION_KINDS;
  trigger: "date" | "mileage";
  status: keyof typeof NOTIFICATION_STATUSES;
  daysRemaining?: number | null;
  kilometresRemaining?: string | null;
  sentAt?: string | null;
  createdAt?: string;
}) {
  return insert(
    `INSERT INTO maintenance_notifications (maintenance_reminder_rule_id, maintenance_reminder_lead_id, additional_cost_id,
       notification_kind, trigger_condition, status, days_remaining, kilometres_remaining, sent_at, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10::timestamp, ${NOW}), COALESCE($10::timestamp, ${NOW})) RETURNING id`,
    [
      n.ruleId,
      n.leadId ?? null,
      n.costId,
      NOTIFICATION_KINDS[n.kind],
      n.trigger === "date" ? 0 : 1,
      NOTIFICATION_STATUSES[n.status],
      n.daysRemaining ?? null,
      n.kilometresRemaining ?? null,
      n.sentAt ?? null,
      n.createdAt ?? null,
    ],
  );
}

export async function count(table: string, userId: number): Promise<number> {
  const scoped: Record<string, string> = {
    vehicles: "SELECT count(*) FROM vehicles WHERE user_id = $1",
    sessions: "SELECT count(*) FROM sessions WHERE user_id = $1",
    refuelings: "SELECT count(*) FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1",
    additional_costs:
      "SELECT count(*) FROM additional_costs c JOIN vehicles v ON v.id = c.vehicle_id WHERE v.user_id = $1",
    maintenance_reminder_rules:
      "SELECT count(*) FROM maintenance_reminder_rules r JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1",
    maintenance_reminder_leads: `SELECT count(*) FROM maintenance_reminder_leads l
       JOIN maintenance_reminder_rules r ON r.id = l.maintenance_reminder_rule_id
       JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1`,
    maintenance_notifications: `SELECT count(*) FROM maintenance_notifications n
       JOIN maintenance_reminder_rules r ON r.id = n.maintenance_reminder_rule_id
       JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1`,
  };
  const rows = await query<{ count: string }>(scoped[table], [userId]);
  return Number(rows[0].count);
}
