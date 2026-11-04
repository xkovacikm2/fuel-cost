package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDashboardDefaultScopeAndVehicleMetrics(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "dashboard@example.com")
	var userID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='dashboard@example.com'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var vehicleA, vehicleB int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Auto A',0,now(),now()) RETURNING id`, userID).Scan(&vehicleA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Auto B',2,now(),now()) RETURNING id`, userID).Scan(&vehicleB); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		vehicle                int64
		days                   int
		distance, amount, cost string
	}{
		{vehicleA, 30, "500", "35", "56"},
		{vehicleA, 20, "400", "30", "48.6"},
		{vehicleA, 10, "600", "39", "62.4"},
		{vehicleB, 40, "100", "15", "4.5"},
	} {
		_, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
			VALUES ($1,current_date - $2::integer,$3,$4,$5,now() - ($2::integer * interval '1 day'),now() - ($2::integer * interval '1 day'))`, row.vehicle, row.days, row.distance, row.amount, row.cost)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := db.Exec(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,comment,created_at,updated_at)
		VALUES ($1,current_date - 15,5,1000.50,'Spojka',now(),now())`, vehicleA)
	if err != nil {
		t.Fatal(err)
	}
	registerTestUser(t, handler, "dashboard-other@example.com")
	var foreignUser int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='dashboard-other@example.com'").Scan(&foreignUser); err != nil {
		t.Fatal(err)
	}
	var foreignVehicle int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Private',0,now(),now()) RETURNING id`, foreignUser).Scan(&foreignVehicle); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at) VALUES ($1,current_date,100,99,999,now(),now())`, foreignVehicle); err != nil {
		t.Fatal(err)
	}

	response := apiRequest(handler, http.MethodGet, "/api/dashboard", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d: %s", response.Code, response.Body)
	}
	var body struct {
		Scope      string `json:"scope"`
		SelectedID int64  `json:"selected_vehicle_id"`
		Chart      struct {
			Labels   []string `json:"labels"`
			Datasets []struct {
				Label string `json:"label"`
				Data  []any  `json:"data"`
			} `json:"datasets"`
			AdditionalCosts []struct {
				Label string `json:"label"`
			} `json:"additional_costs"`
		} `json:"chart"`
		Stats struct {
			DaysSince          *int    `json:"days_since_last_refueling"`
			AverageConsumption *string `json:"average_consumption_per_100km"`
			TotalCost          string  `json:"total_cost_paid"`
			AverageDays        *string `json:"average_days_between_refuelings"`
			FuelRefilled       string  `json:"total_fuel_refilled"`
			FuelCost           string  `json:"total_fuel_cost_paid"`
			AdditionalCost     string  `json:"total_additional_cost_paid"`
		} `json:"stats"`
		Refuelings struct {
			TotalCount int               `json:"total_count"`
			Page       int               `json:"page"`
			TotalPages int               `json:"total_pages"`
			Items      []refuelingRecord `json:"items"`
		} `json:"refuelings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scope != "vehicle" || body.SelectedID != vehicleA || len(body.Chart.Datasets) != 2 || len(body.Chart.AdditionalCosts) != 1 {
		t.Fatalf("dashboard defaults = %+v", body)
	}
	if got := body.Chart.Datasets[0].Data; !reflect.DeepEqual(got, []any{"7.0", "7.5", "6.5"}) {
		t.Fatalf("consumption chart values = %v, want fixed-precision decimal strings", got)
	}
	if len(body.Chart.Labels) != 3 {
		t.Fatalf("chart labels = %v, want only refueling dates", body.Chart.Labels)
	}
	if body.Stats.DaysSince == nil || *body.Stats.DaysSince != 10 || body.Stats.AverageConsumption == nil || *body.Stats.AverageConsumption != "6.9" || body.Stats.TotalCost != "1167.50" || body.Stats.AverageDays == nil || *body.Stats.AverageDays != "10.0" || body.Stats.FuelRefilled != "104.00" || body.Stats.FuelCost != "167.00" || body.Stats.AdditionalCost != "1000.50" {
		t.Fatalf("vehicle stats = %+v", body.Stats)
	}

	combined := apiRequest(handler, http.MethodGet, "/api/dashboard?scope=combined", "", cookie)
	if combined.Code != http.StatusOK {
		t.Fatalf("combined dashboard status = %d: %s", combined.Code, combined.Body)
	}
	if err := json.Unmarshal(combined.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scope != "combined" || body.Stats.AverageConsumption == nil || *body.Stats.AverageConsumption != "7.4" || body.Stats.TotalCost != "1172.00" || body.Stats.FuelRefilled != "119.00" || body.Stats.FuelCost != "171.50" || len(body.Chart.Datasets) != 4 {
		t.Fatalf("combined dashboard = %+v", body)
	}
	if strings.Contains(combined.Body.String(), "Private") {
		t.Fatalf("combined dashboard leaked foreign vehicle: %s", combined.Body)
	}
}

func TestDashboardOffsetAndRefuelingPagination(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "dashboard-page@example.com")
	var userID, vehicleID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='dashboard-page@example.com'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Car',0,now(),now()) RETURNING id`, userID).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 12; index++ {
		if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
			VALUES ($1,current_date - $2::integer,100,7,10,now() - ($2::integer * interval '1 day'),now() - ($2::integer * interval '1 day'))`, vehicleID, index); err != nil {
			t.Fatal(err)
		}
	}
	response := apiRequest(handler, http.MethodGet, "/api/dashboard?table_page=2&offset=365", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard page status = %d: %s", response.Code, response.Body)
	}
	var body struct {
		Offset int `json:"offset"`
		Window struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"window"`
		Refuelings struct {
			Page       int               `json:"page"`
			TotalCount int               `json:"total_count"`
			TotalPages int               `json:"total_pages"`
			Items      []refuelingRecord `json:"items"`
		} `json:"refuelings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Offset != 365 || body.Refuelings.Page != 2 || body.Refuelings.TotalCount != 12 || body.Refuelings.TotalPages != 2 || len(body.Refuelings.Items) != 2 {
		t.Fatalf("dashboard pagination = %+v", body)
	}
	if body.Window.End == "" || body.Window.Start == "" || body.Window.End != time.Now().In(time.Local).AddDate(0, 0, -365).Format(time.DateOnly) {
		t.Fatalf("dashboard window = %+v", body.Window)
	}
	if body.Refuelings.Items[0].RefueledOn != time.Now().In(time.Local).AddDate(0, 0, -11).Format(time.DateOnly) {
		t.Fatalf("second page newest item = %+v", body.Refuelings.Items[0])
	}
}
