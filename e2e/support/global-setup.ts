import pg from "pg";
import { env } from "./env";
import { deleteAllMessages } from "./mail";

const TABLES = [
  "maintenance_notifications",
  "maintenance_reminder_leads",
  "maintenance_reminder_rules",
  "additional_costs",
  "refuelings",
  "vehicles",
  "sessions",
  "users",
];

export default async function globalSetup() {
  const client = new pg.Client({ connectionString: env.databaseUrl });
  await client.connect();
  try {
    const { rows } = await client.query<{ db: string }>("SELECT current_database() AS db");
    if (!rows[0].db.endsWith("_e2e")) {
      throw new Error(`Refusing to truncate non-e2e database "${rows[0].db}"`);
    }
    await client.query(`TRUNCATE ${TABLES.join(", ")} RESTART IDENTITY CASCADE`);
  } finally {
    await client.end();
  }
  await deleteAllMessages();
}
