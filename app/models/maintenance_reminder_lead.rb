class MaintenanceReminderLead < ApplicationRecord
  belongs_to :maintenance_reminder_rule
  has_many :maintenance_notifications, dependent: :destroy

  validates :days_before, numericality: { only_integer: true, greater_than: 0 }, allow_nil: true
  validates :kilometres_before, numericality: { greater_than: 0 }, allow_nil: true
  validate :has_exactly_one_dimension
  validate :matches_rule_interval

  scope :ordered, -> { order(days_before: :desc, kilometres_before: :desc, id: :asc) }

  private
    def has_exactly_one_dimension
      return if days_before.present? ^ kilometres_before.present?

      errors.add(:base, "must have either days or kilometres before")
    end

    def matches_rule_interval
      if days_before.present? && maintenance_reminder_rule&.interval_days.blank?
        errors.add(:days_before, "requires a day interval")
      end

      if kilometres_before.present? && maintenance_reminder_rule&.interval_km.blank?
        errors.add(:kilometres_before, "requires a kilometre interval")
      end
    end
end
