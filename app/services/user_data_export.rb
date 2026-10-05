class UserDataExport
  FORMAT = "spotreba"
  VERSION = 1
  FIELDS = {
    vehicles: { id: :id, name: :string, fuel_type: :fuel_type },
    refuelings: { id: :id, refueled_on: :date, distance_km: :positive_decimal, amount: :positive_decimal, cost: :positive_decimal },
    additional_costs: { id: :id, occurred_on: :date, kind: :kind, cost: :positive_decimal },
    maintenance_reminder_rules: { id: :id, kind: :kind, interval_days: :optional_positive_integer, interval_km: :optional_positive_decimal, active: :boolean },
    maintenance_reminder_leads: { id: :id, days_before: :optional_positive_integer, kilometres_before: :optional_positive_decimal },
    maintenance_notifications: {
      id: :id, maintenance_reminder_rule_id: :id, maintenance_reminder_lead_id: :optional_id,
      additional_cost_id: :id, notification_kind: :notification_kind, trigger_condition: :trigger_condition,
      status: :status, days_remaining: :optional_integer, kilometres_remaining: :optional_decimal, sent_at: :optional_timestamp
    }
  }.transform_values { |fields| fields.merge(created_at: :timestamp, updated_at: :timestamp).freeze }.freeze

  def initialize(user)
    @user = user
  end

  def call
    options = ApplicationRecord.connection.transaction_open? ? {} : { isolation: :repeatable_read }
    ApplicationRecord.transaction(**options) do
      {
        format: FORMAT, version: VERSION, exported_at: Time.current.utc.iso8601(6),
        account: { email_address: @user.email_address, created_at: @user.created_at.utc.iso8601(6), updated_at: @user.updated_at.utc.iso8601(6) },
        vehicles: @user.vehicles.order(:id).includes(
          :refuelings, :additional_costs,
          maintenance_reminder_rules: [ :maintenance_reminder_leads, :maintenance_notifications ]
        ).map { |vehicle| export_vehicle(vehicle) }
      }
    end
  end

  private
    def export_vehicle(vehicle)
      rules = vehicle.maintenance_reminder_rules.sort_by(&:id)
      attributes(vehicle, :vehicles).merge(
        refuelings: vehicle.refuelings.sort_by(&:id).map { |record| attributes(record, :refuelings) },
        additional_costs: vehicle.additional_costs.sort_by(&:id).map { |record| attributes(record, :additional_costs) },
        maintenance_reminder_rules: rules.map do |rule|
          attributes(rule, :maintenance_reminder_rules).merge(
            maintenance_reminder_leads: rule.maintenance_reminder_leads.sort_by(&:id).map { |lead| attributes(lead, :maintenance_reminder_leads) }
          )
        end,
        maintenance_notifications: rules.flat_map(&:maintenance_notifications).sort_by(&:id).map { |record| attributes(record, :maintenance_notifications) }
      )
    end

    def attributes(record, collection)
      FIELDS.fetch(collection).to_h do |field, _type|
        value = record.public_send(field)
        value = case value
        when BigDecimal then value.to_s("F")
        when Time, ActiveSupport::TimeWithZone then value.utc.iso8601(6)
        when Date then value.iso8601
        else value
        end
        [ field, value ]
      end
    end
end
