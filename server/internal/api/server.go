package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"time"

	"spotreba/server/internal/auth"
	"spotreba/server/internal/config"
	"spotreba/server/internal/mail"
	"spotreba/server/internal/webui"

	"github.com/jackc/pgx/v5/pgconn"
)

const sessionCookieName = "session_id"

type Server struct {
	db             *sql.DB
	signer         *auth.SessionSigner
	secureCookie   bool
	loginLimiter   *rateLimiter
	resetLimiter   *rateLimiter
	appTZ          *time.Location
	appHost        string
	resetSigner    *auth.PasswordResetSigner
	mailer         mail.Sender
	mux            *http.ServeMux
	trustedProxies []netip.Prefix
}

type user struct {
	id           int64
	emailAddress string
}

type contextKey uint8

const userContextKey contextKey = 1

type registrationInput struct {
	EmailAddress         string `json:"email_address"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type loginInput struct {
	EmailAddress string `json:"email_address"`
	Password     string `json:"password"`
}

func New(database *sql.DB, cfg config.Config) (*Server, error) {
	signer, err := auth.NewSessionSigner(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	resetSigner, err := auth.NewPasswordResetSigner(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	appTZ := cfg.AppTZ
	if appTZ == nil {
		appTZ, err = time.LoadLocation("Europe/Bratislava")
		if err != nil {
			return nil, err
		}
	}
	s := &Server{
		db:             database,
		signer:         signer,
		resetSigner:    resetSigner,
		secureCookie:   cfg.AssumeSSL,
		loginLimiter:   newRateLimiter(10, 3*time.Minute),
		resetLimiter:   newRateLimiter(10, 3*time.Minute),
		appTZ:          appTZ,
		appHost:        cfg.AppHost,
		mux:            http.NewServeMux(),
		trustedProxies: cfg.TrustedProxies,
	}
	if cfg.SMTPAddress != "" {
		s.mailer = &mail.SMTP{Address: cfg.SMTPAddress, Port: cfg.SMTPPort, Username: cfg.SMTPUser, Password: cfg.SMTPPass, From: cfg.MailerFrom}
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return http.NewCrossOriginProtection().Handler(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /up", s.health)
	s.mux.HandleFunc("POST /api/registrations", s.register)
	s.mux.Handle("POST /api/session", s.loginLimiter.middleware(http.HandlerFunc(s.createSession), s.trustedProxies...))
	s.mux.Handle("GET /api/me", s.requireAuthentication(http.HandlerFunc(s.me)))
	s.mux.Handle("DELETE /api/session", s.requireAuthentication(http.HandlerFunc(s.destroySession)))
	s.mux.Handle("GET /api/vehicles", s.requireAuthentication(http.HandlerFunc(s.listVehicles)))
	s.mux.Handle("POST /api/vehicles", s.requireAuthentication(http.HandlerFunc(s.createVehicle)))
	s.mux.Handle("GET /api/vehicles/{id}", s.requireAuthentication(http.HandlerFunc(s.showVehicle)))
	s.mux.Handle("PATCH /api/vehicles/{id}", s.requireAuthentication(http.HandlerFunc(s.updateVehicle)))
	s.mux.Handle("DELETE /api/vehicles/{id}", s.requireAuthentication(http.HandlerFunc(s.deleteVehicle)))
	s.mux.Handle("GET /api/refuelings/new", s.requireAuthentication(http.HandlerFunc(s.newRefueling)))
	s.mux.Handle("POST /api/refuelings", s.requireAuthentication(http.HandlerFunc(s.createRefueling)))
	s.mux.Handle("GET /api/refuelings/{id}", s.requireAuthentication(http.HandlerFunc(s.showRefueling)))
	s.mux.Handle("PATCH /api/refuelings/{id}", s.requireAuthentication(http.HandlerFunc(s.updateRefueling)))
	s.mux.Handle("DELETE /api/refuelings/{id}", s.requireAuthentication(http.HandlerFunc(s.deleteRefueling)))
	s.mux.Handle("GET /api/additional_costs/new", s.requireAuthentication(http.HandlerFunc(s.newAdditionalCost)))
	s.mux.Handle("POST /api/additional_costs", s.requireAuthentication(http.HandlerFunc(s.createAdditionalCost)))
	s.mux.Handle("GET /api/additional_costs/{id}", s.requireAuthentication(http.HandlerFunc(s.showAdditionalCost)))
	s.mux.Handle("PATCH /api/additional_costs/{id}", s.requireAuthentication(http.HandlerFunc(s.updateAdditionalCost)))
	s.mux.Handle("DELETE /api/additional_costs/{id}", s.requireAuthentication(http.HandlerFunc(s.deleteAdditionalCost)))
	s.mux.Handle("GET /api/maintenance_reminders", s.requireAuthentication(http.HandlerFunc(s.listReminders)))
	s.mux.Handle("GET /api/maintenance_reminders/new", s.requireAuthentication(http.HandlerFunc(s.newReminder)))
	s.mux.Handle("POST /api/maintenance_reminders", s.requireAuthentication(http.HandlerFunc(s.createReminder)))
	s.mux.Handle("GET /api/maintenance_reminders/{id}", s.requireAuthentication(http.HandlerFunc(s.showReminder)))
	s.mux.Handle("PATCH /api/maintenance_reminders/{id}", s.requireAuthentication(http.HandlerFunc(s.updateReminder)))
	s.mux.Handle("DELETE /api/maintenance_reminders/{id}", s.requireAuthentication(http.HandlerFunc(s.deleteReminder)))
	s.mux.Handle("GET /api/dashboard", s.requireAuthentication(http.HandlerFunc(s.dashboard)))
	s.mux.Handle("GET /api/user_data/export", s.requireAuthentication(http.HandlerFunc(s.exportUserData)))
	s.mux.Handle("POST /api/user_data/import", s.requireAuthentication(http.HandlerFunc(s.importUserData)))
	s.mux.Handle("POST /api/passwords", s.resetLimiter.middleware(http.HandlerFunc(s.requestPasswordReset), s.trustedProxies...))
	s.mux.HandleFunc("GET /api/passwords/{token}", s.inspectPasswordReset)
	s.mux.HandleFunc("POST /api/passwords/{token}", s.resetPassword)
	s.mux.Handle("/", webui.Handler())
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input registrationInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	input.EmailAddress = auth.NormalizeEmail(input.EmailAddress)
	if input.EmailAddress == "" || input.Password == "" || len([]byte(input.Password)) > 72 || input.Password != input.PasswordConfirmation {
		writeError(w, http.StatusUnprocessableEntity, "E-mail alebo heslo nie sú platné.")
		return
	}

	digest, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Účet sa nepodarilo vytvoriť.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Účet sa nepodarilo vytvoriť.")
		return
	}
	var created user
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO users (email_address, password_digest, created_at, updated_at)
		VALUES ($1, $2, now() at time zone 'utc', now() at time zone 'utc')
		RETURNING id, email_address
	`, input.EmailAddress, digest).Scan(&created.id, &created.emailAddress)
	if err != nil {
		_ = tx.Rollback()
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			writeError(w, http.StatusUnprocessableEntity, "Účet s touto e-mailovou adresou už existuje.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Účet sa nepodarilo vytvoriť.")
		return
	}
	sessionID, err := s.insertSession(r.Context(), tx, r, created)
	if err != nil {
		_ = tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Prihlásenie sa nepodarilo.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Účet sa nepodarilo vytvoriť.")
		return
	}
	s.setSessionCookie(w, sessionID)
	writeJSON(w, http.StatusCreated, map[string]any{"user": map[string]any{
		"id": created.id, "email_address": created.emailAddress,
	}, "notice": "Vitajte! Váš účet bol vytvorený."})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var input loginInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	var current user
	var digest string
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id, email_address, password_digest FROM users WHERE email_address = $1
	`, auth.NormalizeEmail(input.EmailAddress)).Scan(&current.id, &current.emailAddress, &digest)
	if err != nil || auth.VerifyPassword(input.Password, digest) != nil {
		writeError(w, http.StatusUnauthorized, "Skúste inú e-mailovú adresu alebo heslo.")
		return
	}
	sessionID, err := s.insertSession(r.Context(), s.db, r, current)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Prihlásenie sa nepodarilo.")
		return
	}
	s.setSessionCookie(w, sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{
		"id": current.id, "email_address": current.emailAddress,
	}})
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Server) insertSession(ctx context.Context, database queryRower, r *http.Request, current user) (int64, error) {
	remoteIP := clientAddress(r, s.trustedProxies)
	var sessionID int64
	err := database.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, ip_address, user_agent, created_at, updated_at)
		VALUES ($1, $2, $3, now() at time zone 'utc', now() at time zone 'utc')
		RETURNING id
	`, current.id, remoteIP, r.UserAgent()).Scan(&sessionID)
	if err != nil {
		return 0, err
	}
	return sessionID, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sessionID int64) {
	now := time.Now()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signer.Sign(sessionID),
		Path:     "/",
		Expires:  now.AddDate(20, 0, 0),
		MaxAge:   20 * 365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Prihláste sa.")
			return
		}
		sessionID, ok := s.signer.Verify(cookie.Value)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Prihláste sa.")
			return
		}
		var current user
		err = s.db.QueryRowContext(r.Context(), `
			SELECT u.id, u.email_address
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.id = $1
		`, sessionID).Scan(&current.id, &current.emailAddress)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Prihláste sa.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, current)))
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{
		"id": current.id, "email_address": current.emailAddress,
	}})
}

func (s *Server) destroySession(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	cookie, _ := r.Cookie(sessionCookieName)
	sessionID, ok := s.signer.Verify(cookie.Value)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Prihláste sa.")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE id = $1 AND user_id = $2", sessionID, current.id); err != nil {
		writeError(w, http.StatusInternalServerError, "Odhlásenie sa nepodarilo.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
