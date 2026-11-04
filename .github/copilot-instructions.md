# Copilot Instructions

## Project Overview

Spotreba Fuel Tracker is a Go JSON API with a Lit/TypeScript frontend for tracking vehicle fuel usage and costs.

The app supports:
- Open registration and password-based login
- Multiple vehicles per user
- Refueling records with distance since last refueling (km), amount to full tank, and cost
- Dashboard charting for consumption per 100 km and cost per 100 km
- Selectable dashboard scope (single vehicle or combined)
- PWA install support and responsive mobile-first UI

## Technology Stack

- Backend: Go 1.27+, standard-library HTTP server, pgx, Goose
- Frontend: Lit, TypeScript, Vite, Chart.js
- Database: PostgreSQL 16
- Jobs: Kubernetes CronJob invoking `spotreba check-reminders`
- Assets: Vite build embedded in the Go binary
- Development: DevContainer + Docker Compose

## Setup & Development Commands

```bash
# Install dependencies
cd server && go mod download
cd ../web && npm ci

# Apply schema and run the API
cd ../server && DATABASE_URL=postgres://postgres:postgres@localhost:5432/spotreba_development go run ./cmd/spotreba migrate
DATABASE_URL=postgres://postgres:postgres@localhost:5432/spotreba_development SECRET_KEY=<local-only-secret> go run ./cmd/spotreba serve
cd ../web && npm run dev
```

## Testing, Lint, Security

```bash
# Tests
cd server && MIGRATION_TEST_DATABASE_URL=<isolated-postgres-url> go test ./...
cd web && npm test && npm run typecheck
cd e2e && npm test

# Lint and security
cd server && golangci-lint run ./... && govulncheck ./...
cd web && npm audit
```

## Core Domain Models

- User: account with email and password digest
- Session: persistent login session linked to user
- Vehicle: belongs to user, has name and fuel_type enum (`petrol`, `diesel`, `electricity`)
- Refueling: belongs to vehicle, stores:
	- `refueled_on` (date)
	- `distance_km` (distance since previous refueling)
	- `amount` (litres for petrol/diesel, kWh for electricity)
	- `cost`

Derived metrics:
- Consumption per 100 km = `amount / distance_km * 100`
- Cost per 100 km = `cost / distance_km * 100`

## Product Rules

- Registration is open.
- Data must always be scoped to the signed-in user.
- New refueling form defaults to the most recent refueling vehicle and its unit.
- Dashboard defaults to the vehicle used in the most recent refueling.
- Combined dashboard scope is supported for cross-vehicle view.

## Code Conventions

- Keep business logic in Go domain/service packages; keep HTTP handlers thin.
- Always scope reads/writes by the authenticated user ID.
- Use `decimal` with precision/scale for measured values and money.
- Prefer reversible migrations and explicit constraints (`null: false`, indexes).
- Keep `golangci-lint` clean and use decimal arithmetic for measured values.
- Follow TDD's red-green-refactor workflow when implementing new features or bug fixes: write a failing test, make it pass, then refactor.

## Security Expectations

- Never trust incoming IDs for ownership; verify associations belong to `Current.user`.
- Never commit secrets.
- Keep CSRF protections, signed HttpOnly session cookies, and secure cookie defaults enabled.

## Pull Request Guidelines

- Run Go tests, `golangci-lint`, `govulncheck`, frontend checks, and the Playwright parity suite before requesting review.
- When explicitly asked to poll CI checks, continue checking at the requested interval using an available wait mechanism; merge only after all required checks pass.
- Keep changes focused and avoid unrelated refactors.
- Update tests for behavior changes.
