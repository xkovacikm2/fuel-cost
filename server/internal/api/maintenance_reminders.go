package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

type reminderInput struct {
	VehicleID    *int64       `json:"vehicle_id"`
	Kind         *string      `json:"kind"`
	IntervalDays *int         `json:"interval_days"`
	IntervalKM   *string      `json:"interval_km"`
	Active       *bool        `json:"active"`
	Leads        *[]leadInput `json:"leads"`
}

type leadInput struct {
	ID               *int64  `json:"id"`
	DaysBefore       *int    `json:"days_before"`
	KilometresBefore *string `json:"kilometres_before"`
}

type reminderLead struct {
	ID               *int64  `json:"id"`
	DaysBefore       *int    `json:"days_before"`
	KilometresBefore *string `json:"kilometres_before"`
}

type reminderRecord struct {
	ID                  int64          `json:"id"`
	VehicleID           int64          `json:"vehicle_id"`
	VehicleName         string         `json:"vehicle_name"`
	Kind                string         `json:"kind"`
	KindLabel           string         `json:"kind_label"`
	IntervalDays        *int           `json:"interval_days"`
	IntervalKM          *string        `json:"interval_km"`
	Active              bool           `json:"active"`
	Leads               []reminderLead `json:"leads"`
	BaselineDate        *string        `json:"baseline_date"`
	DaysRemaining       *int           `json:"days_remaining"`
	KilometresRemaining *string        `json:"kilometres_remaining"`
	Status              string         `json:"status"`
}

func (s *Server) newReminder(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicles, err := s.vehiclesForUser(r, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
		return
	}
	var vehicleID *int64
	if len(vehicles) > 0 {
		vehicleID = &vehicles[0].ID
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vehicle_id": vehicleID,
		"active":     true,
		"leads":      []reminderLead{{}},
		"vehicles":   vehicles,
		"kinds":      costKindOptions(),
	})
}

