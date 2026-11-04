package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"spotreba/server/internal/auth"

	"github.com/jackc/pgx/v5/pgconn"
)

type passwordResetRequest struct {
	EmailAddress string `json:"email_address"`
}

type passwordUpdateRequest struct {
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

const passwordResetNotice = "Pokyny na obnovenie hesla boli odoslané, ak používateľ s touto e-mailovou adresou existuje."
const invalidPasswordReset = "Odkaz na obnovenie hesla je neplatný alebo vypršal."

func (s *Server) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input passwordResetRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	var id int64
	var emailAddress, digest string
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id, email_address, password_digest FROM users WHERE email_address = $1
	`, auth.NormalizeEmail(input.EmailAddress)).Scan(&id, &emailAddress, &digest)
	if err == nil && s.mailer != nil {
		token := s.resetSigner.Sign(id, digest, time.Now())
		scheme := "http"
		if s.secureCookie {
			scheme = "https"
		}
		host := strings.TrimSpace(s.appHost)
		if host == "" {
			host = r.Host
		}
		body := fmt.Sprintf("Heslo si môžete obnoviť na\n%s://%s/passwords/%s/edit\n\nTento odkaz má obmedzenú platnosť.\n", scheme, host, token)
		_ = s.mailer.Send(emailAddress, "Reset your password", body)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, passwordResetNotice)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"notice": passwordResetNotice})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input passwordUpdateRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	if input.Password == "" || len([]byte(input.Password)) > 72 || input.Password != input.PasswordConfirmation {
		writeError(w, http.StatusUnprocessableEntity, "Heslá sa nezhodujú.")
		return
	}
	token := r.PathValue("token")
	userID, ok := s.resetSigner.UserID(token)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "Odkaz na obnovenie hesla je neplatný alebo vypršal.")
		return
	}
	var currentDigest string
	if err := s.db.QueryRowContext(r.Context(), "SELECT password_digest FROM users WHERE id = $1", userID).Scan(&currentDigest); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Odkaz na obnovenie hesla je neplatný alebo vypršal.")
		return
	}
	verifiedID, valid := s.resetSigner.Verify(token, currentDigest, time.Now())
	if !valid || verifiedID != userID {
		writeError(w, http.StatusUnprocessableEntity, "Odkaz na obnovenie hesla je neplatný alebo vypršal.")
		return
	}
	newDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(r.Context(), `
		UPDATE users SET password_digest = $1, updated_at = now() at time zone 'utc'
		WHERE id = $2 AND password_digest = $3
	`, newDigest, userID, currentDigest)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
		return
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		writeError(w, http.StatusUnprocessableEntity, "Odkaz na obnovenie hesla je neplatný alebo vypršal.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Heslo sa nepodarilo obnoviť.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"notice": "Heslo bolo obnovené."})
}

func (s *Server) inspectPasswordReset(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	userID, ok := s.resetSigner.UserID(token)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, invalidPasswordReset)
		return
	}
	var digest string
	if err := s.db.QueryRowContext(r.Context(), "SELECT password_digest FROM users WHERE id = $1", userID).Scan(&digest); err != nil {
		writeError(w, http.StatusUnprocessableEntity, invalidPasswordReset)
		return
	}
	verifiedID, valid := s.resetSigner.Verify(token, digest, time.Now())
	if !valid || verifiedID != userID {
		writeError(w, http.StatusUnprocessableEntity, invalidPasswordReset)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}
