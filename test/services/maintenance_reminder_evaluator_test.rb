require "test_helper"

class MaintenanceReminderEvaluatorTest < ActiveSupport::TestCase
  setup do
    @rule = maintenance_reminder_rules(:oil_change)
    @baseline = additional_costs(:one)
  end

  test "queues a date-based advance notification once" do
    result = MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 336).evaluate

    notification = result.queued_notification
    assert_equal maintenance_reminder_leads(:thirty_days), notification.maintenance_reminder_lead
    assert_predicate notification, :advance?
    assert_predicate notification, :date?
    assert_equal 29, notification.days_remaining
    assert_nil MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 336).evaluate.queued_notification
  end

  test "queues a due notification when mileage reaches its threshold first" do
    Refueling.create!(
      vehicle: @rule.vehicle,
      refueled_on: @baseline.occurred_on + 1,
      distance_km: 15_000,
      amount: 20,
      cost: 30
    )

    result = MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 1).evaluate

    assert_predicate result.queued_notification, :due?
    assert_predicate result.queued_notification, :mileage?
    assert_equal 2, result.skipped_notifications.size
  end

  test "does not count a refueling on the maintenance date" do
    Refueling.create!(
      vehicle: @rule.vehicle,
      refueled_on: @baseline.occurred_on,
      distance_km: 15_000,
      amount: 20,
      cost: 30
    )

    result = MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 1).evaluate

    assert_nil result.queued_notification
  end

  test "queues only the most urgent eligible advance lead" do
    urgent_lead = @rule.maintenance_reminder_leads.create!(days_before: 7)

    result = MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 359).evaluate

    assert_equal urgent_lead, result.queued_notification.maintenance_reminder_lead
    assert_equal [ maintenance_reminder_leads(:thirty_days) ], result.skipped_notifications.map(&:maintenance_reminder_lead)
  end

  test "does not notify before a matching maintenance cost exists" do
    rule = MaintenanceReminderRule.create!(vehicle: vehicles(:one), kind: :repair, interval_days: 180)

    result = MaintenanceReminderEvaluator.new(rule, today: Date.current).evaluate

    assert_nil result.queued_notification
    assert_empty result.skipped_notifications
  end

  test "uses a newer matching cost as a new notification cycle" do
    first_notification = MaintenanceReminderEvaluator.new(@rule, today: @baseline.occurred_on + 336).evaluate.queued_notification
    next_cost = AdditionalCost.create!(
      vehicle: @rule.vehicle,
      kind: :oil_change,
      occurred_on: @baseline.occurred_on + 400,
      cost: 50
    )

    next_notification = MaintenanceReminderEvaluator.new(@rule, today: next_cost.occurred_on + 336).evaluate.queued_notification

    assert_equal @baseline, first_notification.additional_cost
    assert_equal next_cost, next_notification.additional_cost
    assert_equal maintenance_reminder_leads(:thirty_days), next_notification.maintenance_reminder_lead
  end
end