func (s *Server) listReminders(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT rr.id, rr.vehicle_id, v.name, rr.kind, rr.interval_days, rr.interval_km, rr.active
		FROM maintenance_reminder_rules rr JOIN vehicles v ON v.id = rr.vehicle_id
		WHERE v.user_id = $1 ORDER BY v.name, rr.kind
	`, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienky sa nepodarilo načítať.")
		return
	}
	defer func() { _ = rows.Close() }()
	reminders := make([]reminderRecord, 0)
	for rows.Next() {
		var reminder reminderRecord
		var kind int
		var intervalDays sql.NullInt64
		var intervalKM sql.NullString
		if err := rows.Scan(&reminder.ID, &reminder.VehicleID, &reminder.VehicleName, &kind, &intervalDays, &intervalKM, &reminder.Active); err != nil {
			writeError(w, http.StatusInternalServerError, "Pripomienky sa nepodarilo načítať.")
			return
		}
		reminder.Kind, reminder.KindLabel = costKindNames[kind], costKindLabel(kind)
		reminder.IntervalDays = nullInt(intervalDays)
		reminder.IntervalKM = fixedNullNumeric(intervalKM)
		reminder.Leads, err = s.reminderLeads(r.Context(), s.db, reminder.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Pripomienky sa nepodarilo načítať.")
			return
		}
		if err := s.populateReminderCycle(r.Context(), &reminder); err != nil {
			writeError(w, http.StatusInternalServerError, "Stav pripomienky sa nepodarilo vypočítať.")
			return
		}
		reminders = append(reminders, reminder)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienky sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminders": reminders})
}

func (s *Server) createReminder(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	var input reminderInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	reminder, kindValue, leads, err := s.validateReminderInput(input, reminderRecord{}, true)
	if err != nil {
		s.reminderInputError(w, err, false)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo vytvoriť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO maintenance_reminder_rules (vehicle_id, kind, interval_days, interval_km, active, created_at, updated_at)
		SELECT v.id, $2, $3, $4, $5, now() at time zone 'utc', now() at time zone 'utc'
		FROM vehicles v WHERE v.id = $1 AND v.user_id = $6 RETURNING id
	`, reminder.VehicleID, kindValue, reminder.IntervalDays, reminder.IntervalKM, reminder.Active, current.id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať pripomienky pre iného používateľa.")
		return
	}
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			writeError(w, http.StatusUnprocessableEntity, "Pripomienka tohto typu už existuje.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo vytvoriť.")
		return
	}
	for _, lead := range leads {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id, days_before, kilometres_before, created_at, updated_at)
			VALUES ($1, $2, $3, now() at time zone 'utc', now() at time zone 'utc')
		`, id, lead.DaysBefore, lead.KilometresBefore); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "Pokročilé pripomienky nie sú platné.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo vytvoriť.")
		return
	}
	reminder.ID, reminder.Kind, reminder.KindLabel = id, costKindNames[kindValue], costKindLabel(kindValue)
	reminder.Leads, err = s.reminderLeads(r.Context(), s.db, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienka sa vytvorila, ale nepodarilo sa ju načítať.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reminder": reminder})
}

func (s *Server) showReminder(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	reminder, err := s.findReminder(r.Context(), r.PathValue("id"), current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Pripomienka sa nenašla.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminder": reminder})
}

func (s *Server) updateReminder(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	reminder, err := s.findReminder(r.Context(), r.PathValue("id"), current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Pripomienka sa nenašla.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo načítať.")
		return
	}
	var input reminderInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	reminder, kindValue, leads, err := s.validateReminderInput(input, reminder, false)
	if err != nil {
		s.reminderInputError(w, err, true)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo upraviť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(r.Context(), `
		UPDATE maintenance_reminder_rules rr
		SET vehicle_id = $1, kind = $2, interval_days = $3, interval_km = $4, active = $5, updated_at = now() at time zone 'utc'
		FROM vehicles v WHERE rr.id = $6 AND v.id = $1 AND v.user_id = $7
	`, reminder.VehicleID, kindValue, reminder.IntervalDays, reminder.IntervalKM, reminder.Active, reminder.ID, current.id)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			writeError(w, http.StatusUnprocessableEntity, "Pripomienka tohto typu už existuje.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo upraviť.")
		return
	}
	updated, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo upraviť.")
		return
	}
	if updated == 0 {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete upravovať pripomienky iného používateľa.")
		return
	}
	if input.Leads != nil {
		if err := s.replaceReminderLeads(r.Context(), tx, reminder.ID, leads); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "Pokročilé pripomienky nie sú platné.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo upraviť.")
		return
	}
	reminder.Kind, reminder.KindLabel = costKindNames[kindValue], costKindLabel(kindValue)
	reminder.Leads, err = s.reminderLeads(r.Context(), s.db, reminder.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienka sa upravila, ale nepodarilo sa ju načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminder": reminder})
}

func (s *Server) deleteReminder(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "Pripomienka sa nenašla.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo odstrániť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM maintenance_notifications WHERE maintenance_reminder_rule_id = $1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo odstrániť.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id = $1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo odstrániť.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		DELETE FROM maintenance_reminder_rules rr USING vehicles v
		WHERE rr.id = $1 AND v.id = rr.vehicle_id AND v.user_id = $2
	`, id, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo odstrániť.")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "Pripomienka sa nenašla.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Pripomienku sa nepodarilo odstrániť.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) findReminder(ctx context.Context, rawID string, userID int64) (reminderRecord, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		return reminderRecord{}, errInvalidID
	}
	var reminder reminderRecord
	var kind int
	var intervalDays sql.NullInt64
	var intervalKM sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT rr.id, rr.vehicle_id, v.name, rr.kind, rr.interval_days, rr.interval_km, rr.active
		FROM maintenance_reminder_rules rr JOIN vehicles v ON v.id = rr.vehicle_id
		WHERE rr.id = $1 AND v.user_id = $2
	`, id, userID).Scan(&reminder.ID, &reminder.VehicleID, &reminder.VehicleName, &kind, &intervalDays, &intervalKM, &reminder.Active)
	if err != nil {
		return reminderRecord{}, err
	}
	reminder.Kind, reminder.KindLabel = costKindNames[kind], costKindLabel(kind)
	reminder.IntervalDays = nullInt(intervalDays)
	reminder.IntervalKM = fixedNullNumeric(intervalKM)
	reminder.Leads, err = s.reminderLeads(ctx, s.db, id)
	if err != nil {
		return reminderRecord{}, err
	}
	return reminder, s.populateReminderCycle(ctx, &reminder)
}

func (s *Server) validateReminderInput(input reminderInput, current reminderRecord, required bool) (reminderRecord, int, []reminderLead, error) {
	reminder := current
	if input.VehicleID != nil {
		if *input.VehicleID < 1 {
			return reminder, 0, nil, errInvalidReminder
		}
		reminder.VehicleID = *input.VehicleID
	} else if required {
		return reminder, 0, nil, errInvalidReminder
	}
	kind := current.Kind
	if input.Kind != nil {
		kind = *input.Kind
	} else if required {
		return reminder, 0, nil, errInvalidReminder
	}
	kindValue, ok := costKinds[kind]
	if !ok {
		return reminder, 0, nil, errInvalidReminder
	}
	if input.IntervalDays != nil {
		reminder.IntervalDays = input.IntervalDays
	} else if required {
		reminder.IntervalDays = nil
	}
	if input.IntervalKM != nil {
		value, err := positiveCost(*input.IntervalKM)
		if err != nil {
			return reminder, 0, nil, errInvalidReminder
		}
		fixed := value.StringFixed(2)
		reminder.IntervalKM = &fixed
	} else if required {
		reminder.IntervalKM = nil
	}
	if input.Active != nil {
		reminder.Active = *input.Active
	} else if required {
		reminder.Active = true
	}
	if reminder.IntervalDays == nil && reminder.IntervalKM == nil {
		return reminder, 0, nil, errInvalidReminder
	}
	if reminder.IntervalDays != nil && *reminder.IntervalDays <= 0 {
		return reminder, 0, nil, errInvalidReminder
	}
	if reminder.IntervalKM != nil {
		if _, err := positiveCost(*reminder.IntervalKM); err != nil {
			return reminder, 0, nil, errInvalidReminder
		}
	}
	leads := make([]reminderLead, 0)
	if input.Leads == nil {
		leads = append(leads, current.Leads...)
	} else {
		seen := make(map[string]bool)
		for _, lead := range *input.Leads {
			if (lead.DaysBefore == nil) == (lead.KilometresBefore == nil) {
				return reminder, 0, nil, errInvalidReminder
			}
			if lead.DaysBefore != nil {
				if *lead.DaysBefore <= 0 || reminder.IntervalDays == nil {
					return reminder, 0, nil, errInvalidReminder
				}
				key := "days:" + strconv.Itoa(*lead.DaysBefore)
				if seen[key] {
					return reminder, 0, nil, errInvalidReminder
				}
				seen[key] = true
			}
			if lead.KilometresBefore != nil {
				value, err := positiveCost(*lead.KilometresBefore)
				if err != nil || reminder.IntervalKM == nil {
					return reminder, 0, nil, errInvalidReminder
				}
				fixed := value.StringFixed(2)
				lead.KilometresBefore = &fixed
				if seen["km:"+fixed] {
					return reminder, 0, nil, errInvalidReminder
				}
				seen["km:"+fixed] = true
			}
			leads = append(leads, reminderLead(lead))
		}
	}
	return reminder, kindValue, leads, nil
}

func (s *Server) reminderLeads(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, ruleID int64) ([]reminderLead, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, days_before, kilometres_before FROM maintenance_reminder_leads
		WHERE maintenance_reminder_rule_id = $1 ORDER BY days_before DESC NULLS LAST, kilometres_before DESC NULLS LAST, id
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	leads := make([]reminderLead, 0)
	for rows.Next() {
		var lead reminderLead
		var id int64
		var days sql.NullInt64
		var kilometres sql.NullString
		if err := rows.Scan(&id, &days, &kilometres); err != nil {
			return nil, err
		}
		lead.ID = &id
		lead.DaysBefore = nullInt(days)
		lead.KilometresBefore = fixedNullNumeric(kilometres)
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func (s *Server) replaceReminderLeads(ctx context.Context, tx *sql.Tx, ruleID int64, leads []reminderLead) error {
	existing, err := s.reminderLeads(ctx, tx, ruleID)
	if err != nil {
		return err
	}
	keep := make(map[int64]bool)
	for _, lead := range leads {
		if lead.ID != nil {
			owned := false
			for _, old := range existing {
				if old.ID != nil && *old.ID == *lead.ID {
					owned = true
				}
			}
			if !owned {
				return errInvalidReminder
			}
			keep[*lead.ID] = true
			if _, err := tx.ExecContext(ctx, `
				UPDATE maintenance_reminder_leads SET days_before = $1, kilometres_before = $2, updated_at = now() at time zone 'utc'
				WHERE id = $3 AND maintenance_reminder_rule_id = $4
			`, lead.DaysBefore, lead.KilometresBefore, *lead.ID, ruleID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, `
			INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id, days_before, kilometres_before, created_at, updated_at)
			VALUES ($1, $2, $3, now() at time zone 'utc', now() at time zone 'utc')
		`, ruleID, lead.DaysBefore, lead.KilometresBefore); err != nil {
			return err
		}
	}
	for _, old := range existing {
		if old.ID != nil && !keep[*old.ID] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_notifications WHERE maintenance_reminder_lead_id = $1`, *old.ID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM maintenance_reminder_leads WHERE id = $1", *old.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) populateReminderCycle(ctx context.Context, reminder *reminderRecord) error {
	reminder.Status = "Čaká na prvý záznam"
	if !reminder.Active {
		reminder.Status = "Neaktívna"
	}
	var costID int64
	var baselineValue time.Time
	kindValue, ok := costKinds[reminder.Kind]
	if !ok {
		return errInvalidReminder
	}
	today := time.Now().In(s.appTZ).Format(time.DateOnly)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, occurred_on FROM additional_costs
		WHERE vehicle_id = $1 AND kind = $2 AND occurred_on <= $3
		ORDER BY occurred_on DESC, created_at DESC LIMIT 1
	`, reminder.VehicleID, kindValue, today).Scan(&costID, &baselineValue)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	baseline := baselineValue.Format(time.DateOnly)
	reminder.BaselineDate = &baseline
	var travelled decimal.Decimal
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(distance_km), 0) FROM refuelings
		WHERE vehicle_id = $1 AND refueled_on > $2 AND refueled_on <= $3
	`, reminder.VehicleID, baseline, today).Scan(&travelled); err != nil {
		return err
	}
	baselineDate, err := time.Parse(time.DateOnly, baseline)
	if err != nil {
		return err
	}
	todayDate, err := time.Parse(time.DateOnly, today)
	if err != nil {
		return err
	}
	if reminder.IntervalDays != nil {
		remaining := *reminder.IntervalDays - int(todayDate.Sub(baselineDate).Hours()/24)
		reminder.DaysRemaining = &remaining
	}
	if reminder.IntervalKM != nil {
		interval, err := decimal.NewFromString(*reminder.IntervalKM)
		if err != nil {
			return err
		}
		remaining := interval.Sub(travelled).StringFixed(2)
		reminder.KilometresRemaining = &remaining
	}
	if !reminder.Active {
		return nil
	}
	if (reminder.DaysRemaining != nil && *reminder.DaysRemaining <= 0) ||
		(reminder.KilometresRemaining != nil && decimal.RequireFromString(*reminder.KilometresRemaining).IsNegative()) ||
		(reminder.KilometresRemaining != nil && decimal.RequireFromString(*reminder.KilometresRemaining).IsZero()) {
		reminder.Status = "Termín je potrebný"
		return nil
	}
	parts := make([]string, 0, 2)
	if reminder.DaysRemaining != nil {
		parts = append(parts, strconv.Itoa(*reminder.DaysRemaining)+" dní")
	}
	if reminder.KilometresRemaining != nil {
		parts = append(parts, stripDecimalZeros(*reminder.KilometresRemaining)+" km")
	}
	reminder.Status = strings.Join(parts, " / ")
	return nil
}

func (s *Server) reminderInputError(w http.ResponseWriter, err error, update bool) {
	if errors.Is(err, errForeignVehicle) {
		if update {
			writeError(w, http.StatusUnprocessableEntity, "Nemôžete upravovať pripomienky iného používateľa.")
		} else {
			writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať pripomienky pre iného používateľa.")
		}
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "Pripomienka údržby nie je platná.")
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}

func fixedNullNumeric(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	fixed := decimal.RequireFromString(value.String).StringFixed(2)
	return &fixed
}

func stripDecimalZeros(value string) string {
	return strings.TrimRight(strings.TrimRight(value, "0"), ".")
}

var errInvalidReminder = errors.New("invalid reminder")
