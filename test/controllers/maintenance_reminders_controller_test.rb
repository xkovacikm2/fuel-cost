require "test_helper"

class MaintenanceRemindersControllerTest < ActionDispatch::IntegrationTest
  setup do
    @rule = maintenance_reminder_rules(:oil_change)
    sign_in_as users(:one)
  end

  test "lists the signed-in user's reminders" do
    get maintenance_reminders_url

    assert_response :success
    assert_select "h2", "Pripomienky údržby"
  end

  test "creates a reminder with an advance lead" do
    assert_difference("MaintenanceReminderRule.count") do
      assert_difference("MaintenanceReminderLead.count") do
        post maintenance_reminders_url, params: { maintenance_reminder_rule: {
          vehicle_id: vehicles(:one).id,
          kind: "repair",
          interval_time_value: 180,
          interval_time_unit: "days",
          interval_km: "",
          active: "1",
          maintenance_reminder_leads_attributes: {
            "0" => { time_value: 14, time_unit: "days", kilometres_before: "" }
          }
        } }
      end
    end

    assert_redirected_to maintenance_reminders_url
  end

  test "converts time units to days and splits a combined advance lead" do
    assert_difference("MaintenanceReminderRule.count") do
      assert_difference("MaintenanceReminderLead.count", 2) do
        post maintenance_reminders_url, params: { maintenance_reminder_rule: {
          vehicle_id: vehicles(:one).id,
          kind: "repair",
          interval_time_value: 2,
          interval_time_unit: "months",
          interval_km: 15_000,
          active: "1",
          maintenance_reminder_leads_attributes: {
            "0" => { time_value: 3, time_unit: "weeks", kilometres_before: 1_000 }
          }
        } }
      end
    end

    reminder = MaintenanceReminderRule.find_by!(vehicle: vehicles(:one), kind: :repair)
    assert_equal 60, reminder.interval_days
    assert_equal 15_000, reminder.interval_km
    assert_equal [ 21 ], reminder.maintenance_reminder_leads.pluck(:days_before).compact
    assert_equal [ 1_000 ], reminder.maintenance_reminder_leads.pluck(:kilometres_before).compact
  end

  test "rejects a reminder for another user's vehicle" do
    assert_no_difference("MaintenanceReminderRule.count") do
      post maintenance_reminders_url, params: { maintenance_reminder_rule: {
        vehicle_id: vehicles(:two).id,
        kind: "repair",
        interval_days: 180,
        active: "1"
      } }
    end

    assert_redirected_to maintenance_reminders_url
  end

  test "updates and removes an advance lead" do
    lead = maintenance_reminder_leads(:thirty_days)

    patch maintenance_reminder_url(@rule), params: { maintenance_reminder_rule: {
      vehicle_id: @rule.vehicle_id,
      kind: @rule.kind,
      interval_days: @rule.interval_days,
      interval_km: @rule.interval_km,
      active: "1",
      maintenance_reminder_leads_attributes: {
        "0" => { id: lead.id, _destroy: "1" }
      }
    } }

    assert_redirected_to maintenance_reminders_url
    assert_not MaintenanceReminderLead.exists?(lead.id)
  end

  test "does not expose another user's reminder" do
    other_rule = MaintenanceReminderRule.create!(vehicle: vehicles(:two), kind: :oil_change, interval_days: 365)

    get edit_maintenance_reminder_url(other_rule)

    assert_response :not_found
  end
end
