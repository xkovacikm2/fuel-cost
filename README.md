# Spotreba Fuel Tracker

[![Tests](https://github.com/xkovacikm2/fuel-cost/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/xkovacikm2/fuel-cost/actions/workflows/test.yml)
[![Build](https://github.com/xkovacikm2/fuel-cost/actions/workflows/docker.yml/badge.svg?branch=main)](https://github.com/xkovacikm2/fuel-cost/actions/workflows/docker.yml)

Spotreba is a Go and Lit app for tracking fuel consumption and travel cost.

Users can:
- Register/login with email and password
- Manage multiple vehicles (petrol, diesel, electricity)
- Log refuelings with distance since last refueling (km), fuel amount, and cost
- View consumption and cost trends on a 365-day chart
- Switch dashboard scope between per-vehicle and combined view
- Edit/delete mistakes in refueling records
- Install the app as a PWA on a smartphone

## Demo
Demo available at [spotreba.kovko.top](https://spotreba.kovko.top)  
Log in with:
<dl>
<dt>email</dt> <dd>demo@demo</dd> 
<dt>password</dt> <dd>demo</dd>
</dl>

## Tech Stack

- Go 1.27+
- Lit and TypeScript
- Vite
- PostgreSQL 16
- Chart.js
- Goose SQL migrations
- PostgreSQL-backed scheduled maintenance reminders

## Local Development (DevContainer)

The repository includes `.devcontainer` configuration with PostgreSQL.

### 1) Install dependencies

```bash
cd server && go mod download
cd ../web && npm ci
cd ../e2e && npm ci && npx playwright install chromium
```

### 2) Set up database

```bash
cd server && DATABASE_URL=postgres://postgres:postgres@localhost:5432/spotreba_development go run ./cmd/spotreba migrate
```

### 3) Start API and frontend

```bash
cd server && DATABASE_URL=postgres://postgres:postgres@localhost:5432/spotreba_development SECRET_KEY=replace-with-a-random-32-byte-secret go run ./cmd/spotreba serve
cd web && npm run dev
```

The API runs at http://localhost:8080 and Vite runs at http://localhost:5173.

## Data Model

### Vehicle
- `name`
- `fuel_type` enum: `petrol`, `diesel`, `electricity`

Unit mapping:
- Petrol/Diesel -> litres (`L`)
- Electricity -> kilowatt-hours (`kWh`)

### Refueling
- `vehicle_id`
- `refueled_on` (date)
- `distance_km` (distance driven since previous refueling)
- `amount` (fuel to full tank)
- `cost`

Derived metrics:
- `consumption_per_100km = amount / distance_km * 100`
- `cost_per_100km = cost / distance_km * 100`

## Dashboard Behavior

- Shows the selected 365-day window (with previous/next navigation)
- Scope selectable:
	- Per vehicle
	- Combined (multiple vehicles)
- Default vehicle is the one from the most recent refueling
- Displays:
	- Consumption per 100 km
	- Cost per 100 km
	- Average days between refuelings
	- Days since last refueling

## Refueling Entry Defaults

When creating a new refueling entry:
- Vehicle is preselected from the most recent refueling
- Unit shown in the form is based on that selected vehicle

## Maintenance Reminders

Users can configure an active reminder for each vehicle and additional-cost type.
Each reminder can have a date interval, a kilometre interval, or both. The latest
matching additional cost starts a new maintenance cycle. When both conditions are
configured, the first one to become due triggers the due notification.

Advanced reminders are optional and can be configured as either a number of days
or kilometres before the base interval. Each advanced reminder sends at most once
per cycle. A due notification sends at most once per cycle and suppresses any
remaining advanced reminders. If the application catches up after downtime, it
sends only the most urgent currently eligible notification.

Mileage is an approximation based on the sum of refueling distances dated after
the matching additional cost. A refueling recorded on the same date as the cost
is not counted. The app does not record absolute odometer readings.

The production scan runs every day at 08:00 in the `Europe/Bratislava` time zone
as a Kubernetes CronJob invoking `spotreba check-reminders`. Each notification is
locked while sent; delivery is retried up to three times before it is marked failed.

### Production Mail Configuration

Set these environment variables in production:

```bash
MAILER_FROM="Spotreba <notifications@example.com>"
APP_HOST="spotreba.example.com"
APP_TZ="Europe/Bratislava"
ASSUME_SSL=true
DATABASE_URL="postgres://spotreba:password@postgres:5432/spotreba_production?sslmode=disable"
SECRET_KEY="a-random-secret-at-least-32-bytes-long"
SMTP_ADDRESS="smtp.example.com"
SMTP_PORT=587
SMTP_USERNAME="smtp-user"
SMTP_PASSWORD="smtp-password"
```

Do not commit SMTP credentials. `SMTP_ADDRESS` and `SMTP_PORT` enable reminder
delivery; the app requires a configured SMTP sender when queued notifications exist.

## JSON Backup and Restore

The **Používateľ** navigation submenu contains JSON export, JSON import, and
logout. Export downloads all owned vehicles, refuelings, additional costs,
maintenance reminder rules and leads, and notification history. Import opens a
file chooser and submits the selected file immediately. The result reports
imported/skipped counts and conflict reasons.

Exports exclude passwords, login sessions, background jobs, and calculated
dashboard metrics. Account email and timestamps are informational: importing
never changes the signed-in account. Treat backup files as private financial data.

### Version 1 Format

The root contains exactly `format`, `version`, `exported_at`, `account`, and
`vehicles`. An empty account produces this valid shape:

```json
{
	"format": "spotreba",
	"version": 1,
	"exported_at": "2026-10-05T12:00:00.000000Z",
	"account": {
		"email_address": "owner@example.com",
		"created_at": "2026-10-01T12:00:00.000000Z",
		"updated_at": "2026-10-01T12:00:00.000000Z"
	},
	"vehicles": []
}
```

Every domain record includes `id`, `created_at`, and `updated_at` plus:

| Collection | Fields |
| --- | --- |
| `vehicles` | `name`, `fuel_type`, `refuelings`, `additional_costs`, `maintenance_reminder_rules`, `maintenance_notifications` |
| `refuelings` | `refueled_on`, `distance_km`, `amount`, `cost` |
| `additional_costs` | `occurred_on`, `kind`, `comment` (optional, may be `null` or omitted), `cost` |
| `maintenance_reminder_rules` | `kind`, `interval_days`, `interval_km`, `active`, `maintenance_reminder_leads` |
| `maintenance_reminder_leads` | `days_before`, `kilometres_before` |
| `maintenance_notifications` | `maintenance_reminder_rule_id`, `maintenance_reminder_lead_id`, `additional_cost_id`, `notification_kind`, `trigger_condition`, `status`, `days_remaining`, `kilometres_remaining`, `sent_at` |

Collections are arrays nested under their vehicle, except leads which are nested
under their rule. Source IDs are positive integers unique within each collection
type and used only to remap relationships, never as destination IDs. Notification
references must resolve inside the same vehicle, with the cost matching the rule
kind and an advance lead belonging to that rule.

Dates use `YYYY-MM-DD`; timestamps use ISO 8601 with a timezone and at most six
fractional digits. Decimal values are strings with at most eight integer digits
and two fractional digits. Enums use the model's string names; `active` is a JSON
boolean. Optional fields must be present with `null` when absent. New records
retain their original timestamps. Unknown/missing fields and unsupported versions
are rejected. Limits: 10 MiB per upload, 100,000 domain records, nesting depth 32.

### Conflicts and Validation

The whole file is validated before writing anything. Invalid JSON, values, or
references reject the entire upload. Valid imports are transactional: unexpected
database failures roll back all newly imported records. Existing records are
never overwritten, and all imported data belongs to the signed-in user.

- Vehicles match exact name plus fuel type. Missing children merge into one
	matching vehicle. No compatible match creates a new vehicle, even if another
	fuel type has the same name. Multiple matches skip that vehicle and its children.
- Refuelings match vehicle, date, distance, amount, and cost; additional costs
	match vehicle, date, kind, and cost. Exact identical records are duplicates even
	if source IDs or timestamps differ; distinct values on the same date import.
- Rules match vehicle and kind. Existing intervals and active state remain
	unchanged. Matching configurations merge missing leads/history; differing
	configurations skip the imported rule, its leads, and related notifications.
- Leads match rule and dimension/value. Notifications match rule, cost, and kind,
	plus lead for advance notifications. Duplicate parents are mapped to existing
	records so missing dependent records can still import.
- Imported queued notifications become skipped; other statuses are preserved.
	Import never sends emails or schedules delivery jobs. Existing notification
	history/status is retained on conflict.

## PWA Installability

This app includes:
- Web app manifest at `/manifest`
- Service worker at `/service-worker`
- Mobile-friendly responsive layout

To install on smartphone:
1. Open the app in a supported browser (Chrome/Safari).
2. Use "Add to Home Screen" / "Install app".

## Quality Commands

```bash
cd server && MIGRATION_TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/spotreba_go_e2e go test ./...
cd web && npm run typecheck && npm test && npm run build
cd e2e && npm test
```

## Notes

- Registration is intentionally open.
- All records are scoped to the signed-in user.
