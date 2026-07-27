# Copilot Instructions

## Project Overview

Spotreba Fuel Tracker is a Ruby on Rails 8 application for tracking vehicle fuel usage and costs.

The app supports:
- Open registration and password-based login
- Multiple vehicles per user
- Refueling records with distance since last refueling (km), amount to full tank, and cost
- Dashboard charting for consumption per 100 km and cost per 100 km
- Selectable dashboard scope (single vehicle or combined)
- PWA install support and responsive mobile-first UI

## Technology Stack

- Language: Ruby 3.x
- Framework: Rails 8.x
- Database: PostgreSQL 16
- Jobs/Queue: Active Job with Solid Queue
- Assets: Propshaft + importmap-rails
- Frontend: Hotwire (Turbo + Stimulus)
- Charting: Chart.js (via importmap)
- Development: DevContainer + Docker Compose

## Setup & Development Commands

```bash
# Install dependencies
bundle install

# Set up database
bin/rails db:create db:migrate

# Start app
bin/rails server
```

## Testing, Lint, Security

```bash
# Tests
bin/rails test

# Lint
bin/rubocop

# Security
bin/brakeman
bin/bundler-audit
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

- Keep business logic in models and plain Ruby methods; keep controllers thin.
- Always scope reads/writes by `Current.user`.
- Use `decimal` with precision/scale for measured values and money.
- Prefer reversible migrations and explicit constraints (`null: false`, indexes).
- Follow Omakase Rails style and keep RuboCop clean.

## Security Expectations

- Never trust incoming IDs for ownership; verify associations belong to `Current.user`.
- Never commit secrets.
- Keep CSRF protections and secure session defaults enabled.

## Pull Request Guidelines

- Run `bin/rails test`, `bin/rubocop`, and `bin/brakeman` before requesting review.
- Keep changes focused and avoid unrelated refactors.
- Update tests for behavior changes.
