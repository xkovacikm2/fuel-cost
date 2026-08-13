class MaintenanceNotification < ApplicationRecord
  belongs_to :maintenance_reminder_rule
  belongs_to :maintenance_reminder_lead, optional: true
  belongs_to :additional_cost

  enum :notification_kind, { advance: 0, due: 1 }
  enum :trigger_condition, { date: 0, mileage: 1 }
  enum :status, { queued: 0, sent: 1, skipped: 2, failed: 3 }

  validates :notification_kind, :trigger_condition, :status, presence: true
  validate :lead_matches_notification_kind

  private
    def lead_matches_notification_kind
      if advance? && maintenance_reminder_lead.blank?
        errors.add(:maintenance_reminder_lead, "is required for an advance notification")
      elsif due? && maintenance_reminder_lead.present?
        errors.add(:maintenance_reminder_lead, "must be blank for a due notification")
      end
    end
end
