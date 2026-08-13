class CreateMaintenanceReminderLeads < ActiveRecord::Migration[8.1]
  def change
    create_table :maintenance_reminder_leads do |t|
      t.references :maintenance_reminder_rule, null: false, foreign_key: true
      t.integer :days_before
      t.decimal :kilometres_before, precision: 10, scale: 2

      t.timestamps
    end

    add_check_constraint :maintenance_reminder_leads,
      "(days_before IS NOT NULL AND kilometres_before IS NULL) OR (days_before IS NULL AND kilometres_before IS NOT NULL)",
      name: "maintenance_reminder_leads_single_dimension"
    add_check_constraint :maintenance_reminder_leads,
      "days_before IS NULL OR days_before > 0",
      name: "maintenance_reminder_leads_days_positive"
    add_check_constraint :maintenance_reminder_leads,
      "kilometres_before IS NULL OR kilometres_before > 0",
      name: "maintenance_reminder_leads_kilometres_positive"
    add_index :maintenance_reminder_leads,
      [ :maintenance_reminder_rule_id, :days_before ],
      unique: true,
      where: "days_before IS NOT NULL"
    add_index :maintenance_reminder_leads,
      [ :maintenance_reminder_rule_id, :kilometres_before ],
      unique: true,
      where: "kilometres_before IS NOT NULL"
  end
end
