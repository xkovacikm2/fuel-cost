require "test_helper"

class MaintenanceMailerTest < ActionMailer::TestCase
  test "renders a Slovak advance reminder" do
    notification = MaintenanceNotification.new(
      maintenance_reminder_rule: maintenance_reminder_rules(:oil_change),
      maintenance_reminder_lead: maintenance_reminder_leads(:thirty_days),
      additional_cost: additional_costs(:one),
      notification_kind: :advance,
      trigger_condition: :date,
      status: :queued,
      days_remaining: 30,
      kilometres_remaining: 1_000
    )

    mail = MaintenanceMailer.maintenance_due(notification)

    assert_equal [ users(:one).email_address ], mail.to
    assert_equal "Blíži sa údržba vozidla: Výmena oleja", mail.subject
    assert_includes mail.text_part.body.decoded, "Naplánujte si prosím vhodný termín."
    assert_includes mail.html_part.body.decoded, "Do termínu zostáva"
  end
end
