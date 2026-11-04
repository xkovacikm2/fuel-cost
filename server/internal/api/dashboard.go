package api

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"spotreba/server/internal/domain"

	"github.com/shopspring/decimal"
)

const dashboardPageSize = 10

type dashboardRefueling struct {
	refuelingRecord
	date     time.Time
	distance decimal.Decimal
	amount   decimal.Decimal
	cost     decimal.Decimal
}

type dashboardCost struct {
	ID          int64   `json:"id"`
	VehicleID   int64   `json:"vehicle_id"`
	VehicleName string  `json:"vehicle_name"`
	OccurredOn  string  `json:"occurred_on"`
	Kind        string  `json:"kind"`
	KindLabel   string  `json:"kind_label"`
	Comment     *string `json:"comment"`
	Cost        string  `json:"cost"`
}

type dashboardDataset struct {
	Label           string    `json:"label"`
	YAxisID         string    `json:"yAxisID"`
	BorderColor     string    `json:"borderColor"`
	BackgroundColor string    `json:"backgroundColor"`
	BorderDash      []int     `json:"borderDash,omitempty"`
	Data            []*string `json:"data"`
	Tension         float64   `json:"tension"`
}

type dashboardChart struct {
	Title           string             `json:"title"`
	Labels          []string           `json:"labels"`
	Datasets        []dashboardDataset `json:"datasets"`
	AdditionalCosts []dashboardCost    `json:"additional_costs"`
}

type dashboardStats struct {
	DaysSinceLastRefueling       *int    `json:"days_since_last_refueling"`
	AverageConsumptionPer100KM   *string `json:"average_consumption_per_100km"`
	TotalFuelRefilled            string  `json:"total_fuel_refilled"`
	TotalFuelCostPaid            string  `json:"total_fuel_cost_paid"`
	TotalAdditionalCostPaid      string  `json:"total_additional_cost_paid"`
	TotalCostPaid                string  `json:"total_cost_paid"`
	AverageDaysBetweenRefuelings *string `json:"average_days_between_refuelings"`
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicles, err := s.vehiclesForUser(r, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
		return
	}
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	if offset > 365000 {
		offset = 365000
	}
	localToday, err := time.ParseInLocation(time.DateOnly, time.Now().In(s.appTZ).Format(time.DateOnly), s.appTZ)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dátum sa nepodarilo vypočítať.")
		return
	}
	end := localToday.AddDate(0, 0, -offset)
	start := end.AddDate(0, 0, -364)
	windowStart, windowEnd := start.Format(time.DateOnly), end.Format(time.DateOnly)
	scope := "vehicle"
	if r.URL.Query().Get("scope") == "combined" {
		scope = "combined"
	}
	selected := s.dashboardVehicle(r, current.id, vehicles)
	if raw := r.URL.Query().Get("vehicle_id"); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		selected = nil
		if parseErr == nil {
			for i := range vehicles {
				if vehicles[i].ID == id {
					selected = &vehicles[i]
				}
			}
		}
	}

	chartRows, err := s.queryDashboardRefuelings(r, current.id, scope, selected, windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Údaje o tankovaní sa nepodarilo načítať.")
		return
	}
	chartCosts, err := s.queryDashboardCosts(r, current.id, scope, selected, windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočné náklady sa nepodarilo načítať.")
		return
	}
	labels := dashboardLabels(chartRows, chartCosts)
	datasets := dashboardDatasets(scope, selected, chartRows, labels)
	chartTitle := "Vozidlo"
	if scope == "combined" {
		chartTitle = "Vozidlá spolu"
	} else if selected != nil {
		chartTitle = selected.Name
	}
	chartTitle += " (" + windowStart + " - " + windowEnd + ")"

	statsVehicle := selected
	if scope == "combined" {
		statsVehicle = nil
	}
	stats, err := s.dashboardStats(r, current.id, scope, statsVehicle, windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Štatistiky sa nepodarilo vypočítať.")
		return
	}
	totalCount, err := s.countTableRefuelings(r, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Záznamy tankovaní sa nepodarilo načítať.")
		return
	}
	page := queryInt(r, "table_page", 1)
	if page < 1 {
		page = 1
	}
	totalPages := (totalCount + dashboardPageSize - 1) / dashboardPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	pageItems, err := s.queryTableRefuelings(r, current.id, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Záznamy tankovaní sa nepodarilo načítať.")
		return
	}
	allCosts, err := s.queryTableCosts(r, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočné náklady sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scope":               scope,
		"offset":              offset,
		"window":              map[string]string{"start": windowStart, "end": windowEnd},
		"selected_vehicle_id": selectedID(selected),
		"vehicles":            vehicles,
		"chart": dashboardChart{
			Title: chartTitle, Labels: labels, Datasets: datasets, AdditionalCosts: chartCosts,
		},
		"stats": stats,
		"refuelings": map[string]any{
			"page": page, "total_pages": totalPages, "total_count": totalCount, "items": pageItems,
		},
		"additional_costs": allCosts,
	})
}

