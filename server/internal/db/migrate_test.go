package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func migrationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MIGRATION_TEST_DATABASE_URL to run PostgreSQL migration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	})

	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrateCreatesApplicationSchema(t *testing.T) {
	db := migrationTestDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var tableCount int
	err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
		  AND table_type = 'BASE TABLE'
		  AND table_name IN ('users', 'sessions', 'vehicles', 'refuelings', 'additional_costs',
		                     'maintenance_reminder_rules', 'maintenance_reminder_leads', 'maintenance_notifications')
	`).Scan(&tableCount)
	if err != nil {
		t.Fatal(err)
	}
	if tableCount != 8 {
		t.Fatalf("application table count = %d, want 8", tableCount)
	}

	var requiredColumns int
	err = db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND (table_name, column_name) IN (
		    ('users', 'email_address'),
		    ('users', 'password_digest'),
		    ('refuelings', 'distance_km'),
		    ('additional_costs', 'comment'),
		    ('maintenance_reminder_rules', 'interval_km'),
		    ('maintenance_reminder_leads', 'kilometres_before'),
		    ('maintenance_notifications', 'trigger_condition')
		  )
	`).Scan(&requiredColumns)
	if err != nil {
		t.Fatal(err)
	}
	if requiredColumns != 7 {
		t.Fatalf("required migration column count = %d, want 7", requiredColumns)
	}

	var constraintCount int
	err = db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE connamespace = current_schema()::regnamespace
		  AND contype = 'c'
	`).Scan(&constraintCount)
	if err != nil {
		t.Fatal(err)
	}
	if constraintCount != 10 {
		t.Fatalf("check constraint count = %d, want 10", constraintCount)
	}

	var foreignKeyCount int
	err = db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE connamespace = current_schema()::regnamespace
		  AND contype = 'f'
	`).Scan(&foreignKeyCount)
	if err != nil {
		t.Fatal(err)
	}
	if foreignKeyCount != 9 {
		t.Fatalf("foreign key count = %d, want 9", foreignKeyCount)
	}
}

