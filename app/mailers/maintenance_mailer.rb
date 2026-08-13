class MaintenanceMailer < ApplicationMailer
  def maintenance_due(notification)
    @notification = notification
    @rule = notification.maintenance_reminder_rule
    @vehicle = @rule.vehicle
    @additional_cost = notification.additional_cost

    mail subject: subject, to: @vehicle.user.email_address
  end

  private
    def subject
      if @notification.due?
        "Údržba vozidla je potrebná: #{@rule.kind_label}"
      else
        "Blíži sa údržba vozidla: #{@rule.kind_label}"
      end
    end
end
