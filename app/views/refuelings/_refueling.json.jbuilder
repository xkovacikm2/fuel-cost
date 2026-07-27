json.extract! refueling, :id, :vehicle_id, :refueled_on, :distance_km, :amount, :cost, :created_at, :updated_at
json.url refueling_url(refueling, format: :json)
