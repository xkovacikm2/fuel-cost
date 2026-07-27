# Spotreba Fuel Tracker

Spotreba is a Ruby on Rails 8 app for tracking fuel consumption and travel cost.

Users can:
- Register/login with email and password
- Manage multiple vehicles (petrol, diesel, electricity)
- Log refuelings with distance since last refueling (km), fuel amount, and cost
- View consumption and cost trends on a 365-day chart
- Switch dashboard scope between per-vehicle and combined view
- Edit/delete mistakes in refueling records
- Install the app as a PWA on a smartphone

## Tech Stack

- Ruby 3.x
- Rails 8.x
- PostgreSQL 16
- Hotwire (Turbo + Stimulus)
- Chart.js via importmap
- Propshaft asset pipeline
- Solid Queue / Solid Cache / Solid Cable defaults

## Local Development (DevContainer)

The repository includes `.devcontainer` configuration with PostgreSQL.

### 1) Install dependencies

```bash
bundle install
```

### 2) Set up database

```bash
bin/rails db:create db:migrate
```

### 3) Start server

```bash
bin/rails server
```

App runs at http://localhost:3000.

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

## PWA Installability

This app includes:
- Web app manifest at `/manifest.json`
- Service worker at `/service-worker.js`
- Mobile-friendly responsive layout

To install on smartphone:
1. Open the app in a supported browser (Chrome/Safari).
2. Use "Add to Home Screen" / "Install app".

## Quality Commands

```bash
bin/rails test
bin/rubocop
bin/brakeman
```

## Notes

- Registration is intentionally open.
- All records are scoped to the signed-in user.
