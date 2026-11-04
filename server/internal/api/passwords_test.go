package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedEmail struct {
	to, subject, body string
}

type recordingMailer struct {
	messages []capturedEmail
}

func (m *recordingMailer) Send(to, subject, body string) error {
	m.messages = append(m.messages, capturedEmail{to: to, subject: subject, body: body})
	return nil
}

func TestPasswordResetRequestDoesNotRevealAccountExistence(t *testing.T) {
	server, _ := newTestServer(t)
	handler := server.Handler()
	registerTestUser(t, handler, "reset-known@example.com")
	mailer := &recordingMailer{}
	server.mailer = mailer

	requestReset := func(email string) *httptest.ResponseRecorder {
		return apiRequest(handler, http.MethodPost, "/api/passwords", fmt.Sprintf(`{"email_address":%q}`, email), nil)
	}
	known := requestReset("RESET-KNOWN@example.com")
	unknown := requestReset("missing@example.com")
	if known.Code != http.StatusAccepted || unknown.Code != http.StatusAccepted || known.Body.String() != unknown.Body.String() {
		t.Fatalf("reset responses differ: known=%d %s unknown=%d %s", known.Code, known.Body, unknown.Code, unknown.Body)
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("reset email count = %d, want 1", len(mailer.messages))
	}
	message := mailer.messages[0]
	if message.to != "reset-known@example.com" || message.subject != "Reset your password" || !strings.Contains(message.body, "/passwords/") || !strings.Contains(message.body, "/edit") {
		t.Fatalf("reset message = %+v", message)
	}
	token := extractResetToken(t, message.body)
	valid := apiRequest(handler, http.MethodGet, "/api/passwords/"+token, "", nil)
	if valid.Code != http.StatusOK {
		t.Fatalf("valid reset token inspection = %d: %s", valid.Code, valid.Body)
	}
	invalid := apiRequest(handler, http.MethodGet, "/api/passwords/invalid-token", "", nil)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid reset token inspection = %d, want 422", invalid.Code)
	}
}

func TestPasswordResetChangesPasswordAndInvalidatesSessions(t *testing.T) {
	server, _ := newTestServer(t)
	handler := server.Handler()
	oldSession := registerTestUser(t, handler, "reset-change@example.com")
	mailer := &recordingMailer{}
	server.mailer = mailer
	request := apiRequest(handler, http.MethodPost, "/api/passwords", `{"email_address":"reset-change@example.com"}`, nil)
	if request.Code != http.StatusAccepted || len(mailer.messages) != 1 {
		t.Fatalf("reset request = %d %s; mail count=%d", request.Code, request.Body, len(mailer.messages))
	}
	pathStart := strings.Index(mailer.messages[0].body, "/passwords/")
	if pathStart < 0 {
		t.Fatalf("reset message has no link: %s", mailer.messages[0].body)
	}
	path := strings.Fields(mailer.messages[0].body[pathStart:])[0]
	path = strings.TrimRight(path, ".")
	if !strings.HasSuffix(path, "/edit") {
		t.Fatalf("reset link path = %q", path)
	}
	resetPath := "/api" + strings.TrimSuffix(strings.TrimPrefix(path, "http://localhost:8080"), "/edit")
	token := strings.TrimPrefix(resetPath, "/api/passwords/")
	if preview := apiRequest(handler, http.MethodGet, "/api/passwords/"+token, "", nil); preview.Code != http.StatusOK {
		t.Fatalf("reset token inspection = %d: %s", preview.Code, preview.Body)
	}

	mismatch := apiRequest(handler, http.MethodPost, resetPath,
		`{"password":"new-password-123","password_confirmation":"other-password"}`, nil)
	if mismatch.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatch status = %d, want 422", mismatch.Code)
	}

	updated := apiRequest(handler, http.MethodPost, resetPath,
		`{"password":"new-password-123","password_confirmation":"new-password-123"}`, nil)
	if updated.Code != http.StatusOK {
		t.Fatalf("password update status = %d: %s", updated.Code, updated.Body)
	}
	oldRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	oldRequest.AddCookie(oldSession)
	oldResponse := httptest.NewRecorder()
	handler.ServeHTTP(oldResponse, oldRequest)
	if oldResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old session status = %d, want 401", oldResponse.Code)
	}
	if preview := apiRequest(handler, http.MethodGet, "/api/passwords/"+token, "", nil); preview.Code != http.StatusUnprocessableEntity {
		t.Fatalf("used reset token inspection = %d, want 422", preview.Code)
	}
	newLogin := apiRequest(handler, http.MethodPost, "/api/session", `{"email_address":"reset-change@example.com","password":"new-password-123"}`, nil)
	if newLogin.Code != http.StatusOK {
		t.Fatalf("new password login status = %d: %s", newLogin.Code, newLogin.Body)
	}
}

func extractResetToken(t *testing.T, message string) string {
	t.Helper()
	start := strings.Index(message, "/passwords/")
	if start < 0 {
		t.Fatalf("reset message has no password URL: %s", message)
	}
	value := message[start+len("/passwords/"):]
	value = strings.TrimSuffix(value, "/edit\n\nTento odkaz má obmedzenú platnosť.\n")
	return value
}

func TestPasswordResetRejectsInvalidToken(t *testing.T) {
	handler, _ := newTestHandler(t)
	response := apiRequest(handler, http.MethodPost, "/api/passwords/invalid-token", `{"password":"new-password-123","password_confirmation":"new-password-123"}`, nil)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid reset token status = %d, want 422", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] == "" {
		t.Fatalf("invalid token response = %s", response.Body)
	}
}
