class SendMaintenanceNotificationJob < ApplicationJob
  queue_as :default

  retry_on StandardError, wait: :polynomially_longer, attempts: 3
  discard_on ActiveRecord::RecordNotFound

  after_discard do |job, _error|
    MaintenanceNotification.find_by(id: job.arguments.first)&.update(status: :failed)
  end

  def perform(notification_id)
    notification = MaintenanceNotification.find(notification_id)

    notification.with_lock do
      return unless notification.queued?

      MaintenanceMailer.maintenance_due(notification).deliver_now
      notification.update!(status: :sent, sent_at: Time.current)
    end
  end
end
