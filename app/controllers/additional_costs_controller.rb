class AdditionalCostsController < ApplicationController
  before_action :set_additional_cost, only: %i[ edit update destroy ]
  before_action :set_vehicles, only: %i[ new create edit update ]
  before_action :ensure_vehicles_exist, only: %i[ new create ]

  def new
    @additional_cost = AdditionalCost.new(occurred_on: Date.current)
    @additional_cost.vehicle = @vehicles.first
  end

  def edit
  end

  def create
    @additional_cost = AdditionalCost.new(additional_cost_params)
    unless Current.user.vehicles.exists?(id: @additional_cost.vehicle_id)
      redirect_to root_path, alert: "Nemôžete vytvárať náklady pre iného používateľa."
      return
    end

    if @additional_cost.save
      redirect_to root_path, notice: "Dodatočný náklad bol úspešne vytvorený."
    else
      render :new, status: :unprocessable_content
    end
  end

  def update
    unless Current.user.vehicles.exists?(id: additional_cost_params[:vehicle_id])
      redirect_to root_path, alert: "Nemôžete upravovať náklady iného používateľa."
      return
    end

    if @additional_cost.update(additional_cost_params)
      redirect_to root_path, notice: "Dodatočný náklad bol úspešne upravený.", status: :see_other
    else
      render :edit, status: :unprocessable_content
    end
  end

  def destroy
    @additional_cost.destroy!
    redirect_to root_path, notice: "Dodatočný náklad bol úspešne odstránený.", status: :see_other
  end

  private
    def set_additional_cost
      @additional_cost = AdditionalCost.for_user(Current.user).find(params.expect(:id))
    end

    def additional_cost_params
      params.expect(additional_cost: [ :vehicle_id, :occurred_on, :kind, :cost ])
    end

    def set_vehicles
      @vehicles = Current.user.vehicles.order(:name)
    end

    def ensure_vehicles_exist
      return if Current.user.vehicles.exists?

      redirect_to new_vehicle_path, alert: "Najprv vytvorte vozidlo."
    end
end
