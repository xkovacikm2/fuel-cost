class MaintenanceReminderRule < ApplicationRecord
  belongs_to :vehicle
  has_many :maintenance_reminder_leads, dependent: :destroy
  has_many :maintenance_notifications, dependent: :destroy

  accepts_nested_attributes_for :maintenance_reminder_leads,
    allow_destroy: true,
    reject_if: ->(attributes) { attributes["id"].blank? && attributes["days_before"].blank? && attributes["kilometres_before"].blank? }

  delegate :user, to: :vehicle

  enum :kind, AdditionalCost.kinds

  validates :kind, presence: true, uniqueness: { scope: :vehicle_id }
  validates :interval_days, numericality: { only_integer: true, greater_than: 0 }, allow_nil: true
  validates :interval_km, numericality: { greater_than: 0 }, allow_nil: true
  validate :has_an_interval

  scope :active, -> { where(active: true) }

  def kind_label
    AdditionalCost::KIND_LABELS.fetch(kind)
  end

  def current_cycle(today: Date.current)
    additional_cost = latest_matching_cost(today: today)
    return unless additional_cost

    elapsed_days = (today - additional_cost.occurred_on).to_i
    travelled_km = vehicle.refuelings
      .where(refueled_on: (additional_cost.occurred_on + 1)..today)
      .sum(:distance_km)

    Cycle.new(
      additional_cost: additional_cost,
      days_remaining: interval_days && interval_days - elapsed_days,
      kilometres_remaining: interval_km && interval_km - travelled_km
    )
  end

  def latest_matching_cost(today: Date.current)
    vehicle.additional_costs
      .where(kind: kind, occurred_on: ..today)
      .order(occurred_on: :desc, created_at: :desc)
      .first
  end

  def self.for_user(user)
    joins(:vehicle).where(vehicles: { user_id: user.id })
  end

  Cycle = Struct.new(:additional_cost, :days_remaining, :kilometres_remaining, keyword_init: true)

  private
    def has_an_interval
      return if interval_days.present? || interval_km.present?

      errors.add(:base, "must have a day or kilometre interval")
    end
end
