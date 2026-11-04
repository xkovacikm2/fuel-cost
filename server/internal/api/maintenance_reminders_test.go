package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestNewReminderDefaultsAndListCurrentCycle(t *testing.T) {
	server, db := newTestServer(t)
	handler := server.Handler()
	cookie := registerTestUser(t, handler, "reminder-default@example.com")
	var userID, firstID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='reminder-default@example.com'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Zeta',0,now(),now())`, userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Alfa',0,now(),now()) RETURNING id`, userID).Scan(&firstID); err != nil {
		t.Fatal(err)
	}

	newResponse := apiRequest(handler, http.MethodGet, "/api/maintenance_reminders/new", "", cookie)
	if newResponse.Code != http.StatusOK {
		t.Fatalf("new reminder status = %d: %s", newResponse.Code, newResponse.Body)
	}
	var newBody struct {
		VehicleID int64 `json:"vehicle_id"`
		Active    bool  `json:"active"`
		Leads     []any `json:"leads"`
	}
	if err := json.Unmarshal(newResponse.Body.Bytes(), &newBody); err != nil {
		t.Fatal(err)
	}
	if newBody.VehicleID != firstID || !newBody.Active || len(newBody.Leads) != 1 {
		t.Fatalf("new reminder defaults = %+v", newBody)
	}

	var ruleID int64
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_rules (vehicle_id,kind,interval_days,interval_km,active,created_at,updated_at)
		VALUES ($1,0,365,10000,true,now(),now()) RETURNING id`, firstID).Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,days_before,created_at,updated_at)
		VALUES ($1,30,now(),now())`, ruleID); err != nil {
		t.Fatal(err)
	}
	baseline := time.Now().In(time.Local).AddDate(0, 0, -100).Format(time.DateOnly)
	if _, err := db.Exec(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,created_at,updated_at) VALUES ($1,$2,0,80,now(),now())`, firstID, baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1,$2::date + 1,1000,70,100,now(),now()), ($1,$2::date + 10,1000,70,100,now(),now())`, firstID, baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := server.findReminder(context.Background(), fmt.Sprint(ruleID), userID); err != nil {
		t.Fatalf("find reminder with cycle: %v", err)
	}
	list := apiRequest(handler, http.MethodGet, "/api/maintenance_reminders", "", cookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list reminder status = %d: %s", list.Code, list.Body)
	}
	var listBody struct {
		Reminders []struct {
			VehicleName         string `json:"vehicle_name"`
			KindLabel           string `json:"kind_label"`
			Status              string `json:"status"`
			DaysRemaining       *int   `json:"days_remaining"`
			KilometresRemaining string `json:"kilometres_remaining"`
			BaselineDate        string `json:"baseline_date"`
		} `json:"reminders"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Reminders) != 1 {
		t.Fatalf("reminders = %+v", listBody.Reminders)
	}
	reminder := listBody.Reminders[0]
	if reminder.VehicleName != "Alfa" || reminder.KindLabel != "Výmena oleja" || reminder.Status != "265 dní / 8000 km" || reminder.DaysRemaining == nil || *reminder.DaysRemaining != 265 || reminder.KilometresRemaining != "8000.00" || reminder.BaselineDate != baseline {
		t.Fatalf("current cycle = %+v", reminder)
	}
}

