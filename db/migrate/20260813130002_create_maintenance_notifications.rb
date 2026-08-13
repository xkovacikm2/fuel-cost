class CreateMaintenanceNotifications < ActiveRecord::Migration[8.1]
  def change
    create_table :maintenance_notifications do |t|
      t.references :maintenance_reminder_rule, null: false, foreign_key: true
      t.references :maintenance_reminder_lead, foreign_key: true
      t.references :additional_cost, null: false, foreign_key: true
      t.integer :notification_kind, null: false
      t.integer :trigger_condition, null: false
      t.integer :status, null: false, default: 0
      t.integer :days_remaining
      t.decimal :kilometres_remaining, precision: 10, scale: 2
      t.datetime :sent_at

      t.timestamps
    end

    add_check_constraint :maintenance_notifications,
      "notification_kind IN (0, 1)",
      name: "maintenance_notifications_kind_valid"
    add_check_constraint :maintenance_notifications,
      "trigger_condition IN (0, 1)",
      name: "maintenance_notifications_condition_valid"
    add_check_constraint :maintenance_notifications,
      "status IN (0, 1, 2, 3)",
      name: "maintenance_notifications_status_valid"
    add_check_constraint :maintenance_notifications,
      "(notification_kind = 0 AND maintenance_reminder_lead_id IS NOT NULL) OR (notification_kind = 1 AND maintenance_reminder_lead_id IS NULL)",
      name: "maintenance_notifications_lead_matches_kind"
    add_index :maintenance_notifications,
      [ :maintenance_reminder_rule_id, :additional_cost_id, :maintenance_reminder_lead_id ],
      unique: true,
      where: "notification_kind = 0",
      name: "index_maintenance_notifications_unique_advance"
    add_index :maintenance_notifications,
      [ :maintenance_reminder_rule_id, :additional_cost_id ],
      unique: true,
      where: "notification_kind = 1",
      name: "index_maintenance_notifications_unique_due"
  end
end
