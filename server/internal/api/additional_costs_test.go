package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewAdditionalCostDefaultsToFirstOwnedVehicle(t *testing.T) {
	handler, db := newTestHandler(t)
	cookie := registerTestUser(t, handler, "cost-default@example.com")
	var zetaID, alfaID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Zeta',0,now(),now() FROM users WHERE email_address='cost-default@example.com' RETURNING id`).Scan(&zetaID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Alfa',2,now(),now() FROM users WHERE email_address='cost-default@example.com' RETURNING id`).Scan(&alfaID); err != nil {
		t.Fatal(err)
	}

	response := apiRequest(handler, http.MethodGet, "/api/additional_costs/new", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("new cost status = %d: %s", response.Code, response.Body)
	}
	var body struct {
		OccurredOn string `json:"occurred_on"`
		VehicleID  int64  `json:"vehicle_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.VehicleID != alfaID || body.OccurredOn != time.Now().In(time.Local).Format(time.DateOnly) {
		t.Fatalf("new cost defaults = %+v, want vehicle %d and today's date", body, alfaID)
	}
}

func TestAdditionalCostCRUDValidatesAndScopesVehicleOwnership(t *testing.T) {
	handler, db := newTestHandler(t)
	owner := registerTestUser(t, handler, "cost-owner@example.com")
	other := registerTestUser(t, handler, "cost-other@example.com")
	var vehicleID, foreignVehicleID int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Owner car',0,now(),now() FROM users WHERE email_address='cost-owner@example.com' RETURNING id`).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at)
		SELECT id,'Other car',0,now(),now() FROM users WHERE email_address='cost-other@example.com' RETURNING id`).Scan(&foreignVehicleID); err != nil {
		t.Fatal(err)
	}

	created := apiRequest(handler, http.MethodPost, "/api/additional_costs", fmt.Sprintf(
		`{"vehicle_id":%d,"occurred_on":"2026-02-11","kind":"technical_inspection","comment":"  STK  ","cost":"85.50"}`,
		vehicleID,
	), owner)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}
	var body struct {
		AdditionalCost struct {
			ID        int64  `json:"id"`
			Kind      string `json:"kind"`
			KindLabel string `json:"kind_label"`
			Comment   string `json:"comment"`
			Cost      string `json:"cost"`
		} `json:"additional_cost"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cost := body.AdditionalCost
	if cost.ID < 1 || cost.Kind != "technical_inspection" || cost.KindLabel != "Technická kontrola" || cost.Comment != "STK" || cost.Cost != "85.50" {
		t.Fatalf("created additional cost = %+v", cost)
	}

	foreignCreate := apiRequest(handler, http.MethodPost, "/api/additional_costs", fmt.Sprintf(
		`{"vehicle_id":%d,"occurred_on":"2026-02-11","kind":"repair","cost":"1"}`,
		foreignVehicleID,
	), owner)
	if foreignCreate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign create status = %d, want 422", foreignCreate.Code)
	}
	for _, body := range []string{
		fmt.Sprintf(`{"vehicle_id":%d,"occurred_on":"2026-02-11","kind":"repair","cost":"0"}`, vehicleID),
		fmt.Sprintf(`{"vehicle_id":%d,"occurred_on":"2026-02-11","kind":"unknown","cost":"1"}`, vehicleID),
		fmt.Sprintf(`{"vehicle_id":%d,"occurred_on":"2026-02-11","kind":"repair","comment":%q,"cost":"1"}`, vehicleID, strings.Repeat("x", 256)),
	} {
		response := apiRequest(handler, http.MethodPost, "/api/additional_costs", body, owner)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid cost status = %d, want 422: %s", response.Code, response.Body)
		}
	}

	updated := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/additional_costs/%d", cost.ID),
		`{"kind":"other","comment":"  Updated  ","cost":"90"}`, owner)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"comment":"Updated"`) {
		t.Fatalf("update response = %d %s", updated.Code, updated.Body)
	}
	foreignRead := apiRequest(handler, http.MethodGet, fmt.Sprintf("/api/additional_costs/%d", cost.ID), "", other)
	if foreignRead.Code != http.StatusNotFound {
		t.Fatalf("foreign read status = %d, want 404", foreignRead.Code)
	}
	foreignUpdate := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/additional_costs/%d", cost.ID),
		fmt.Sprintf(`{"vehicle_id":%d}`, foreignVehicleID), owner)
	if foreignUpdate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign update status = %d, want 422", foreignUpdate.Code)
	}
	foreignDelete := apiRequest(handler, http.MethodDelete, fmt.Sprintf("/api/additional_costs/%d", cost.ID), "", other)
	if foreignDelete.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status = %d, want 404", foreignDelete.Code)
	}
	deleted := apiRequest(handler, http.MethodDelete, fmt.Sprintf("/api/additional_costs/%d", cost.ID), "", owner)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleted.Code)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM additional_costs WHERE id = $1", cost.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cost rows after delete = %d, want 0", count)
	}
}
