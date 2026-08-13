require "test_helper"

class MaintenanceReminderLeadTest < ActiveSupport::TestCase
  test "requires exactly one lead dimension" do
    lead = MaintenanceReminderLead.new(maintenance_reminder_rule: maintenance_reminder_rules(:oil_change))

    assert_not lead.valid?
    assert_includes lead.errors[:base], "must have either days or kilometres before"
  end

  test "requires its dimension on the parent rule" do
    lead = MaintenanceReminderLead.new(
      maintenance_reminder_rule: maintenance_reminder_rules(:days_only),
      kilometres_before: 1_000
    )

    assert_not lead.valid?
    assert_includes lead.errors[:kilometres_before], "requires a kilometre interval"
  end

  test "allows a matching lead dimension" do
    lead = MaintenanceReminderLead.new(
      maintenance_reminder_rule: maintenance_reminder_rules(:oil_change),
      days_before: 30
    )

    assert_predicate lead, :valid?
  end
end
