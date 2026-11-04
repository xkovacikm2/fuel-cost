package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"spotreba/server/internal/config"
	database "spotreba/server/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func newTestHandler(t *testing.T) (http.Handler, *sql.DB) {
	server, db := newTestServer(t)
	return server.Handler(), db
}

func newTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	url := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MIGRATION_TEST_DATABASE_URL to run PostgreSQL API tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pgConfig, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, pgConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })

	schema := fmt.Sprintf("api_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	})

	pgConfig.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*pgConfig)
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	server, err := New(db, config.Config{
		AppHost:    "localhost:8080",
		SecretKey:  []byte("test-session-signing-key-that-is-at-least-32-bytes"),
		MailerFrom: "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return server, db
}

func TestRegistrationCreatesUserAndSignedSession(t *testing.T) {
	handler, db := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/registrations", strings.NewReader(`{
		"email_address":"  DRIVER@Example.COM  ",
		"password":"existing-bcrypt-password",
		"password_confirmation":"existing-bcrypt-password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, want 201; body=%s", response.Code, response.Body)
	}
	var body struct {
		User struct {
			ID           int64  `json:"id"`
			EmailAddress string `json:"email_address"`
		} `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.User.EmailAddress != "driver@example.com" || body.User.ID < 1 {
		t.Fatalf("registration user = %+v", body.User)
	}

	cookie := response.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != "session_id" {
		t.Fatalf("registration cookies = %+v, want signed session_id", cookie)
	}
	if !cookie[0].HttpOnly || cookie[0].SameSite != http.SameSiteLaxMode || cookie[0].MaxAge <= 0 {
		t.Fatalf("session cookie options = %+v", cookie[0])
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meRequest.AddCookie(cookie[0])
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK || !strings.Contains(meResponse.Body.String(), `"driver@example.com"`) {
		t.Fatalf("authenticated me response = %d %s", meResponse.Code, meResponse.Body)
	}

	var sessions int
	if err := db.QueryRow("SELECT count(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Fatalf("session rows = %d, want 1", sessions)
	}
}

func TestLoginVerifiesExistingBcryptHashAndLogoutEndsSession(t *testing.T) {
	handler, db := newTestHandler(t)
	const railsDigest = "$2a$04$IWYUNT5/xfvoyUuPfEiXRu2shBJFdm4r/b30v9wXJWT2UIf1OpdG6"
	_, err := db.Exec(`INSERT INTO users (email_address, password_digest, created_at, updated_at)
		VALUES ('legacy@example.com', $1, now(), now())`, railsDigest)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{
		"email_address":"LEGACY@example.com",
		"password":"existing-bcrypt-password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", response.Code, response.Body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %+v, want session cookie", cookies)
	}

	logout := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	logout.AddCookie(cookies[0])
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutResponse.Code)
	}
	cleared := logoutResponse.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("logout cookie = %+v, want expired cookie", cleared)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.AddCookie(cookies[0])
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, want 401", meResponse.Code)
	}
}

func TestRegistrationRejectsMismatchedPasswordsAndDuplicateEmail(t *testing.T) {
	handler, db := newTestHandler(t)
	post := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/registrations", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	mismatch := post(`{"email_address":"match@example.com","password":"one-password","password_confirmation":"other-password"}`)
	if mismatch.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatch status = %d, want 422", mismatch.Code)
	}
	if strings.Contains(mismatch.Body.String(), "password_digest") {
		t.Fatalf("mismatch response leaked sensitive fields: %s", mismatch.Body)
	}

	first := post(`{"email_address":"duplicate@example.com","password":"same-password","password_confirmation":"same-password"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first registration status = %d, want 201", first.Code)
	}
	second := post(`{"email_address":" DUPLICATE@example.com ","password":"same-password","password_confirmation":"same-password"}`)
	if second.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate registration status = %d, want 422", second.Code)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate registration created %d users, want 1", count)
	}
}

func TestRegistrationRollsBackWhenSessionCreationFails(t *testing.T) {
	handler, db := newTestHandler(t)
	_, err := db.Exec(`
		CREATE FUNCTION reject_session_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'session insert rejected';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_session_insert BEFORE INSERT ON sessions
		FOR EACH ROW EXECUTE FUNCTION reject_session_insert();
	`)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/registrations", strings.NewReader(`{
		"email_address":"atomic@example.com",
		"password":"same-password",
		"password_confirmation":"same-password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("registration status = %d, want 500", response.Code)
	}
	var users int
	if err := db.QueryRow("SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Fatalf("failed registration persisted %d users, want 0", users)
	}
}

func TestCrossOriginRegistrationIsRejected(t *testing.T) {
	handler, db := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "http://spotreba.example/api/registrations", strings.NewReader(`{
		"email_address":"attacker@example.com",
		"password":"password",
		"password_confirmation":"password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code < 400 {
		t.Fatalf("cross-origin status = %d, want rejection", response.Code)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cross-origin request created %d users, want 0", count)
	}
}

func TestLoginRateLimitReturnsRailsMessageOnEleventhAttempt(t *testing.T) {
	handler, _ := newTestHandler(t)
	for attempt := 1; attempt <= 11; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{
			"email_address":"missing@example.com",
			"password":"wrong-password"
		}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt <= 10 && response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", attempt, response.Code)
		}
		if attempt == 11 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt 11 status = %d, want 429", response.Code)
			}
			if !strings.Contains(response.Body.String(), "Skúste to neskôr.") {
				t.Fatalf("attempt 11 response = %s", response.Body)
			}
		}
	}
}
