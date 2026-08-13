class Vehicle < ApplicationRecord
  belongs_to :user
  has_many :refuelings, dependent: :destroy
  has_many :additional_costs, dependent: :destroy

  enum :fuel_type, { petrol: 0, diesel: 1, electricity: 2 }

  FUEL_TYPE_LABELS = {
    "petrol" => "Benzín",
    "diesel" => "Nafta",
    "electricity" => "Elektrina"
  }.freeze

  validates :name, presence: true
  validates :fuel_type, presence: true

  def self.fuel_type_options
    fuel_types.keys.map { |type| [ FUEL_TYPE_LABELS[type], type ] }
  end

  def fuel_type_label
    FUEL_TYPE_LABELS[fuel_type] || fuel_type.to_s
  end

  def unit
    electricity? ? "kWh" : "L"
  end
end
