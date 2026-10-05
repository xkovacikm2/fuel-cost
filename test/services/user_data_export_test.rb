require "test_helper"

class UserDataExportTest < ActiveSupport::TestCase
  test "exports only owned domain data using explicit fields" do
    user = vehicles(:one).user
    payload = JSON.parse(UserDataExport.new(user).call.to_json)

    assert_equal "spotreba", payload["format"]
    assert_equal 1, payload["version"]
    assert_equal user.vehicles.order(:id).pluck(:id), payload["vehicles"].map { |vehicle| vehicle["id"] }
    assert_not_includes payload.to_json, "password_digest"
    assert_not_includes payload.to_json, "sessions"
    assert_not_includes payload.to_json, "user_id"
    vehicle = payload["vehicles"].find { |record| record["id"] == vehicles(:one).id }
    assert_equal refuelings(:one).distance_km.to_s("F"), vehicle["refuelings"].find { |record| record["id"] == refuelings(:one).id }["distance_km"]
    assert_equal vehicles(:one).created_at.utc.iso8601(6), vehicle["created_at"]
    assert_equal maintenance_reminder_rules(:oil_change).id, vehicle["maintenance_reminder_rules"].first["id"]
    assert vehicle["maintenance_reminder_rules"].first["maintenance_reminder_leads"].any?
  end

  test "exports an empty account" do
    user = User.create!(email_address: "empty-export@example.com", password: "password")
    assert_empty UserDataExport.new(user).call[:vehicles]
  end
end
