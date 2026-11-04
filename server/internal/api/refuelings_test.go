package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewRefuelingDefaultsToMostRecentlyUsedVehicleAndUnit(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "refueling-default@example.com")
	var petrolID, electricID int64
	err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Petrol car',0,now(),now() FROM users WHERE email_address='refueling-default@example.com' RETURNING id`).Scan(&petrolID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Electric car',2,now(),now() FROM users WHERE email_address='refueling-default@example.com' RETURNING id`).Scan(&electricID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1, current_date - 10, 100, 8, 15, now() - interval '10 days', now() - interval '10 days'),
		       ($2, current_date - 2, 200, 30, 12, now() - interval '2 days', now() - interval '2 days')`, petrolID, electricID); err != nil {
		t.Fatal(err)
	}

	response := apiRequest(handler, http.MethodGet, "/api/refuelings/new", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("new refueling status = %d: %s", response.Code, response.Body)
	}
	var body struct {
		RefueledOn string          `json:"refueled_on"`
		VehicleID  int64           `json:"vehicle_id"`
		Unit       string          `json:"unit"`
		Vehicles   []vehicleRecord `json:"vehicles"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.VehicleID != electricID || body.Unit != "kWh" {
		t.Fatalf("default vehicle=%d unit=%q; want %d kWh", body.VehicleID, body.Unit, electricID)
	}
	if body.RefueledOn != time.Now().In(time.Local).Format("2006-01-02") {
		t.Fatalf("default date = %q", body.RefueledOn)
	}
	if len(body.Vehicles) != 2 || body.Vehicles[0].Name != "Electric car" {
		t.Fatalf("vehicles not sorted or scoped: %+v", body.Vehicles)
	}
}

func TestRefuelingCRUDKeepsDecimalPrecisionAndScopesVehicles(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "refueling-owner@example.com")
	otherCookie := registerTestUser(t, handler, "refueling-other@example.com")
	var vehicleID, foreignVehicleID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'My car',0,now(),now() FROM users WHERE email_address='refueling-owner@example.com' RETURNING id`).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Other car',0,now(),now() FROM users WHERE email_address='refueling-other@example.com' RETURNING id`).Scan(&foreignVehicleID); err != nil {
		t.Fatal(err)
	}

	created := apiRequest(handler, http.MethodPost, "/api/refuelings", fmt.Sprintf(
		`{"vehicle_id":%d,"refueled_on":"2026-01-12","distance_km":"512.30","amount":"35.50","cost":"56.80"}`,
		vehicleID,
	), cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}
	var body struct {
		Refueling struct {
			ID                  int64  `json:"id"`
			DistanceKM          string `json:"distance_km"`
			Amount              string `json:"amount"`
			Cost                string `json:"cost"`
			ConsumptionPer100KM string `json:"consumption_per_100km"`
			CostPer100KM        string `json:"cost_per_100km"`
		} `json:"refueling"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Refueling.ID < 1 || body.Refueling.DistanceKM != "512.30" || body.Refueling.Amount != "35.50" || body.Refueling.Cost != "56.80" || body.Refueling.ConsumptionPer100KM != "6.93" || body.Refueling.CostPer100KM != "11.09" {
		t.Fatalf("created refueling = %+v", body.Refueling)
	}

	foreign := apiRequest(handler, http.MethodPost, "/api/refuelings", fmt.Sprintf(
		`{"vehicle_id":%d,"refueled_on":"2026-01-12","distance_km":"100","amount":"7","cost":"10"}`,
		foreignVehicleID,
	), cookie)
	if foreign.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign create status = %d, want 422", foreign.Code)
	}

	for _, values := range [][3]string{{"0", "7", "10"}, {"100", "0", "10"}, {"100", "7", "-1"}, {"100000000", "7", "10"}} {
		invalid := apiRequest(handler, http.MethodPost, "/api/refuelings", fmt.Sprintf(
			`{"vehicle_id":%d,"refueled_on":"2026-01-12","distance_km":%q,"amount":%q,"cost":%q}`,
			vehicleID, values[0], values[1], values[2]), cookie)
		if invalid.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid values %v status = %d, want 422: %s", values, invalid.Code, invalid.Body)
		}
	}

	updated := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/refuelings/%d", body.Refueling.ID),
		`{"amount":"36.25"}`, cookie)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"amount":"36.25"`) {
		t.Fatalf("update response = %d %s", updated.Code, updated.Body)
	}
	foreignRead := apiRequest(handler, http.MethodGet, fmt.Sprintf("/api/refuelings/%d", body.Refueling.ID), "", otherCookie)
	if foreignRead.Code != http.StatusNotFound {
		t.Fatalf("foreign read status = %d, want 404", foreignRead.Code)
	}
	deleted := apiRequest(handler, http.MethodDelete, fmt.Sprintf("/api/refuelings/%d", body.Refueling.ID), "", cookie)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleted.Code)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM refuelings WHERE vehicle_id = $1", vehicleID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("refuelings after delete = %d, want 0", count)
	}
}
