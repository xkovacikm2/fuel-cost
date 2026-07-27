# frozen_string_literal: true

# Usage:
#   bin/rails runner script/generate_random_refuelings.rb
#
# Optional environment variables:
#   USER_ID=1 VEHICLE_ID=1 START_DATE=2024-01-01 END_DATE=2026-07-27 COUNT=80 REPLACE=false \
#     bin/rails runner script/generate_random_refuelings.rb

require "date"

user_id = ENV.fetch("USER_ID", "1").to_i
vehicle_id = ENV.fetch("VEHICLE_ID", "1").to_i
start_date = Date.parse(ENV.fetch("START_DATE", "2024-01-01"))
end_date = Date.parse(ENV.fetch("END_DATE", Date.current.iso8601))
target_count = ENV.fetch("COUNT", "120").to_i
replace_existing = ENV.fetch("REPLACE", "false").downcase == "true"

if end_date < start_date
  abort "END_DATE must be on or after START_DATE"
end

user = User.find_by(id: user_id)
abort "User ##{user_id} not found" unless user

vehicle = Vehicle.find_by(id: vehicle_id)
abort "Vehicle ##{vehicle_id} not found" unless vehicle

if vehicle.user_id != user.id
  abort "Vehicle ##{vehicle_id} does not belong to user ##{user_id}"
end

if replace_existing
  deleted = Refueling.where(vehicle_id: vehicle.id, refueled_on: start_date..end_date).delete_all
  puts "Deleted #{deleted} existing refuelings in range"
end

existing_dates = Refueling.where(vehicle_id: vehicle.id, refueled_on: start_date..end_date).pluck(:refueled_on).to_set

max_days = (end_date - start_date).to_i + 1
if target_count > max_days
  abort "COUNT is too high for selected date range (max #{max_days})"
end

rng = Random.new
fuel_amount_range = vehicle.electricity? ? (12.0..55.0) : (22.0..68.0)
distance_range = vehicle.electricity? ? (110.0..430.0) : (280.0..850.0)
unit_price_range = vehicle.electricity? ? (0.12..0.34) : (1.35..2.15)

chosen_dates = []
attempts = 0

while chosen_dates.size < target_count && attempts < (target_count * 40)
  attempts += 1
  date = start_date + rng.rand(max_days)
  next if existing_dates.include?(date)
  next if chosen_dates.include?(date)

  chosen_dates << date
end

if chosen_dates.empty?
  abort "No dates available to create records in the selected range"
end

chosen_dates.sort.each do |date|
  distance_km = rng.rand(distance_range).round(2)
  amount = rng.rand(fuel_amount_range).round(2)

  # Add slight realism by varying price with occasional spikes.
  unit_price = rng.rand(unit_price_range)
  unit_price *= (1.0 + rng.rand(0.0..0.08)) if rng.rand < 0.15
  cost = (amount * unit_price).round(2)

  Refueling.create!(
    vehicle: vehicle,
    refueled_on: date,
    distance_km: distance_km,
    amount: amount,
    cost: cost
  )
end

puts "Created #{chosen_dates.size} refuelings for user ##{user.id}, vehicle ##{vehicle.id}"
puts "Range: #{start_date}..#{end_date}"
puts "Vehicle fuel type: #{vehicle.fuel_type} (unit #{vehicle.unit})"
