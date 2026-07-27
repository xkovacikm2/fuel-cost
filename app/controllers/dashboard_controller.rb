class DashboardController < ApplicationController
  TABLE_PER_PAGE = 10

  def show
    @vehicles = Current.user.vehicles.order(:name)
    @offset_days = [ params[:offset].to_i, 0 ].max
    @window_end = Date.current - @offset_days
    @window_start = @window_end - 364

    @scope = params[:scope] == "combined" ? "combined" : "vehicle"
    @selected_vehicle = selected_vehicle

    @chart_payload = chart_payload
    @average_days_between = Refueling.average_days_between(Current.user, vehicle: stats_vehicle)
    @days_since_last_refueling = Refueling.days_since_last(Current.user, vehicle: stats_vehicle)
    @average_consumption_per_100km = average_consumption_per_100km
    @total_fuel_refilled = total_fuel_refilled
    @total_cost_paid = total_cost_paid

    setup_refueling_table
  end

  private
    def selected_vehicle
      return @selected_vehicle if defined?(@selected_vehicle)

      @selected_vehicle = if params[:vehicle_id].present?
        Current.user.vehicles.find_by(id: params[:vehicle_id])
      else
        Refueling.most_recent_for(Current.user)&.vehicle || @vehicles.first
      end
    end

    def scoped_refuelings
      scope = Refueling.for_user(Current.user).within_range(@window_start, @window_end)
      return scope if @scope == "combined"
      return Refueling.none unless selected_vehicle

      scope.where(vehicle_id: selected_vehicle.id)
    end

    def stats_vehicle
      @scope == "combined" ? nil : selected_vehicle
    end

    def chart_payload
      rows = scoped_refuelings.includes(:vehicle).order(:refueled_on)
      labels = rows.pluck(:refueled_on).map(&:iso8601).uniq
      series = @scope == "combined" ? build_combined_series(rows, labels) : build_single_vehicle_series(rows, labels)

      {
        labels: labels,
        datasets: series,
        title: chart_title
      }
    end

    def build_single_vehicle_series(rows, labels)
      consumption_by_date = rows.index_by(&:refueled_on)

      [
        {
          label: "Spotreba na 100 km (#{selected_vehicle&.unit || "jednotka"})",
          yAxisID: "yConsumption",
          borderColor: "#1f5aff",
          backgroundColor: "rgba(31, 90, 255, 0.2)",
          data: labels.map { |label| consumption_by_date.fetch(Date.parse(label), nil)&.consumption_per_100km },
          tension: 0.2
        },
        {
          label: "Náklady na 100 km",
          yAxisID: "yCost",
          borderColor: "#ff5a1f",
          backgroundColor: "rgba(255, 90, 31, 0.2)",
          borderDash: [ 6, 4 ],
          data: labels.map { |label| consumption_by_date.fetch(Date.parse(label), nil)&.cost_per_100km },
          tension: 0.2
        }
      ]
    end

    def build_combined_series(rows, labels)
      grouped = rows.group_by(&:vehicle)

      grouped.flat_map.with_index do |(vehicle, vehicle_rows), idx|
        by_date = vehicle_rows.index_by(&:refueled_on)
        consumption_color = "hsl(#{(idx * 57) % 360} 80% 45%)"
        cost_color = "hsl(#{(idx * 57 + 24) % 360} 70% 40%)"

        [
          {
            label: "#{vehicle.name}: Spotreba (#{vehicle.unit})",
            yAxisID: "yConsumption",
            borderColor: consumption_color,
            backgroundColor: consumption_color,
            data: labels.map { |label| by_date.fetch(Date.parse(label), nil)&.consumption_per_100km },
            tension: 0.2
          },
          {
            label: "#{vehicle.name}: Náklady na 100 km",
            yAxisID: "yCost",
            borderColor: cost_color,
            backgroundColor: cost_color,
            borderDash: [ 6, 4 ],
            data: labels.map { |label| by_date.fetch(Date.parse(label), nil)&.cost_per_100km },
            tension: 0.2
          }
        ]
      end
    end

    def chart_title
      period = "#{@window_start.iso8601} - #{@window_end.iso8601}"
      @scope == "combined" ? "Vozidlá spolu (#{period})" : "#{selected_vehicle&.name || "Vozidlo"} (#{period})"
    end

    def setup_refueling_table
      @table_page = [ params.fetch(:table_page, 1).to_i, 1 ].max
      table_scope = Refueling.for_user(Current.user).includes(:vehicle).ordered
      @table_total_count = table_scope.count
      @table_total_pages = [ (@table_total_count.to_f / TABLE_PER_PAGE).ceil, 1 ].max
      @table_page = [ @table_page, @table_total_pages ].min

      offset = (@table_page - 1) * TABLE_PER_PAGE
      @table_refuelings = table_scope.limit(TABLE_PER_PAGE).offset(offset)
    end

    def average_consumption_per_100km
      scope = scoped_refuelings
      total_distance = scope.sum(:distance_km)
      return nil if total_distance.zero?

      ((scope.sum(:amount) / total_distance) * 100).round(1)
    end

    def total_fuel_refilled
      scoped_refuelings.sum(:amount).round(2)
    end

    def total_cost_paid
      scoped_refuelings.sum(:cost).round(2)
    end
end
