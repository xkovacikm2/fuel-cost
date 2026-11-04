package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func registerTestUser(t *testing.T, handler http.Handler, email string) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/registrations", strings.NewReader(fmt.Sprintf(
		`{"email_address":%q,"password":"password-123","password_confirmation":"password-123"}`,
		email,
	)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("test registration status = %d: %s", response.Code, response.Body)
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	t.Fatal("test registration did not return a session cookie")
	return nil
}

func apiRequest(handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestVehiclesCreateListUpdateAndDeleteAreUserScoped(t *testing.T) {
	handler, _ := newTestHandler(t)
	mine := registerTestUser(t, handler, "vehicle-owner@example.com")
	other := registerTestUser(t, handler, "vehicle-other@example.com")

	created := apiRequest(handler, http.MethodPost, "/api/vehicles", `{"name":"  Octavia  ","fuel_type":"diesel"}`, mine)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}
	var payload struct {
		Vehicle struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FuelType string `json:"fuel_type"`
			Unit     string `json:"unit"`
		} `json:"vehicle"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	vehicle := payload.Vehicle
	if vehicle.ID < 1 || vehicle.Name != "  Octavia  " || vehicle.FuelType != "diesel" || vehicle.Unit != "L" {
		t.Fatalf("created vehicle = %+v", vehicle)
	}

	list := apiRequest(handler, http.MethodGet, "/api/vehicles", "", mine)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "Octavia") {
		t.Fatalf("owner list = %d %s", list.Code, list.Body)
	}
	otherList := apiRequest(handler, http.MethodGet, "/api/vehicles", "", other)
	if otherList.Code != http.StatusOK || strings.Contains(otherList.Body.String(), "Octavia") {
		t.Fatalf("other user list = %d %s", otherList.Code, otherList.Body)
	}

	foreign := apiRequest(handler, http.MethodGet, fmt.Sprintf("/api/vehicles/%d", vehicle.ID), "", other)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign read status = %d, want 404", foreign.Code)
	}
	updated := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/vehicles/%d", vehicle.ID), `{"name":"Zoe","fuel_type":"electricity"}`, mine)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"unit":"kWh"`) {
		t.Fatalf("update response = %d %s", updated.Code, updated.Body)
	}
	foreignUpdate := apiRequest(handler, http.MethodPatch, fmt.Sprintf("/api/vehicles/%d", vehicle.ID), `{"name":"Stolen"}`, other)
	if foreignUpdate.Code != http.StatusNotFound {
		t.Fatalf("foreign update status = %d, want 404", foreignUpdate.Code)
	}

	foreignDelete := apiRequest(handler, http.MethodDelete, fmt.Sprintf("/api/vehicles/%d", vehicle.ID), "", other)
	if foreignDelete.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status = %d, want 404", foreignDelete.Code)
	}
	deleted := apiRequest(handler, http.MethodDelete, fmt.Sprintf("/api/vehicles/%d", vehicle.ID), "", mine)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("owner delete status = %d, want 204: %s", deleted.Code, deleted.Body)
	}
	remaining := apiRequest(handler, http.MethodGet, "/api/vehicles", "", mine)
	if remaining.Code != http.StatusOK || strings.Contains(remaining.Body.String(), "Zoe") {
		t.Fatalf("list after delete = %d %s", remaining.Code, remaining.Body)
	}
}

func TestVehiclesRejectInvalidFieldsAndRequireAuthentication(t *testing.T) {
	handler, _ := newTestHandler(t)
	unauthenticated := apiRequest(handler, http.MethodPost, "/api/vehicles", `{"name":"Car","fuel_type":"petrol"}`, nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d, want 401", unauthenticated.Code)
	}

	cookie := registerTestUser(t, handler, "vehicle-validation@example.com")
	for _, body := range []string{
		`{"name":"  ","fuel_type":"petrol"}`,
		`{"name":"Car","fuel_type":"hydrogen"}`,
		`{"name":"","fuel_type":"petrol"}`,
	} {
		response := apiRequest(handler, http.MethodPost, "/api/vehicles", body, cookie)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("create with %s returned %d, want 422", body, response.Code)
		}
	}
}
