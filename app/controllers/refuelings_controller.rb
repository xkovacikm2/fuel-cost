class RefuelingsController < ApplicationController
  before_action :set_refueling, only: %i[ edit update destroy ]
  before_action :set_vehicles, only: %i[ new create edit update ]
  before_action :ensure_vehicles_exist, only: %i[ new create ]

  # GET /refuelings/new
  def new
    @refueling = Refueling.new(refueled_on: Date.current)
    @refueling.vehicle = last_used_vehicle || @vehicles.first
  end

  # GET /refuelings/1/edit
  def edit
  end

  # POST /refuelings or /refuelings.json
  def create
    @refueling = Refueling.new(refueling_params)
    unless Current.user.vehicles.exists?(id: @refueling.vehicle_id)
      redirect_to root_path, alert: "Nemôžete vytvárať tankovania pre iného používateľa."
      return
    end

    respond_to do |format|
      if @refueling.save
        format.html { redirect_to root_path, notice: "Tankovanie bolo úspešne vytvorené." }
        format.json { render json: @refueling, status: :created }
      else
        format.html { render :new, status: :unprocessable_content }
        format.json { render json: @refueling.errors, status: :unprocessable_content }
      end
    end
  end

  # PATCH/PUT /refuelings/1 or /refuelings/1.json
  def update
    unless Current.user.vehicles.exists?(id: refueling_params[:vehicle_id])
      redirect_to root_path, alert: "Nemôžete upravovať tankovania iného používateľa."
      return
    end

    respond_to do |format|
      if @refueling.update(refueling_params)
        format.html { redirect_to root_path, notice: "Tankovanie bolo úspešne upravené.", status: :see_other }
        format.json { render json: @refueling, status: :ok }
      else
        format.html { render :edit, status: :unprocessable_content }
        format.json { render json: @refueling.errors, status: :unprocessable_content }
      end
    end
  end

  # DELETE /refuelings/1 or /refuelings/1.json
  def destroy
    @refueling.destroy!

    respond_to do |format|
      format.html { redirect_to root_path, notice: "Tankovanie bolo úspešne odstránené.", status: :see_other }
      format.json { head :no_content }
    end
  end

  private
    # Use callbacks to share common setup or constraints between actions.
    def set_refueling
      @refueling = Refueling.for_user(Current.user).find(params.expect(:id))
    end

    # Only allow a list of trusted parameters through.
    def refueling_params
      params.expect(refueling: [ :vehicle_id, :refueled_on, :distance_km, :amount, :cost ])
    end

    def set_vehicles
      @vehicles = Current.user.vehicles.order(:name)
      @vehicle_units = @vehicles.index_with(&:unit)
    end

    def ensure_vehicles_exist
      return if Current.user.vehicles.exists?

      redirect_to new_vehicle_path, alert: "Najprv vytvorte vozidlo."
    end

    def last_used_vehicle
      Refueling.most_recent_for(Current.user)&.vehicle
    end
end
