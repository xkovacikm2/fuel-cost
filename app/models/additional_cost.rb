class AdditionalCost < ApplicationRecord
  belongs_to :vehicle
  has_many :maintenance_notifications, dependent: :destroy

  delegate :user, to: :vehicle

  enum :kind, {
    oil_change: 0,
    fuel_filter_change: 1,
    technical_inspection: 2,
    emissions_inspection: 3,
    tire_change: 4,
    repair: 5,
    other: 6
  }

  KIND_LABELS = {
    "oil_change" => "Výmena oleja",
    "fuel_filter_change" => "Výmena palivového filtra",
    "technical_inspection" => "Technická kontrola",
    "emissions_inspection" => "Emisná kontrola",
    "tire_change" => "Prezutie pneumatík",
    "repair" => "Oprava",
    "other" => "Iné"
  }.freeze

  validates :occurred_on, presence: true
  validates :kind, presence: true
  validates :cost, presence: true, numericality: { greater_than: 0 }

  scope :ordered, -> { order(occurred_on: :desc, created_at: :desc) }
  scope :within_range, ->(from_date, to_date) { where(occurred_on: from_date..to_date) }

  def kind_label
    KIND_LABELS[kind] || kind.to_s
  end

  def self.kind_options
    kinds.keys.map { |kind| [ KIND_LABELS[kind], kind ] }
  end

  class << self
    def for_user(user)
      joins(:vehicle).where(vehicles: { user_id: user.id })
    end
  end
end
