class Refueling < ApplicationRecord
  belongs_to :vehicle

  delegate :user, to: :vehicle

  validates :refueled_on, presence: true
  validates :distance_km, :amount, :cost, presence: true, numericality: { greater_than: 0 }

  scope :ordered, -> { order(refueled_on: :desc, created_at: :desc) }
  scope :within_range, ->(from_date, to_date) { where(refueled_on: from_date..to_date) }

  def consumption_per_100km
    ((amount / distance_km) * 100).round(2)
  end

  def cost_per_100km
    ((cost / distance_km) * 100).round(2)
  end

  class << self
    def for_user(user)
      joins(:vehicle).where(vehicles: { user_id: user.id })
    end

    def most_recent_for(user)
      for_user(user).ordered.first
    end

    def average_days_between(user, vehicle: nil)
      records = scoped_for(user, vehicle).order(:refueled_on).pluck(:refueled_on)
      return nil if records.length < 2

      diffs = records.each_cons(2).map { |previous, current| (current - previous).to_i }
      (diffs.sum.to_f / diffs.size).round(1)
    end

    def days_since_last(user, vehicle: nil, today: Date.current)
      last_refuel_date = scoped_for(user, vehicle).maximum(:refueled_on)
      return nil unless last_refuel_date

      (today - last_refuel_date).to_i
    end

    private
      def scoped_for(user, vehicle)
        scope = for_user(user)
        vehicle ? scope.where(vehicle_id: vehicle.id) : scope
      end
  end
end
