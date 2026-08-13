class CreateMaintenanceReminderRules < ActiveRecord::Migration[8.1]
  def change
    create_table :maintenance_reminder_rules do |t|
      t.references :vehicle, null: false, foreign_key: true
      t.integer :kind, null: false
      t.integer :interval_days
      t.decimal :interval_km, precision: 10, scale: 2
      t.boolean :active, null: false, default: true

      t.timestamps
    end

    add_index :maintenance_reminder_rules, [ :vehicle_id, :kind ], unique: true
    add_check_constraint :maintenance_reminder_rules,
      "interval_days IS NOT NULL OR interval_km IS NOT NULL",
      name: "maintenance_reminder_rules_interval_present"
    add_check_constraint :maintenance_reminder_rules,
      "interval_days IS NULL OR interval_days > 0",
      name: "maintenance_reminder_rules_interval_days_positive"
    add_check_constraint :maintenance_reminder_rules,
      "interval_km IS NULL OR interval_km > 0",
      name: "maintenance_reminder_rules_interval_km_positive"
  end
end
