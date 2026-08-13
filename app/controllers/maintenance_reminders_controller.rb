class MaintenanceRemindersController < ApplicationController
  TIME_UNIT_DAYS = {
    "days" => 1,
    "weeks" => 7,
    "months" => 30,
    "years" => 365
  }.freeze

  before_action :set_maintenance_reminder, only: %i[ edit update destroy ]
  before_action :set_vehicles, only: %i[ new create edit update ]

  def index
    @maintenance_reminders = MaintenanceReminderRule.for_user(Current.user)
      .includes(:vehicle, :maintenance_reminder_leads)
      .order("vehicles.name", :kind)
  end

  def new
    @maintenance_reminder = MaintenanceReminderRule.new(active: true)
    @maintenance_reminder.vehicle = @vehicles.first
    @maintenance_reminder.maintenance_reminder_leads.build
  end

  def edit
    @maintenance_reminder.maintenance_reminder_leads.build if @maintenance_reminder.maintenance_reminder_leads.empty?
  end

  def create
    @maintenance_reminder = MaintenanceReminderRule.new(maintenance_reminder_params)
    unless Current.user.vehicles.exists?(id: @maintenance_reminder.vehicle_id)
      redirect_to maintenance_reminders_path, alert: "Nemôžete vytvárať pripomienky pre iného používateľa."
      return
    end

    if @maintenance_reminder.save
      redirect_to maintenance_reminders_path, notice: "Pripomienka údržby bola úspešne vytvorená."
    else
      render :new, status: :unprocessable_content
    end
  end

  def update
    unless Current.user.vehicles.exists?(id: maintenance_reminder_params[:vehicle_id])
      redirect_to maintenance_reminders_path, alert: "Nemôžete upravovať pripomienky iného používateľa."
      return
    end

    if @maintenance_reminder.update(maintenance_reminder_params)
      redirect_to maintenance_reminders_path, notice: "Pripomienka údržby bola úspešne upravená.", status: :see_other
    else
      render :edit, status: :unprocessable_content
    end
  end

  def destroy
    @maintenance_reminder.destroy!
    redirect_to maintenance_reminders_path, notice: "Pripomienka údržby bola úspešne odstránená.", status: :see_other
  end

  private
    def set_maintenance_reminder
      @maintenance_reminder = MaintenanceReminderRule.for_user(Current.user).find(params.expect(:id))
    end

    def set_vehicles
      @vehicles = Current.user.vehicles.order(:name)
    end

    def maintenance_reminder_params
      attributes = params.require(:maintenance_reminder_rule).permit(
        :vehicle_id,
        :kind,
        :interval_time_value,
        :interval_time_unit,
        :interval_km,
        :active,
        maintenance_reminder_leads_attributes: [ :id, :time_value, :time_unit, :kilometres_before, :_destroy ]
      ).to_h

      attributes["interval_days"] = days_from(attributes.delete("interval_time_value"), attributes.delete("interval_time_unit"))
      attributes["maintenance_reminder_leads_attributes"] = normalized_lead_attributes(attributes["maintenance_reminder_leads_attributes"])
      attributes
    end

    def normalized_lead_attributes(lead_attributes)
      return {} if lead_attributes.blank?

      lead_attributes.each_with_object({}) do |(index, attributes), normalized|
        attributes = attributes.stringify_keys
        time_value = attributes.delete("time_value")
        time_unit = attributes.delete("time_unit")
        attributes["days_before"] = days_from(time_value, time_unit)

        if attributes["days_before"].present? && attributes["kilometres_before"].present? && attributes["id"].blank?
          normalized["#{index}_days"] = attributes.except("kilometres_before")
          normalized["#{index}_kilometres"] = attributes.except("days_before")
        elsif attributes["days_before"].present? && attributes["kilometres_before"].present?
          split_existing_lead_attributes(index, attributes, normalized)
        else
          normalized[index] = attributes
        end
      end
    end

    def split_existing_lead_attributes(index, attributes, normalized)
      existing_lead = @maintenance_reminder&.maintenance_reminder_leads&.find_by(id: attributes["id"])

      if existing_lead&.days_before.present?
        normalized[index] = attributes.except("kilometres_before")
        normalized["#{index}_kilometres"] = attributes.except("id", "days_before")
      else
        normalized[index] = attributes.except("days_before")
        normalized["#{index}_days"] = attributes.except("id", "kilometres_before")
      end
    end

    def days_from(value, unit)
      return if value.blank?

      value.to_i * TIME_UNIT_DAYS.fetch(unit, 1)
    end
end