func TestMigrateIsIdempotentAndPreservesExistingRows(t *testing.T) {
	db := migrationTestDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `
		CREATE TABLE users (
			id bigserial PRIMARY KEY,
			email_address character varying NOT NULL,
			password_digest character varying NOT NULL,
			created_at timestamp(6) without time zone NOT NULL,
			updated_at timestamp(6) without time zone NOT NULL
		);
		INSERT INTO users (email_address, password_digest, created_at, updated_at)
		VALUES ('existing@example.com', '$2a$existing', now(), now());
	`)
	if err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var email, digest string
	if err := db.QueryRowContext(ctx, "SELECT email_address, password_digest FROM users WHERE id = 1").Scan(&email, &digest); err != nil {
		t.Fatal(err)
	}
	if email != "existing@example.com" || digest != "$2a$existing" {
		t.Fatalf("existing user changed: email=%q digest=%q", email, digest)
	}

	var duplicateEmailIndex int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND indexname = 'index_users_on_email_address'
	`).Scan(&duplicateEmailIndex); err != nil {
		t.Fatal(err)
	}
	if duplicateEmailIndex != 1 {
		t.Fatalf("unique user email index count = %d, want 1", duplicateEmailIndex)
	}
}

func TestMigrateMatchesReferenceCatalog(t *testing.T) {
	migrated := migrationTestDB(t)
	if err := Migrate(context.Background(), migrated); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	reference, err := sql.Open("pgx", os.Getenv("MIGRATION_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reference.Close() })

	want, err := schemaCatalog(context.Background(), reference)
	if err != nil {
		t.Fatal(err)
	}
	got, err := schemaCatalog(context.Background(), migrated)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("catalog entry count = %d, reference database has %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("catalog entry %d differs:\n  Go:        %s\n  Reference: %s", i, got[i], want[i])
		}
	}
}

func TestAdoptingPopulatedDatabasePreservesAllRowsAndSequences(t *testing.T) {
	database := migrationTestDB(t)
	ctx := t.Context()
	baseline, err := migrationFiles.ReadFile("migrations/00001_baseline.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(baseline), "-- +goose Down")
	if _, err := database.ExecContext(ctx, up); err != nil {
		t.Fatal(err)
	}
	_, err = database.ExecContext(ctx, `
		CREATE TABLE schema_migrations (version varchar PRIMARY KEY);
		INSERT INTO schema_migrations VALUES ('20261103123200');
		CREATE TABLE ar_internal_metadata (key varchar PRIMARY KEY, value varchar);
		INSERT INTO ar_internal_metadata VALUES ('environment', 'production');
		INSERT INTO users VALUES (41, 'existing@example.com', '$2a$legacy', '2020-01-01', '2020-01-02');
		INSERT INTO vehicles VALUES (51, 41, 'Existing car', 1, '2020-01-01', '2020-01-02');
		INSERT INTO sessions VALUES (61, 41, '127.0.0.1', 'legacy browser', '2020-01-01', '2020-01-02');
		INSERT INTO refuelings VALUES (71, 51, '2025-01-03', 512.30, 35.50, 56.80, '2025-01-03', '2025-01-03');
		INSERT INTO additional_costs VALUES (81, 51, '2025-01-02', 0, 89.90, '2025-01-02', '2025-01-02', 'Oil');
		INSERT INTO maintenance_reminder_rules VALUES (91, 51, 0, 365, 15000, true, '2025-01-02', '2025-01-02');
		INSERT INTO maintenance_reminder_leads VALUES (101, 91, 30, null, '2025-01-02', '2025-01-02');
		INSERT INTO maintenance_notifications VALUES (111, 91, 101, 81, 0, 0, 1, 25, 1000, '2025-12-01', '2025-12-01', '2025-12-01');
		SELECT setval('users_id_seq', 450);
		SELECT setval('vehicles_id_seq', 550);
		SELECT setval('sessions_id_seq', 650);
		SELECT setval('refuelings_id_seq', 750);
		SELECT setval('additional_costs_id_seq', 850);
		SELECT setval('maintenance_reminder_rules_id_seq', 950);
		SELECT setval('maintenance_reminder_leads_id_seq', 1050);
		SELECT setval('maintenance_notifications_id_seq', 1150);
	`)
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{"users", "vehicles", "sessions", "refuelings", "additional_costs",
		"maintenance_reminder_rules", "maintenance_reminder_leads", "maintenance_notifications",
		"schema_migrations", "ar_internal_metadata"}
	snapshot := func() map[string]string {
		t.Helper()
		result := make(map[string]string)
		for _, table := range tables {
			var value string
			query := "SELECT COALESCE(jsonb_agg(to_jsonb(record) ORDER BY to_jsonb(record)::text), '[]'::jsonb)::text FROM " + pgx.Identifier{table}.Sanitize() + " record"
			if err := database.QueryRowContext(ctx, query).Scan(&value); err != nil {
				t.Fatal(err)
			}
			result[table] = value
		}
		for _, table := range tables[:8] {
			var last int64
			var called bool
			if err := database.QueryRowContext(ctx, "SELECT last_value, is_called FROM "+pgx.Identifier{table + "_id_seq"}.Sanitize()).Scan(&last, &called); err != nil {
				t.Fatal(err)
			}
			result[table+"_sequence"] = fmt.Sprintf("%d:%t", last, called)
		}
		return result
	}
	before := snapshot()
	for attempt := 0; attempt < 2; attempt++ {
		if err := Migrate(ctx, database); err != nil {
			t.Fatal(err)
		}
		if after := snapshot(); !reflect.DeepEqual(before, after) {
			t.Fatalf("migration changed inherited records or sequences:\nbefore: %v\nafter: %v", before, after)
		}
	}
}

func schemaCatalog(ctx context.Context, database *sql.DB) ([]string, error) {
	entries := make([]string, 0, 100)
	columns, err := database.QueryContext(ctx, `
		SELECT c.relname::text || '.' || a.attname::text || '|' || pg_catalog.format_type(a.atttypid, a.atttypmod) || '|' ||
		       a.attnotnull::text || '|' ||
		       CASE WHEN pg_get_expr(d.adbin, d.adrelid) LIKE 'nextval(%' THEN 'serial'
		            ELSE COALESCE(pg_get_expr(d.adbin, d.adrelid), '') END
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
		WHERE n.nspname = current_schema() AND c.relkind = 'r' AND a.attnum > 0 AND NOT a.attisdropped
		  AND c.relname IN ('users', 'sessions', 'vehicles', 'refuelings', 'additional_costs',
		                    'maintenance_reminder_rules', 'maintenance_reminder_leads', 'maintenance_notifications')
		ORDER BY c.relname, a.attnum
	`)
	if err != nil {
		return nil, err
	}
	for columns.Next() {
		var entry string
		if err := columns.Scan(&entry); err != nil {
			_ = columns.Close()
			return nil, err
		}
		entries = append(entries, "column|"+entry)
	}
	if err := columns.Close(); err != nil {
		return nil, err
	}
	if err := columns.Err(); err != nil {
		return nil, err
	}

	constraints, err := database.QueryContext(ctx, `
		SELECT r.relname::text || '|' || c.contype::text || '|' || pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class r ON r.oid = c.conrelid
		WHERE c.connamespace = current_schema()::regnamespace
		  AND r.relname IN ('users', 'sessions', 'vehicles', 'refuelings', 'additional_costs',
		                    'maintenance_reminder_rules', 'maintenance_reminder_leads', 'maintenance_notifications')
		ORDER BY r.relname, c.contype, pg_get_constraintdef(c.oid)
	`)
	if err != nil {
		return nil, err
	}
	for constraints.Next() {
		var entry string
		if err := constraints.Scan(&entry); err != nil {
			_ = constraints.Close()
			return nil, err
		}
		entries = append(entries, "constraint|"+entry)
	}
	if err := constraints.Close(); err != nil {
		return nil, err
	}
	if err := constraints.Err(); err != nil {
		return nil, err
	}

	indexes, err := database.QueryContext(ctx, `
		SELECT tablename || '|' || indexname || '|' || regexp_replace(indexdef, ' ON [^ ]+[.]', ' ON ')
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename IN ('users', 'sessions', 'vehicles', 'refuelings', 'additional_costs',
	                    'maintenance_reminder_rules', 'maintenance_reminder_leads', 'maintenance_notifications')
		ORDER BY tablename, indexname
	`)
	if err != nil {
		return nil, err
	}
	for indexes.Next() {
		var entry string
		if err := indexes.Scan(&entry); err != nil {
			_ = indexes.Close()
			return nil, err
		}
		entries = append(entries, "index|"+entry)
	}
	if err := indexes.Close(); err != nil {
		return nil, err
	}
	return entries, indexes.Err()
}
