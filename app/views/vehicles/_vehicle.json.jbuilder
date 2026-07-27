json.extract! vehicle, :id, :user_id, :name, :fuel_type, :created_at, :updated_at
json.url vehicle_url(vehicle, format: :json)
