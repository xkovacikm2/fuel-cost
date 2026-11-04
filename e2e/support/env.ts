import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const databaseUrl = process.env.E2E_DATABASE_URL ?? "postgres://postgres:postgres@db:5432/spotreba_go_e2e";
const serverDirectory = process.env.REMINDERS_CWD ?? path.join(repoRoot, "server");
const secretKey = process.env.SECRET_KEY ?? "development-only-secret-key-change-me";
const mailpitUrl = process.env.MAILPIT_URL ?? "http://127.0.0.1:8025";
const smtpAddress = process.env.SMTP_ADDRESS ?? new URL(mailpitUrl).hostname;

export const env = {
  databaseUrl,
  mailpitUrl,
  remindersCmd:
    process.env.REMINDERS_CMD ??
    `DATABASE_URL=${databaseUrl} SECRET_KEY=${secretKey} APP_TZ=${process.env.APP_TZ ?? "Europe/Bratislava"} ` +
      `SMTP_ADDRESS=${smtpAddress} SMTP_PORT=${process.env.SMTP_PORT ?? "1025"} ` +
      `MAILER_FROM=${process.env.MAILER_FROM ?? "spotreba@localhost"} go run ./cmd/spotreba check-reminders`,
  remindersCwd: serverDirectory,
  updateGolden: process.env.UPDATE_GOLDEN === "1",
};

export const backend = {
  exportPath: "/api/user_data/export",
  createVehicle: {
    path: "/api/vehicles",
    json: { name: "Cudzie vozidlo", fuel_type: "petrol" },
  },
};
