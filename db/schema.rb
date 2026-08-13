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

ActiveRecord::Schema[8.1].define(version: 2026_08_13_120000) do
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
  add_foreign_key "refuelings", "vehicles"
  add_foreign_key "sessions", "users"
  add_foreign_key "vehicles", "users"
end
