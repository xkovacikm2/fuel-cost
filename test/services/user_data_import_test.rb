require "test_helper"

class UserDataImportTest < ActiveSupport::TestCase
  include ActiveJob::TestHelper

  setup do
    @source = vehicles(:one).user
    @target = User.create!(email_address: "import-target@example.com", password: "password")
    @payload = JSON.parse(UserDataExport.new(@source).call.to_json)
  end

  test "round trips the complete graph and skips repeated records" do
    rule = maintenance_reminder_rules(:oil_change)
    notification = rule.maintenance_notifications.create!(
      additional_cost: additional_costs(:one), maintenance_reminder_lead: maintenance_reminder_leads(:thirty_days),
      notification_kind: :advance, trigger_condition: :date, status: :queued, days_remaining: 25
    )
    @payload = JSON.parse(UserDataExport.new(@source).call.to_json)
    assert_no_enqueued_jobs do
      result = import
      assert_equal @source.vehicles.count, result.imported[:vehicles]
      assert_equal @source.refuelings.count, result.imported[:refuelings]
      assert_equal @source.additional_costs.count, result.imported[:additional_costs]
      assert_equal 1, result.normalized_notifications
    end
    vehicle = @target.vehicles.find_by!(name: rule.vehicle.name)
    copied_rule = vehicle.maintenance_reminder_rules.find_by!(kind: rule.kind)
    copied_notification = copied_rule.maintenance_notifications.first!
    assert_predicate copied_notification, :skipped?
    assert_equal vehicle, copied_notification.additional_cost.vehicle
    assert_equal copied_rule, copied_notification.maintenance_reminder_lead.maintenance_reminder_rule
    assert_equal notification.created_at, copied_notification.created_at
    assert_equal @source.email_address, @source.reload.email_address
    assert_equal "import-target@example.com", @target.reload.email_address
    assert_equal rule.vehicle.created_at, vehicle.created_at
    assert_equal 0, import.imported.values.sum
  end

  test "merges missing children into an existing vehicle" do
    original = @payload["vehicles"].first
    vehicle = @target.vehicles.create!(name: original["name"], fuel_type: original["fuel_type"])
    result = import
    assert_equal 1, result.skipped[:vehicles]
    assert_equal original["refuelings"].size, vehicle.refuelings.count
  end

  test "creates a same-name vehicle with a different fuel type" do
    original = @payload["vehicles"].first
    @target.vehicles.create!(name: original["name"], fuel_type: original["fuel_type"] == "diesel" ? :petrol : :diesel)
    assert_equal @payload["vehicles"].size, import.imported[:vehicles]
  end

  test "skips ambiguous cars with their children" do
    original = @payload["vehicles"].first
    2.times { @target.vehicles.create!(name: original["name"], fuel_type: original["fuel_type"]) }
    result = import
    assert_equal 1, result.reasons[:ambiguous_vehicle]
    assert_empty @target.refuelings
  end

  test "rejects an invalid late record without any writes" do
    @payload["vehicles"].last["refuelings"].last["amount"] = "0"
    assert_no_difference("Vehicle.count") { assert_raises(UserDataImport::InvalidFile) { import } }
    assert_empty @target.refuelings
  end

  test "rejects unsupported formats and unsafe extra fields" do
    @payload["version"] = 2
    assert_raises(UserDataImport::InvalidFile) { import }
    @payload["version"] = 1
    @payload["vehicles"].first["user_id"] = @source.id
    assert_raises(UserDataImport::InvalidFile) { import }
  end

  test "rejects malformed JSON and excess precision" do
    assert_raises(UserDataImport::InvalidFile) { UserDataImport.new(@target, "{").call }
    @payload["vehicles"].first["refuelings"].first["cost"] = "12.345"
    assert_raises(UserDataImport::InvalidFile) { import }
  end

  test "normalizes decimals and skips duplicates within the file without losing distinct same-date records" do
    rows = @payload["vehicles"].first["refuelings"]
    duplicate = rows.first.deep_dup
    duplicate["id"] = 9_000_001
    duplicate["cost"] = BigDecimal(duplicate["cost"]).to_s("F")
    distinct = duplicate.merge("id" => 9_000_002, "cost" => "123.45")
    rows.concat([ duplicate, distinct ])
    result = import
    assert_equal @source.refuelings.count + 1, result.imported[:refuelings]
    assert_equal 1, result.skipped[:refuelings]
  end

  test "skips conflicting reminder settings but imports other vehicle records" do
    vehicle_data = @payload["vehicles"].first
    rule_data = vehicle_data["maintenance_reminder_rules"].first
    vehicle = @target.vehicles.create!(name: vehicle_data["name"], fuel_type: vehicle_data["fuel_type"])
    rule = vehicle.maintenance_reminder_rules.create!(kind: rule_data["kind"], interval_days: 999)
    result = import
    assert_equal 1, result.reasons[:rule_configuration]
    assert_equal 999, rule.reload.interval_days
    assert_empty rule.maintenance_reminder_leads
    assert_equal @source.refuelings.count, @target.refuelings.count
    assert_equal @source.additional_costs.count, @target.additional_costs.count
  end

  test "merges missing leads into matching rules without updating existing values" do
    import
    rule = @target.maintenance_reminder_rules.find_by!(kind: :oil_change)
    lead = rule.maintenance_reminder_leads.first!
    lead.destroy!
    original_updated_at = rule.updated_at
    result = import
    assert_equal 1, result.imported[:maintenance_reminder_leads]
    assert_equal original_updated_at, rule.reload.updated_at
  end

  test "preserves sent history and skips notification duplicates with changed statuses" do
    rule = maintenance_reminder_rules(:oil_change)
    rule.maintenance_notifications.create!(additional_cost: additional_costs(:one), notification_kind: :due,
      trigger_condition: :date, status: :sent, sent_at: Time.current, days_remaining: -1)
    @payload = JSON.parse(UserDataExport.new(@source).call.to_json)
    import
    copied = @target.maintenance_reminder_rules.find_by!(kind: :oil_change).maintenance_notifications.first!
    assert_predicate copied, :sent?
    original_sent_at = copied.sent_at
    @payload["vehicles"].first["maintenance_notifications"].first["status"] = "queued"
    result = import
    assert_equal 1, result.skipped[:maintenance_notifications]
    assert_equal 0, result.normalized_notifications
    assert_predicate copied.reload, :sent?
    assert_equal original_sent_at, copied.sent_at
  end

  test "rejects missing and cross-vehicle notification references" do
    rule = maintenance_reminder_rules(:oil_change)
    rule.maintenance_notifications.create!(additional_cost: additional_costs(:one), notification_kind: :due,
      trigger_condition: :date, status: :sent)
    other_vehicle = @source.vehicles.create!(name: "Other source vehicle", fuel_type: :diesel)
    other_cost = other_vehicle.additional_costs.create!(kind: :oil_change, occurred_on: Date.current, cost: 20)
    @payload = JSON.parse(UserDataExport.new(@source).call.to_json)
    notification = @payload["vehicles"].first["maintenance_notifications"].first
    notification["additional_cost_id"] = other_cost.id
    assert_no_difference("Vehicle.count") { assert_raises(UserDataImport::InvalidFile) { import } }
    notification["additional_cost_id"] = 9_999_999
    assert_raises(UserDataImport::InvalidFile) { import }
  end

  test "rejects invalid types dates enums booleans and duplicate source IDs" do
    valid = @payload.deep_dup
    changes = [
      -> { @payload["version"] = 1.0 },
      -> { @payload["vehicles"] = {} },
      -> { @payload["vehicles"].first["fuel_type"] = "unknown" },
      -> { @payload["vehicles"].first["refuelings"].first["refueled_on"] = "2026-02-30" },
      -> { @payload["vehicles"].first["refuelings"].first["amount"] = "NaN" },
      -> { @payload["vehicles"].first["refuelings"].first["cost"] = "100000000.00" },
      -> { @payload["vehicles"].first["created_at"] = "2026-02-30T10:00:00Z" },
      -> { @payload["vehicles"].first["maintenance_reminder_rules"].first["active"] = "false" },
      -> { @payload["vehicles"].first["maintenance_reminder_rules"].first["interval_days"] = 1.5 },
      -> { @payload["vehicles"].first["refuelings"] << @payload["vehicles"].first["refuelings"].first.deep_dup }
    ]
    changes.each do |change|
      @payload = valid.deep_dup
      change.call
      assert_no_difference("Vehicle.count") { assert_raises(UserDataImport::InvalidFile) { import } }
    end
  end

  test "rejects upload record and nesting limits" do
    assert_raises(UserDataImport::InvalidFile) { UserDataImport.new(@target, " " * (UserDataImport::MAX_BYTES + 1)).call }
    assert_raises(UserDataImport::InvalidFile) { UserDataImport.new(@target, "[" * 33 + "0" + "]" * 33).call }
    stub_const(UserDataImport, :MAX_RECORDS, 1) do
      assert_raises(UserDataImport::InvalidFile) { import }
    end
  end

  test "rolls back all writes if persistence fails after valid records were inserted" do
    importer_class = Class.new(UserDataImport) do
      private
        def import_rule(_node, _vehicle)
          raise ActiveRecord::RecordInvalid.new(MaintenanceReminderRule.new)
        end
    end
    assert_no_difference([ "Vehicle.count", "Refueling.count", "AdditionalCost.count" ]) do
      assert_raises(ActiveRecord::RecordInvalid) do
        importer_class.new(@target, @payload.to_json).call
      end
    end
  end

  test "recovers from a database uniqueness race and preserves the competing notification" do
    rule = maintenance_reminder_rules(:oil_change)
    rule.maintenance_notifications.create!(additional_cost: additional_costs(:one), notification_kind: :due,
      trigger_condition: :date, status: :queued)
    @payload = JSON.parse(UserDataExport.new(@source).call.to_json)
    importer_class = Class.new(UserDataImport) do
      private
        def persist(node, scope, existing, **overrides)
          if node.collection == :maintenance_notifications && existing.nil?
            scope.create!(writable_attributes(node.attributes).merge(overrides).merge(status: :sent))
          end
          super
        end
    end
    result = importer_class.new(@target, @payload.to_json).call
    assert_equal 1, result.skipped[:maintenance_notifications]
    assert_equal 0, result.normalized_notifications
    notification = @target.maintenance_reminder_rules.find_by!(kind: :oil_change).maintenance_notifications.first!
    assert_predicate notification, :sent?
    assert_equal @source.additional_costs.count, @target.additional_costs.count
  end

  private
    def import
      UserDataImport.new(@target, @payload.to_json).call
    end
end
