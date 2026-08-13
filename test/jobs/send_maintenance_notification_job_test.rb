require "test_helper"

class SendMaintenanceNotificationJobTest < ActiveJob::TestCase
  test "delivers a queued notification once and marks it as sent" do
    notification = MaintenanceNotification.create!(
      maintenance_reminder_rule: maintenance_reminder_rules(:oil_change),
      maintenance_reminder_lead: maintenance_reminder_leads(:thirty_days),
      additional_cost: additional_costs(:one),
      notification_kind: :advance,
      trigger_condition: :date,
      status: :queued,
      days_remaining: 30,
      kilometres_remaining: 1_000
    )

    assert_difference("ActionMailer::Base.deliveries.size", 1) do
      SendMaintenanceNotificationJob.perform_now(notification.id)
    end

    assert_predicate notification.reload, :sent?
    assert_not_nil notification.sent_at
  end
end
