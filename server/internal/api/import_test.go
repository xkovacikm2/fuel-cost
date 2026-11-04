package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func multipartImport(handler http.Handler, cookie *http.Cookie, contents string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, _ := writer.CreateFormFile("file", "spotreba.json")
	_, _ = file.Write([]byte(contents))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/user_data/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestImportMergesV1GraphAndSkipsDuplicates(t *testing.T) {
	handler, db := newTestHandler(t)
	source := registerTestUser(t, handler, "import-source@example.com")
	target := registerTestUser(t, handler, "import-target@example.com")
	var sourceID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='import-source@example.com'").Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	var petrolID, electricID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Imported Diesel',1,'2026-01-02 03:04:05.123456','2026-01-02 03:04:05.123456') RETURNING id`, sourceID).Scan(&petrolID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Imported Electric',2,'2026-01-02 03:04:06','2026-01-02 03:04:06') RETURNING id`, sourceID).Scan(&electricID); err != nil {
		t.Fatal(err)
	}
	var refuelingID, costID, ruleID, dayLeadID int64
	if err := db.QueryRow(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1,'2026-01-10',512.30,35.50,56.80,'2026-01-02 03:04:07.5','2026-01-02 03:04:07.5') RETURNING id`, petrolID).Scan(&refuelingID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1,'2026-02-10',600,40,60.25,'2026-01-02 03:04:08','2026-01-02 03:04:08')`, petrolID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,comment,created_at,updated_at)
		VALUES ($1,'2026-01-15',0,89.90,'Oil','2026-01-02 03:04:09','2026-01-02 03:04:09') RETURNING id`, petrolID).Scan(&costID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,comment,created_at,updated_at)
		VALUES ($1,'2026-02-01',5,120,NULL,'2026-01-02 03:04:10','2026-01-02 03:04:10')`, petrolID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_rules (vehicle_id,kind,interval_days,interval_km,active,created_at,updated_at)
		VALUES ($1,0,365,15000,true,'2026-01-02 03:04:11','2026-01-02 03:04:11') RETURNING id`, petrolID).Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,days_before,created_at,updated_at)
		VALUES ($1,30,'2026-01-02 03:04:12','2026-01-02 03:04:12') RETURNING id`, ruleID).Scan(&dayLeadID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,kilometres_before,created_at,updated_at)
		VALUES ($1,1000,'2026-01-02 03:04:13','2026-01-02 03:04:13')`, ruleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO maintenance_notifications (maintenance_reminder_rule_id,maintenance_reminder_lead_id,additional_cost_id,notification_kind,trigger_condition,status,days_remaining,sent_at,created_at,updated_at)
		VALUES ($1,$2,$3,0,0,1,25,'2026-02-01 08:00:00.5','2026-01-02 03:04:14','2026-01-02 03:04:14')`, ruleID, dayLeadID, costID); err != nil {
		t.Fatal(err)
	}
	_ = electricID
	_ = refuelingID

	export := apiRequest(handler, http.MethodGet, "/api/user_data/export", "", source)
	if export.Code != http.StatusOK {
		t.Fatalf("source export status = %d: %s", export.Code, export.Body)
	}
	first := multipartImport(handler, target, export.Body.String())
	if first.Code != http.StatusOK {
		t.Fatalf("first import status = %d: %s", first.Code, first.Body)
	}
	if !strings.Contains(first.Body.String(), `"notice":"Import: 10 nových, 0 preskočených záznamov."`) {
		t.Fatalf("first import result = %s", first.Body)
	}
	second := multipartImport(handler, target, export.Body.String())
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"notice":"Import: 0 nových, 10 preskočených záznamov. Duplicity: 10."`) {
		t.Fatalf("second import result = %d %s", second.Code, second.Body)
	}
	var vehicles, refuelings, costs, rules, leads, notifications int
	queries := []struct {
		table string
		dest  *int
	}{
		{"vehicles", &vehicles}, {"refuelings", &refuelings}, {"additional_costs", &costs},
		{"maintenance_reminder_rules", &rules}, {"maintenance_reminder_leads", &leads}, {"maintenance_notifications", &notifications},
	}
	for _, query := range queries {
		sqlQuery := fmt.Sprintf("SELECT count(*) FROM %s WHERE ", query.table)
		switch query.table {
		case "vehicles":
			sqlQuery += "user_id = $1"
		case "refuelings", "additional_costs":
			sqlQuery += "vehicle_id IN (SELECT id FROM vehicles WHERE user_id = $1)"
		case "maintenance_reminder_rules":
			sqlQuery += "vehicle_id IN (SELECT id FROM vehicles WHERE user_id = $1)"
		case "maintenance_reminder_leads":
			sqlQuery += "maintenance_reminder_rule_id IN (SELECT id FROM maintenance_reminder_rules WHERE vehicle_id IN (SELECT id FROM vehicles WHERE user_id = $1))"
		default:
			sqlQuery += "maintenance_reminder_rule_id IN (SELECT id FROM maintenance_reminder_rules WHERE vehicle_id IN (SELECT id FROM vehicles WHERE user_id = $1))"
		}
		if err := db.QueryRow(sqlQuery, targetID(t, db, "import-target@example.com")).Scan(query.dest); err != nil {
			t.Fatal(err)
		}
	}
	if vehicles != 2 || refuelings != 2 || costs != 2 || rules != 1 || leads != 2 || notifications != 1 {
		t.Fatalf("imported counts = %d vehicles, %d refuelings, %d costs, %d rules, %d leads, %d notifications", vehicles, refuelings, costs, rules, leads, notifications)
	}
}

func TestImportRejectsInvalidFilesWithoutWriting(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "import-invalid@example.com")
	crossReference := `{"format":"spotreba","version":1,"exported_at":"2026-01-01T00:00:00Z","account":{"email_address":"source@example.com","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"vehicles":[{"id":1,"name":"Car","fuel_type":"petrol","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","refuelings":[],"additional_costs":[{"id":2,"occurred_on":"2026-01-01","kind":"oil_change","comment":null,"cost":"1.0","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}],"maintenance_reminder_rules":[{"id":3,"kind":"oil_change","interval_days":30,"interval_km":null,"active":true,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","maintenance_reminder_leads":[]}],"maintenance_notifications":[{"id":4,"maintenance_reminder_rule_id":99,"maintenance_reminder_lead_id":null,"additional_cost_id":2,"notification_kind":"due","trigger_condition":"date","status":"sent","days_remaining":null,"kilometres_remaining":null,"sent_at":null,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}]}`
	for _, input := range []string{"{", `{"format":"other","version":1,"exported_at":"2026-01-01T00:00:00Z","account":{"email_address":"x@example.com","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"vehicles":[]}`, crossReference} {
		response := multipartImport(handler, cookie, input)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid import status = %d, want 422: %s", response.Code, response.Body)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM vehicles").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid import created %d vehicles", count)
	}
}

func targetID(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address=$1", email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
