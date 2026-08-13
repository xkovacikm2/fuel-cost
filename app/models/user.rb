class User < ApplicationRecord
  has_secure_password
  has_many :sessions, dependent: :destroy
  has_many :vehicles, dependent: :destroy
  has_many :refuelings, through: :vehicles
  has_many :additional_costs, through: :vehicles
  has_many :maintenance_reminder_rules, through: :vehicles

  normalizes :email_address, with: ->(e) { e.strip.downcase }

  validates :email_address, presence: true
end
