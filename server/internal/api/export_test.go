package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
)

func TestExportReturnsV1NestedDataWithOwnedRecordsOnly(t *testing.T) {
	server, db := newTestServer(t)
	handler := server.Handler()
	cookie := registerTestUser(t, handler, "export@example.com")
	registerTestUser(t, handler, "export-other@example.com")
	var userID, otherID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='export@example.com'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='export-other@example.com'").Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	var vehicleID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Export car',1,'2026-01-02 03:04:05.123456','2026-01-02 03:04:05.123456') RETURNING id`, userID).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Private',0,now(),now())`, otherID); err != nil {
		t.Fatal(err)
	}
	var refuelingID, costID, ruleID, leadID int64
	if err := db.QueryRow(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1,'2026-01-10','512.30','35.50','56.80','2026-01-02 03:04:07.5','2026-01-02 03:04:07.5') RETURNING id`, vehicleID).Scan(&refuelingID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,comment,created_at,updated_at)
		VALUES ($1,'2026-01-15',0,'89.90','Castrol 5W-30','2026-01-02 03:04:09','2026-01-02 03:04:09') RETURNING id`, vehicleID).Scan(&costID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_rules (vehicle_id,kind,interval_days,interval_km,active,created_at,updated_at)
		VALUES ($1,0,365,15000,true,'2026-01-02 03:04:11','2026-01-02 03:04:11') RETURNING id`, vehicleID).Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,days_before,created_at,updated_at)
		VALUES ($1,30,'2026-01-02 03:04:12','2026-01-02 03:04:12') RETURNING id`, ruleID).Scan(&leadID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO maintenance_notifications (maintenance_reminder_rule_id,maintenance_reminder_lead_id,additional_cost_id,notification_kind,trigger_condition,status,days_remaining,sent_at,created_at,updated_at)
		VALUES ($1,$2,$3,0,0,1,25,'2026-02-01 08:00:00.5','2026-01-02 03:04:14','2026-01-02 03:04:14')`, ruleID, leadID, costID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Foreign',0,now(),now())`, otherID); err != nil {
		t.Fatal(err)
	}
	_ = refuelingID
	if _, err := server.buildExport(t.Context(), userID); err != nil {
		t.Fatalf("buildExport: %v", err)
	}

	response := apiRequest(handler, http.MethodGet, "/api/user_data/export", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("export status = %d: %s", response.Code, response.Body)
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || !regexp.MustCompile(`attachment; filename="spotreba-\d{8}-\d{6}\.json"`).MatchString(response.Header().Get("Content-Disposition")) {
		t.Fatalf("export headers = %+v", response.Header())
	}
	var export struct {
		Format     string `json:"format"`
		Version    int    `json:"version"`
		ExportedAt string `json:"exported_at"`
		Account    struct {
			Email string `json:"email_address"`
		} `json:"account"`
		Vehicles []struct {
			Name       string `json:"name"`
			FuelType   string `json:"fuel_type"`
			CreatedAt  string `json:"created_at"`
			Refuelings []struct {
				Date     string `json:"refueled_on"`
				Distance string `json:"distance_km"`
				Amount   string `json:"amount"`
				Cost     string `json:"cost"`
			} `json:"refuelings"`
			AdditionalCosts []struct {
				Comment string `json:"comment"`
				Kind    string `json:"kind"`
				Cost    string `json:"cost"`
			} `json:"additional_costs"`
			Rules []struct {
				Kind       string `json:"kind"`
				IntervalKM string `json:"interval_km"`
				Leads      []struct {
					Days int `json:"days_before"`
				} `json:"maintenance_reminder_leads"`
			} `json:"maintenance_reminder_rules"`
			Notifications []struct {
				RuleID    int64  `json:"maintenance_reminder_rule_id"`
				LeadID    int64  `json:"maintenance_reminder_lead_id"`
				CostID    int64  `json:"additional_cost_id"`
				Kind      string `json:"notification_kind"`
				Condition string `json:"trigger_condition"`
				Status    string `json:"status"`
				Days      int    `json:"days_remaining"`
			} `json:"maintenance_notifications"`
		} `json:"vehicles"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &export); err != nil {
		t.Fatal(err)
	}
	if export.Format != "spotreba" || export.Version != 1 || export.Account.Email != "export@example.com" || !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`).MatchString(export.ExportedAt) {
		t.Fatalf("export header = %+v", export)
	}
	if len(export.Vehicles) != 1 || export.Vehicles[0].Name != "Export car" || export.Vehicles[0].FuelType != "diesel" || export.Vehicles[0].CreatedAt != "2026-01-02T03:04:05.123456Z" {
		t.Fatalf("export vehicles = %+v", export.Vehicles)
	}
	vehicle := export.Vehicles[0]
	if len(vehicle.Refuelings) != 1 || vehicle.Refuelings[0].Date != "2026-01-10" || vehicle.Refuelings[0].Distance != "512.3" || vehicle.Refuelings[0].Amount != "35.5" || vehicle.Refuelings[0].Cost != "56.8" {
		t.Fatalf("export refuelings = %+v", vehicle.Refuelings)
	}
	if len(vehicle.AdditionalCosts) != 1 || vehicle.AdditionalCosts[0].Kind != "oil_change" || vehicle.AdditionalCosts[0].Comment != "Castrol 5W-30" || vehicle.AdditionalCosts[0].Cost != "89.9" {
		t.Fatalf("export costs = %+v", vehicle.AdditionalCosts)
	}
	if len(vehicle.Rules) != 1 || vehicle.Rules[0].Kind != "oil_change" || vehicle.Rules[0].IntervalKM != "15000.0" || len(vehicle.Rules[0].Leads) != 1 || vehicle.Rules[0].Leads[0].Days != 30 {
		t.Fatalf("export rules = %+v", vehicle.Rules)
	}
	if len(vehicle.Notifications) != 1 || vehicle.Notifications[0].Kind != "advance" || vehicle.Notifications[0].Condition != "date" || vehicle.Notifications[0].Status != "sent" || vehicle.Notifications[0].Days != 25 || vehicle.Notifications[0].RuleID != ruleID || vehicle.Notifications[0].LeadID != leadID || vehicle.Notifications[0].CostID != costID {
		t.Fatalf("export notifications = %+v", vehicle.Notifications)
	}
}

func TestExportRequiresAuthentication(t *testing.T) {
	handler, _ := newTestHandler(t)
	response := apiRequest(handler, http.MethodGet, "/api/user_data/export", "", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated export status = %d, want 401", response.Code)
	}
}
