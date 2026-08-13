require "test_helper"

class CheckMaintenanceRemindersJobTest < ActiveJob::TestCase
  test "queues delivery for a newly eligible notification" do
    maintenance_reminder_rules(:oil_change).update!(interval_days: 30)

    assert_enqueued_with(job: SendMaintenanceNotificationJob) do
      CheckMaintenanceRemindersJob.perform_now(today: Date.new(2026, 8, 13))
    end

    assert_equal 1, MaintenanceNotification.queued.count
  end
end
