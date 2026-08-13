require "test_helper"

class MaintenanceReminderRuleTest < ActiveSupport::TestCase
  test "requires a day or kilometre interval" do
    rule = MaintenanceReminderRule.new(vehicle: vehicles(:one), kind: :oil_change)

    assert_not rule.valid?
    assert_includes rule.errors[:base], "must have a day or kilometre interval"
  end

  test "requires unique vehicle and cost kind" do
    duplicate = MaintenanceReminderRule.new(
      vehicle: vehicles(:one),
      kind: :oil_change,
      interval_days: 365
    )

    assert_not duplicate.valid?
    assert_includes duplicate.errors[:kind], "has already been taken"
  end

  test "for_user returns only rules belonging to that user" do
    assert_equal [ maintenance_reminder_rules(:oil_change), maintenance_reminder_rules(:days_only) ], MaintenanceReminderRule.for_user(users(:one)).order(:kind).to_a
  end

  test "uses additional cost labels" do
    assert_equal "Výmena oleja", maintenance_reminder_rules(:oil_change).kind_label
  end
end
