#!/bin/sh
set -eu

cd /workspace/spotreba

for database in spotreba_development spotreba_go_e2e; do
  if ! PGPASSWORD=postgres psql -h db -U postgres -d "$database" -c 'SELECT 1' >/dev/null 2>&1; then
    PGPASSWORD=postgres createdb -h db -U postgres "$database"
  fi
done

(cd server && go mod download)
(cd web && npm ci && npm run build)
(cd e2e && npm ci && npx playwright install chromium)

DATABASE_URL=postgres://postgres:postgres@db:5432/spotreba_development?sslmode=disable \
  go -C server run ./cmd/spotreba migrate
DATABASE_URL=postgres://postgres:postgres@db:5432/spotreba_go_e2e?sslmode=disable \
  go -C server run ./cmd/spotreba migrate