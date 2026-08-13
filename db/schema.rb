# This file is auto-generated from the current state of the database. Instead
# of editing this file, please use the migrations feature of Active Record to
# incrementally modify your database, and then regenerate this schema definition.
#
# This file is the source Rails uses to define your schema when running `bin/rails
# db:schema:load`. When creating a new database, `bin/rails db:schema:load` tends to
# be faster and is potentially less error prone than running all of your
# migrations from scratch. Old migrations may fail to apply correctly if those
# migrations use external dependencies or application code.
#
# It's strongly recommended that you check this file into your version control system.

ActiveRecord::Schema[8.1].define(version: 2026_08_13_130002) do
  # These are extensions that must be enabled in order to support this database
  enable_extension "pg_catalog.plpgsql"

  create_table "additional_costs", force: :cascade do |t|
    t.decimal "cost", precision: 10, scale: 2, null: false
    t.datetime "created_at", null: false
    t.integer "kind", null: false
    t.date "occurred_on", null: false
    t.datetime "updated_at", null: false
    t.bigint "vehicle_id", null: false
    t.index ["vehicle_id", "occurred_on"], name: "index_additional_costs_on_vehicle_id_and_occurred_on"
    t.index ["vehicle_id"], name: "index_additional_costs_on_vehicle_id"
  end

  create_table "maintenance_notifications", force: :cascade do |t|
    t.bigint "additional_cost_id", null: false
    t.datetime "created_at", null: false
    t.integer "days_remaining"
    t.decimal "kilometres_remaining", precision: 10, scale: 2
    t.bigint "maintenance_reminder_lead_id"
    t.bigint "maintenance_reminder_rule_id", null: false
    t.integer "notification_kind", null: false
    t.datetime "sent_at"
    t.integer "status", default: 0, null: false
    t.integer "trigger_condition", null: false
    t.datetime "updated_at", null: false
    t.index ["additional_cost_id"], name: "index_maintenance_notifications_on_additional_cost_id"
    t.index ["maintenance_reminder_lead_id"], name: "idx_on_maintenance_reminder_lead_id_8855203404"
    t.index ["maintenance_reminder_rule_id", "additional_cost_id", "maintenance_reminder_lead_id"], name: "index_maintenance_notifications_unique_advance", unique: true, where: "(notification_kind = 0)"
    t.index ["maintenance_reminder_rule_id", "additional_cost_id"], name: "index_maintenance_notifications_unique_due", unique: true, where: "(notification_kind = 1)"
    t.index ["maintenance_reminder_rule_id"], name: "idx_on_maintenance_reminder_rule_id_d1924928ac"
    t.check_constraint "notification_kind = 0 AND maintenance_reminder_lead_id IS NOT NULL OR notification_kind = 1 AND maintenance_reminder_lead_id IS NULL", name: "maintenance_notifications_lead_matches_kind"
    t.check_constraint "notification_kind = ANY (ARRAY[0, 1])", name: "maintenance_notifications_kind_valid"
    t.check_constraint "status = ANY (ARRAY[0, 1, 2, 3])", name: "maintenance_notifications_status_valid"
    t.check_constraint "trigger_condition = ANY (ARRAY[0, 1])", name: "maintenance_notifications_condition_valid"
  end

  create_table "maintenance_reminder_leads", force: :cascade do |t|
    t.datetime "created_at", null: false
    t.integer "days_before"
    t.decimal "kilometres_before", precision: 10, scale: 2
    t.bigint "maintenance_reminder_rule_id", null: false
    t.datetime "updated_at", null: false
    t.index ["maintenance_reminder_rule_id", "days_before"], name: "idx_on_maintenance_reminder_rule_id_days_before_9d0bdbafc7", unique: true, where: "(days_before IS NOT NULL)"
    t.index ["maintenance_reminder_rule_id", "kilometres_before"], name: "idx_on_maintenance_reminder_rule_id_kilometres_befo_c419955954", unique: true, where: "(kilometres_before IS NOT NULL)"
    t.index ["maintenance_reminder_rule_id"], name: "idx_on_maintenance_reminder_rule_id_bf47e7d936"
    t.check_constraint "days_before IS NOT NULL AND kilometres_before IS NULL OR days_before IS NULL AND kilometres_before IS NOT NULL", name: "maintenance_reminder_leads_single_dimension"
    t.check_constraint "days_before IS NULL OR days_before > 0", name: "maintenance_reminder_leads_days_positive"
    t.check_constraint "kilometres_before IS NULL OR kilometres_before > 0::numeric", name: "maintenance_reminder_leads_kilometres_positive"
  end

  create_table "maintenance_reminder_rules", force: :cascade do |t|
    t.boolean "active", default: true, null: false
    t.datetime "created_at", null: false
    t.integer "interval_days"
    t.decimal "interval_km", precision: 10, scale: 2
    t.integer "kind", null: false
    t.datetime "updated_at", null: false
    t.bigint "vehicle_id", null: false
    t.index ["vehicle_id", "kind"], name: "index_maintenance_reminder_rules_on_vehicle_id_and_kind", unique: true
    t.index ["vehicle_id"], name: "index_maintenance_reminder_rules_on_vehicle_id"
    t.check_constraint "interval_days IS NOT NULL OR interval_km IS NOT NULL", name: "maintenance_reminder_rules_interval_present"
    t.check_constraint "interval_days IS NULL OR interval_days > 0", name: "maintenance_reminder_rules_interval_days_positive"
    t.check_constraint "interval_km IS NULL OR interval_km > 0::numeric", name: "maintenance_reminder_rules_interval_km_positive"
  end

  create_table "refuelings", force: :cascade do |t|
    t.decimal "amount", precision: 10, scale: 2, null: false
    t.decimal "cost", precision: 10, scale: 2, null: false
    t.datetime "created_at", null: false
    t.decimal "distance_km", precision: 10, scale: 2, null: false
    t.date "refueled_on", null: false
    t.datetime "updated_at", null: false
    t.bigint "vehicle_id", null: false
    t.index ["vehicle_id", "refueled_on"], name: "index_refuelings_on_vehicle_id_and_refueled_on"
    t.index ["vehicle_id"], name: "index_refuelings_on_vehicle_id"
  end

  create_table "sessions", force: :cascade do |t|
    t.datetime "created_at", null: false
    t.string "ip_address"
    t.datetime "updated_at", null: false
    t.string "user_agent"
    t.bigint "user_id", null: false
    t.index ["user_id"], name: "index_sessions_on_user_id"
  end

  create_table "users", force: :cascade do |t|
    t.datetime "created_at", null: false
    t.string "email_address", null: false
    t.string "password_digest", null: false
    t.datetime "updated_at", null: false
    t.index ["email_address"], name: "index_users_on_email_address", unique: true
  end

  create_table "vehicles", force: :cascade do |t|
    t.datetime "created_at", null: false
    t.integer "fuel_type", null: false
    t.string "name", null: false
    t.datetime "updated_at", null: false
    t.bigint "user_id", null: false
    t.index ["user_id", "name"], name: "index_vehicles_on_user_id_and_name"
    t.index ["user_id"], name: "index_vehicles_on_user_id"
  end

  add_foreign_key "additional_costs", "vehicles"
  add_foreign_key "maintenance_notifications", "additional_costs"
  add_foreign_key "maintenance_notifications", "maintenance_reminder_leads"
  add_foreign_key "maintenance_notifications", "maintenance_reminder_rules"
  add_foreign_key "maintenance_reminder_leads", "maintenance_reminder_rules"
  add_foreign_key "maintenance_reminder_rules", "vehicles"
  add_foreign_key "refuelings", "vehicles"
  add_foreign_key "sessions", "users"
  add_foreign_key "vehicles", "users"
end