func (s *Server) dashboardVehicle(r *http.Request, userID int64, vehicles []vehicleRecord) *vehicleRecord {
	var id int64
	err := s.db.QueryRowContext(r.Context(), `
		SELECT v.id FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id
		WHERE v.user_id = $1 ORDER BY r.refueled_on DESC, r.created_at DESC LIMIT 1
	`, userID).Scan(&id)
	if err == nil {
		for i := range vehicles {
			if vehicles[i].ID == id {
				return &vehicles[i]
			}
		}
	}
	if len(vehicles) > 0 {
		return &vehicles[0]
	}
	return nil
}

func (s *Server) queryDashboardRefuelings(r *http.Request, userID int64, scope string, selected *vehicleRecord, from, to string) ([]dashboardRefueling, error) {
	query := `
		SELECT r.id, r.vehicle_id, v.name, v.fuel_type, r.refueled_on, r.distance_km, r.amount, r.cost
		FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id
		WHERE v.user_id = $1 AND r.refueled_on BETWEEN $2 AND $3
	`
	args := []any{userID, from, to}
	if scope != "combined" {
		if selected == nil {
			query += " AND FALSE"
		} else {
			query += " AND r.vehicle_id = $4"
			args = append(args, selected.ID)
		}
	}
	query += " ORDER BY r.refueled_on ASC, r.created_at ASC, v.name ASC"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	records := make([]dashboardRefueling, 0)
	for rows.Next() {
		var record dashboardRefueling
		var fuelType int
		if err := rows.Scan(&record.ID, &record.VehicleID, &record.VehicleName, &fuelType, &record.date, &record.distance, &record.amount, &record.cost); err != nil {
			return nil, err
		}
		vehicle := makeVehicleRecord(record.VehicleID, record.VehicleName, fuelType)
		record.Unit = vehicle.Unit
		record.RefueledOn = record.date.Format(time.DateOnly)
		record.DistanceKM = record.distance.StringFixed(2)
		record.Amount = record.amount.StringFixed(2)
		record.Cost = record.cost.StringFixed(2)
		record.ConsumptionPer100KM = domain.ConsumptionPer100Km(record.amount, record.distance).StringFixed(2)
		record.CostPer100KM = domain.CostPer100Km(record.cost, record.distance).StringFixed(2)
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Server) queryDashboardCosts(r *http.Request, userID int64, scope string, selected *vehicleRecord, from, to string) ([]dashboardCost, error) {
	query := `
		SELECT c.id, c.vehicle_id, v.name, c.occurred_on, c.kind, c.comment, c.cost
		FROM additional_costs c JOIN vehicles v ON v.id = c.vehicle_id
		WHERE v.user_id = $1 AND c.occurred_on BETWEEN $2 AND $3
	`
	args := []any{userID, from, to}
	if scope != "combined" {
		if selected == nil {
			query += " AND FALSE"
		} else {
			query += " AND c.vehicle_id = $4"
			args = append(args, selected.ID)
		}
	}
	query += " ORDER BY c.occurred_on ASC, c.created_at ASC"
	return s.loadCosts(r, query, args...)
}

func (s *Server) queryTableCosts(r *http.Request, userID int64) ([]dashboardCost, error) {
	return s.loadCosts(r, `
		SELECT c.id, c.vehicle_id, v.name, c.occurred_on, c.kind, c.comment, c.cost
		FROM additional_costs c JOIN vehicles v ON v.id = c.vehicle_id
		WHERE v.user_id = $1 ORDER BY c.occurred_on DESC, c.created_at DESC
	`, userID)
}

func (s *Server) loadCosts(r *http.Request, query string, args ...any) ([]dashboardCost, error) {
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	costs := make([]dashboardCost, 0)
	for rows.Next() {
		var cost dashboardCost
		var occurred time.Time
		var kind int
		var amount decimal.Decimal
		if err := rows.Scan(&cost.ID, &cost.VehicleID, &cost.VehicleName, &occurred, &kind, &cost.Comment, &amount); err != nil {
			return nil, err
		}
		cost.OccurredOn = occurred.Format(time.DateOnly)
		if kind >= 0 && kind < len(costKindNames) {
			cost.Kind, cost.KindLabel = costKindNames[kind], costKindLabel(kind)
		}
		cost.Cost = amount.StringFixed(2)
		costs = append(costs, cost)
	}
	return costs, rows.Err()
}

func (s *Server) dashboardStats(r *http.Request, userID int64, scope string, selected *vehicleRecord, from, to string) (dashboardStats, error) {
	var stats dashboardStats
	query := `
		SELECT COALESCE(SUM(r.amount), 0), COALESCE(SUM(r.cost), 0), COALESCE(SUM(r.distance_km), 0)
		FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id
		WHERE v.user_id = $1 AND r.refueled_on BETWEEN $2 AND $3
	`
	args := []any{userID, from, to}
	if scope != "combined" {
		if selected == nil {
			query += " AND FALSE"
		} else {
			query += " AND r.vehicle_id = $4"
			args = append(args, selected.ID)
		}
	}
	var amount, fuelCost, distance decimal.Decimal
	if err := s.db.QueryRowContext(r.Context(), query, args...).Scan(&amount, &fuelCost, &distance); err != nil {
		return stats, err
	}
	stats.TotalFuelRefilled = amount.StringFixed(2)
	stats.TotalFuelCostPaid = fuelCost.StringFixed(2)
	if !distance.IsZero() {
		average := amount.Div(distance).Mul(decimal.NewFromInt(100)).Round(1).StringFixed(1)
		stats.AverageConsumptionPer100KM = &average
	}

	costQuery := `
		SELECT COALESCE(SUM(c.cost), 0) FROM additional_costs c JOIN vehicles v ON v.id = c.vehicle_id
		WHERE v.user_id = $1 AND c.occurred_on BETWEEN $2 AND $3
	`
	costArgs := []any{userID, from, to}
	if scope != "combined" {
		if selected == nil {
			costQuery += " AND FALSE"
		} else {
			costQuery += " AND c.vehicle_id = $4"
			costArgs = append(costArgs, selected.ID)
		}
	}
	var additionalCost decimal.Decimal
	if err := s.db.QueryRowContext(r.Context(), costQuery, costArgs...).Scan(&additionalCost); err != nil {
		return stats, err
	}
	stats.TotalAdditionalCostPaid = additionalCost.StringFixed(2)
	stats.TotalCostPaid = fuelCost.Add(additionalCost).StringFixed(2)

	averageDays, err := s.averageDaysBetween(r, userID, selectedForStats(scope, selected))
	if err != nil {
		return stats, err
	}
	stats.AverageDaysBetweenRefuelings = averageDays

	latestQuery := `SELECT MAX(r.refueled_on) FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1`
	latestArgs := []any{userID}
	if selectedForStats(scope, selected) != nil {
		latestQuery += " AND r.vehicle_id = $2"
		latestArgs = append(latestArgs, selected.ID)
	}
	var latest sql.NullTime
	if err := s.db.QueryRowContext(r.Context(), latestQuery, latestArgs...).Scan(&latest); err != nil {
		return stats, err
	}
	if latest.Valid {
		today, err := time.Parse(time.DateOnly, time.Now().In(s.appTZ).Format(time.DateOnly))
		if err != nil {
			return stats, err
		}
		lastDate, err := time.Parse(time.DateOnly, latest.Time.Format(time.DateOnly))
		if err != nil {
			return stats, err
		}
		days := int(today.Sub(lastDate).Hours() / 24)
		stats.DaysSinceLastRefueling = &days
	}
	return stats, nil
}

func (s *Server) averageDaysBetween(r *http.Request, userID int64, selected *vehicleRecord) (*string, error) {
	query := `SELECT r.refueled_on FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1`
	args := []any{userID}
	if selected != nil {
		query += " AND r.vehicle_id = $2"
		args = append(args, selected.ID)
	}
	query += " ORDER BY r.refueled_on"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var previous time.Time
	var sum int64
	var intervals int64
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		if !previous.IsZero() {
			sum += int64(date.Sub(previous).Hours() / 24)
			intervals++
		}
		previous = date
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if intervals == 0 {
		return nil, nil
	}
	average := decimal.NewFromInt(sum).Div(decimal.NewFromInt(intervals)).Round(1).StringFixed(1)
	return &average, nil
}

func (s *Server) countTableRefuelings(r *http.Request, userID int64) (int, error) {
	var count int
	if err := s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM refuelings r JOIN vehicles v ON v.id=r.vehicle_id WHERE v.user_id=$1`, userID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Server) queryTableRefuelings(r *http.Request, userID int64, page int) ([]refuelingRecord, error) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT r.id, r.vehicle_id, v.name, v.fuel_type, r.refueled_on, r.distance_km, r.amount, r.cost
		FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id WHERE v.user_id = $1
		ORDER BY r.refueled_on DESC, r.created_at DESC LIMIT $2 OFFSET $3
	`, userID, dashboardPageSize, (page-1)*dashboardPageSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	records := make([]refuelingRecord, 0, dashboardPageSize)
	for rows.Next() {
		var record refuelingRecord
		var date time.Time
		var fuelType int
		var distance, amount, cost decimal.Decimal
		if err := rows.Scan(&record.ID, &record.VehicleID, &record.VehicleName, &fuelType, &date, &distance, &amount, &cost); err != nil {
			return nil, err
		}
		record.RefueledOn = date.Format(time.DateOnly)
		record.Unit = makeVehicleRecord(record.VehicleID, record.VehicleName, fuelType).Unit
		record.DistanceKM, record.Amount, record.Cost = distance.StringFixed(2), amount.StringFixed(2), cost.StringFixed(2)
		record.ConsumptionPer100KM = domain.ConsumptionPer100Km(amount, distance).StringFixed(2)
		record.CostPer100KM = domain.CostPer100Km(cost, distance).StringFixed(2)
		records = append(records, record)
	}
	return records, rows.Err()
}

func dashboardLabels(rows []dashboardRefueling, costs []dashboardCost) []string {
	set := make(map[string]bool)
	for _, row := range rows {
		set[row.RefueledOn] = true
	}
	if len(set) == 0 {
		for _, cost := range costs {
			set[cost.OccurredOn] = true
		}
	}
	labels := make([]string, 0, len(set))
	for label := range set {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

func dashboardDatasets(scope string, selected *vehicleRecord, rows []dashboardRefueling, labels []string) []dashboardDataset {
	if scope != "combined" {
		unit := "jednotka"
		if selected != nil {
			unit = selected.Unit
		}
		consumption := make(map[string]*string)
		cost := make(map[string]*string)
		for _, row := range rows {
			consumption[row.RefueledOn] = stringPointer(exportDecimal(decimal.RequireFromString(row.ConsumptionPer100KM)))
			cost[row.RefueledOn] = stringPointer(exportDecimal(decimal.RequireFromString(row.CostPer100KM)))
		}
		return []dashboardDataset{
			{Label: "Spotreba na 100 km (" + unit + ")", YAxisID: "yConsumption", BorderColor: "#1f5aff", BackgroundColor: "rgba(31, 90, 255, 0.2)", Data: alignChartData(labels, consumption), Tension: 0.2},
			{Label: "Náklady na 100 km", YAxisID: "yCost", BorderColor: "#ff5a1f", BackgroundColor: "rgba(255, 90, 31, 0.2)", BorderDash: []int{6, 4}, Data: alignChartData(labels, cost), Tension: 0.2},
		}
	}
	groups := make([]int64, 0)
	rowsByVehicle := make(map[int64][]dashboardRefueling)
	vehicles := make(map[int64]vehicleRecord)
	for _, row := range rows {
		if _, ok := rowsByVehicle[row.VehicleID]; !ok {
			groups = append(groups, row.VehicleID)
			vehicles[row.VehicleID] = vehicleRecord{ID: row.VehicleID, Name: row.VehicleName, Unit: row.Unit}
		}
		rowsByVehicle[row.VehicleID] = append(rowsByVehicle[row.VehicleID], row)
	}
	datasets := make([]dashboardDataset, 0, len(groups)*2)
	for index, id := range groups {
		byDateConsumption := make(map[string]*string)
		byDateCost := make(map[string]*string)
		for _, row := range rowsByVehicle[id] {
			byDateConsumption[row.RefueledOn] = stringPointer(exportDecimal(decimal.RequireFromString(row.ConsumptionPer100KM)))
			byDateCost[row.RefueledOn] = stringPointer(exportDecimal(decimal.RequireFromString(row.CostPer100KM)))
		}
		vehicle := vehicles[id]
		consumptionColor := "hsl(" + strconv.Itoa((index*57)%360) + " 80% 45%)"
		costColor := "hsl(" + strconv.Itoa((index*57+24)%360) + " 70% 40%)"
		datasets = append(datasets,
			dashboardDataset{Label: vehicle.Name + ": Spotreba (" + vehicle.Unit + ")", YAxisID: "yConsumption", BorderColor: consumptionColor, BackgroundColor: consumptionColor, Data: alignChartData(labels, byDateConsumption), Tension: 0.2},
			dashboardDataset{Label: vehicle.Name + ": Náklady na 100 km", YAxisID: "yCost", BorderColor: costColor, BackgroundColor: costColor, BorderDash: []int{6, 4}, Data: alignChartData(labels, byDateCost), Tension: 0.2},
		)
	}
	return datasets
}

func alignChartData(labels []string, values map[string]*string) []*string {
	data := make([]*string, len(labels))
	for i, label := range labels {
		data[i] = values[label]
	}
	return data
}

func stringPointer(value string) *string { return &value }

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func selectedID(vehicle *vehicleRecord) *int64 {
	if vehicle == nil {
		return nil
	}
	return &vehicle.ID
}

func selectedForStats(scope string, selected *vehicleRecord) *vehicleRecord {
	if scope == "combined" {
		return nil
	}
	return selected
}
