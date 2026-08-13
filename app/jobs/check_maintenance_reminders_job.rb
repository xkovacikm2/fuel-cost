class CheckMaintenanceRemindersJob < ApplicationJob
  queue_as :default

  def perform(today: Date.current)
    MaintenanceReminderRule.active.find_each do |rule|
      result = MaintenanceReminderEvaluator.new(rule, today: today).evaluate
      next unless result.queued_notification

      SendMaintenanceNotificationJob.perform_later(result.queued_notification.id)
    end
  end
end
