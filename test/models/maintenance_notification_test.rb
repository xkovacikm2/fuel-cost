require "test_helper"

class MaintenanceNotificationTest < ActiveSupport::TestCase
  test "requires a lead for an advance notification" do
    notification = MaintenanceNotification.new(
      maintenance_reminder_rule: maintenance_reminder_rules(:oil_change),
      additional_cost: additional_costs(:one),
      notification_kind: :advance,
      trigger_condition: :date
    )

    assert_not notification.valid?
    assert_includes notification.errors[:maintenance_reminder_lead], "is required for an advance notification"
  end

  test "does not allow a lead for a due notification" do
    notification = MaintenanceNotification.new(
      maintenance_reminder_rule: maintenance_reminder_rules(:oil_change),
      maintenance_reminder_lead: maintenance_reminder_leads(:thirty_days),
      additional_cost: additional_costs(:one),
      notification_kind: :due,
      trigger_condition: :date
    )

    assert_not notification.valid?
    assert_includes notification.errors[:maintenance_reminder_lead], "must be blank for a due notification"
  end
end
