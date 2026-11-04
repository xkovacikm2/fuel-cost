-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    email_address character varying NOT NULL,
    password_digest character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS index_users_on_email_address ON users (email_address);

CREATE TABLE IF NOT EXISTS sessions (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id),
    ip_address character varying,
    user_agent character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

CREATE INDEX IF NOT EXISTS index_sessions_on_user_id ON sessions (user_id);

CREATE TABLE IF NOT EXISTS vehicles (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id),
    name character varying NOT NULL,
    fuel_type integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

CREATE INDEX IF NOT EXISTS index_vehicles_on_user_id ON vehicles (user_id);
CREATE INDEX IF NOT EXISTS index_vehicles_on_user_id_and_name ON vehicles (user_id, name);

CREATE TABLE IF NOT EXISTS refuelings (
    id bigserial PRIMARY KEY,
    vehicle_id bigint NOT NULL REFERENCES vehicles (id),
    refueled_on date NOT NULL,
    distance_km numeric(10, 2) NOT NULL,
    amount numeric(10, 2) NOT NULL,
    cost numeric(10, 2) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

CREATE INDEX IF NOT EXISTS index_refuelings_on_vehicle_id ON refuelings (vehicle_id);
CREATE INDEX IF NOT EXISTS index_refuelings_on_vehicle_id_and_refueled_on ON refuelings (vehicle_id, refueled_on);

CREATE TABLE IF NOT EXISTS additional_costs (
    id bigserial PRIMARY KEY,
    vehicle_id bigint NOT NULL REFERENCES vehicles (id),
    occurred_on date NOT NULL,
    kind integer NOT NULL,
    cost numeric(10, 2) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    comment character varying(255)
);

CREATE INDEX IF NOT EXISTS index_additional_costs_on_vehicle_id ON additional_costs (vehicle_id);
CREATE INDEX IF NOT EXISTS index_additional_costs_on_vehicle_id_and_occurred_on ON additional_costs (vehicle_id, occurred_on);

CREATE TABLE IF NOT EXISTS maintenance_reminder_rules (
    id bigserial PRIMARY KEY,
    vehicle_id bigint NOT NULL REFERENCES vehicles (id),
    kind integer NOT NULL,
    interval_days integer,
    interval_km numeric(10, 2),
    active boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    CONSTRAINT maintenance_reminder_rules_interval_present CHECK (interval_days IS NOT NULL OR interval_km IS NOT NULL),
    CONSTRAINT maintenance_reminder_rules_interval_days_positive CHECK (interval_days IS NULL OR interval_days > 0),
    CONSTRAINT maintenance_reminder_rules_interval_km_positive CHECK (interval_km IS NULL OR interval_km > 0)
);

CREATE INDEX IF NOT EXISTS index_maintenance_reminder_rules_on_vehicle_id ON maintenance_reminder_rules (vehicle_id);
CREATE UNIQUE INDEX IF NOT EXISTS index_maintenance_reminder_rules_on_vehicle_id_and_kind ON maintenance_reminder_rules (vehicle_id, kind);

CREATE TABLE IF NOT EXISTS maintenance_reminder_leads (
    id bigserial PRIMARY KEY,
    maintenance_reminder_rule_id bigint NOT NULL REFERENCES maintenance_reminder_rules (id),
    days_before integer,
    kilometres_before numeric(10, 2),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    CONSTRAINT maintenance_reminder_leads_single_dimension CHECK (
        (days_before IS NOT NULL AND kilometres_before IS NULL) OR
        (days_before IS NULL AND kilometres_before IS NOT NULL)
    ),
    CONSTRAINT maintenance_reminder_leads_days_positive CHECK (days_before IS NULL OR days_before > 0),
    CONSTRAINT maintenance_reminder_leads_kilometres_positive CHECK (kilometres_before IS NULL OR kilometres_before > 0)
);

CREATE INDEX IF NOT EXISTS idx_on_maintenance_reminder_rule_id_bf47e7d936 ON maintenance_reminder_leads (maintenance_reminder_rule_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_on_maintenance_reminder_rule_id_days_before_9d0bdbafc7 ON maintenance_reminder_leads (maintenance_reminder_rule_id, days_before) WHERE days_before IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_on_maintenance_reminder_rule_id_kilometres_befo_c419955954 ON maintenance_reminder_leads (maintenance_reminder_rule_id, kilometres_before) WHERE kilometres_before IS NOT NULL;

CREATE TABLE IF NOT EXISTS maintenance_notifications (
    id bigserial PRIMARY KEY,
    maintenance_reminder_rule_id bigint NOT NULL REFERENCES maintenance_reminder_rules (id),
    maintenance_reminder_lead_id bigint REFERENCES maintenance_reminder_leads (id),
    additional_cost_id bigint NOT NULL REFERENCES additional_costs (id),
    notification_kind integer NOT NULL,
    trigger_condition integer NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    days_remaining integer,
    kilometres_remaining numeric(10, 2),
    sent_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    CONSTRAINT maintenance_notifications_lead_matches_kind CHECK (
        (notification_kind = 0 AND maintenance_reminder_lead_id IS NOT NULL) OR
        (notification_kind = 1 AND maintenance_reminder_lead_id IS NULL)
    ),
    CONSTRAINT maintenance_notifications_kind_valid CHECK (notification_kind = ANY (ARRAY[0, 1])),
    CONSTRAINT maintenance_notifications_status_valid CHECK (status = ANY (ARRAY[0, 1, 2, 3])),
    CONSTRAINT maintenance_notifications_condition_valid CHECK (trigger_condition = ANY (ARRAY[0, 1]))
);

CREATE INDEX IF NOT EXISTS idx_on_maintenance_reminder_rule_id_d1924928ac ON maintenance_notifications (maintenance_reminder_rule_id);
CREATE INDEX IF NOT EXISTS idx_on_maintenance_reminder_lead_id_8855203404 ON maintenance_notifications (maintenance_reminder_lead_id);
CREATE INDEX IF NOT EXISTS index_maintenance_notifications_on_additional_cost_id ON maintenance_notifications (additional_cost_id);
CREATE UNIQUE INDEX IF NOT EXISTS index_maintenance_notifications_unique_advance
    ON maintenance_notifications (maintenance_reminder_rule_id, additional_cost_id, maintenance_reminder_lead_id)
    WHERE notification_kind = 0;
CREATE UNIQUE INDEX IF NOT EXISTS index_maintenance_notifications_unique_due
    ON maintenance_notifications (maintenance_reminder_rule_id, additional_cost_id)
    WHERE notification_kind = 1;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    RAISE EXCEPTION 'Baseline rollback is disabled to protect inherited data';
END $$;
-- +goose StatementEnd