func TestReminderCreateUpdateAndNestedLeadRemoval(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "reminder-crud@example.com")
	var userID, vehicleID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='reminder-crud@example.com'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Car',0,now(),now()) RETURNING id`, userID).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}

	created := apiRequest(handler, http.MethodPost, "/api/maintenance_reminders", fmt.Sprintf(
		`{"vehicle_id":%d,"kind":"oil_change","interval_days":365,"interval_km":"15000","active":true,"leads":[{"days_before":30},{"kilometres_before":"1000"}]}`,
		vehicleID,
	), cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create reminder status = %d: %s", created.Code, created.Body)
	}
	var body struct {
		Reminder struct {
			ID    int64 `json:"id"`
			Leads []struct {
				ID int64 `json:"id"`
			} `json:"leads"`
		} `json:"reminder"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Reminder.ID < 1 || len(body.Reminder.Leads) != 2 {
		t.Fatalf("created reminder = %+v", body.Reminder)
	}
	dayLeadID := body.Reminder.Leads[0].ID
	update := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/maintenance_reminders/%d", body.Reminder.ID), fmt.Sprintf(
		`{"interval_days":730,"leads":[{"id":%d,"days_before":14},{"kilometres_before":"500"}]}`,
		dayLeadID,
	), cookie)
	if update.Code != http.StatusOK {
		t.Fatalf("update reminder status = %d: %s", update.Code, update.Body)
	}
	var intervals struct {
		Reminder struct {
			IntervalDays int `json:"interval_days"`
			Leads        []struct {
				ID               int64   `json:"id"`
				DaysBefore       *int    `json:"days_before"`
				KilometresBefore *string `json:"kilometres_before"`
			} `json:"leads"`
		} `json:"reminder"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &intervals); err != nil {
		t.Fatal(err)
	}
	if intervals.Reminder.IntervalDays != 730 || len(intervals.Reminder.Leads) != 2 {
		t.Fatalf("updated reminder = %+v", intervals.Reminder)
	}
	if intervals.Reminder.Leads[0].ID != dayLeadID || intervals.Reminder.Leads[0].DaysBefore == nil || *intervals.Reminder.Leads[0].DaysBefore != 14 || intervals.Reminder.Leads[1].KilometresBefore == nil || *intervals.Reminder.Leads[1].KilometresBefore != "500.00" {
		t.Fatalf("updated leads = %+v", intervals.Reminder.Leads)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id = $1", body.Reminder.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("lead rows after update = %d, want 2", count)
	}
}

func TestReminderValidationAndForeignOwnership(t *testing.T) {
	handler, db := newTestHandler(t)
	owner := registerTestUser(t, handler, "reminder-owner@example.com")
	registerTestUser(t, handler, "reminder-other@example.com")
	var ownerID, foreignID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='reminder-owner@example.com'").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT id FROM users WHERE email_address='reminder-other@example.com'").Scan(&foreignID); err != nil {
		t.Fatal(err)
	}
	var vehicleID, foreignVehicleID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Owner',0,now(),now()) RETURNING id`, ownerID).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,'Foreign',0,now(),now()) RETURNING id`, foreignID).Scan(&foreignVehicleID); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		fmt.Sprintf(`{"vehicle_id":%d,"kind":"oil_change"}`, vehicleID),
		fmt.Sprintf(`{"vehicle_id":%d,"kind":"oil_change","interval_days":30,"leads":[{"days_before":5,"kilometres_before":"100"}]}`, vehicleID),
		fmt.Sprintf(`{"vehicle_id":%d,"kind":"oil_change","interval_km":"1000","leads":[{"days_before":5}]}`, vehicleID),
		fmt.Sprintf(`{"vehicle_id":%d,"kind":"oil_change","interval_days":30}`, foreignVehicleID),
	} {
		response := apiRequest(handler, http.MethodPost, "/api/maintenance_reminders", body, owner)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("create with %s returned %d, want 422: %s", body, response.Code, response.Body)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM maintenance_reminder_rules").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid reminder requests created %d rows, want 0", count)
	}
	ruleID := insertTestReminderRule(t, db, vehicleID, 365, "10000", true, "oil_change")
	moved := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/maintenance_reminders/%d", ruleID), fmt.Sprintf(`{"vehicle_id":%d}`, foreignVehicleID), owner)
	if moved.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign reminder move status = %d, want 422: %s", moved.Code, moved.Body)
	}
	var persistedVehicle int64
	if err := db.QueryRow("SELECT vehicle_id FROM maintenance_reminder_rules WHERE id=$1", ruleID).Scan(&persistedVehicle); err != nil {
		t.Fatal(err)
	}
	if persistedVehicle != vehicleID {
		t.Fatalf("reminder moved to vehicle %d, want original owner vehicle %d", persistedVehicle, vehicleID)
	}
}
