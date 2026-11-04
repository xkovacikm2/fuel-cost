package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

var costKinds = map[string]int{
	"oil_change":           0,
	"fuel_filter_change":   1,
	"technical_inspection": 2,
	"emissions_inspection": 3,
	"tire_change":          4,
	"repair":               5,
	"other":                6,
}

var costKindNames = []string{
	"oil_change", "fuel_filter_change", "technical_inspection", "emissions_inspection", "tire_change", "repair", "other",
}

type additionalCostInput struct {
	VehicleID  *int64  `json:"vehicle_id"`
	OccurredOn *string `json:"occurred_on"`
	Kind       *string `json:"kind"`
	Comment    *string `json:"comment"`
	Cost       *string `json:"cost"`
}

type additionalCostRecord struct {
	ID          int64   `json:"id"`
	VehicleID   int64   `json:"vehicle_id"`
	VehicleName string  `json:"vehicle_name"`
	OccurredOn  string  `json:"occurred_on"`
	Kind        string  `json:"kind"`
	KindLabel   string  `json:"kind_label"`
	Comment     *string `json:"comment"`
	Cost        string  `json:"cost"`
}

func (s *Server) newAdditionalCost(w http.ResponseWriter, r *http.Request) {
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
		"occurred_on": time.Now().In(s.appTZ).Format(time.DateOnly),
		"vehicle_id":  vehicleID,
		"vehicles":    vehicles,
		"kinds":       costKindOptions(),
	})
}

func (s *Server) createAdditionalCost(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	var input additionalCostInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	cost, err := s.applyAdditionalCostInput(r, current.id, additionalCostRecord{}, input, true)
	if err != nil {
		s.additionalCostError(w, err, false)
		return
	}
	kind := costKinds[cost.Kind]
	var id int64
	err = s.db.QueryRowContext(r.Context(), `
		INSERT INTO additional_costs (vehicle_id, occurred_on, kind, cost, comment, created_at, updated_at)
		SELECT v.id, $2, $3, $4, $5, now() at time zone 'utc', now() at time zone 'utc'
		FROM vehicles v WHERE v.id = $1 AND v.user_id = $6
		RETURNING id
	`, cost.VehicleID, cost.OccurredOn, kind, cost.Cost, cost.Comment, current.id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať náklady pre iného používateľa.")
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Dodatočný náklad sa nepodarilo uložiť.")
		return
	}
	cost.ID = id
	writeJSON(w, http.StatusCreated, map[string]any{"additional_cost": cost})
}

func (s *Server) showAdditionalCost(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	cost, err := s.findAdditionalCost(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Dodatočný náklad sa nenašiel.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"additional_cost": cost})
}

func (s *Server) updateAdditionalCost(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	cost, err := s.findAdditionalCost(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Dodatočný náklad sa nenašiel.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo načítať.")
		return
	}
	var input additionalCostInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	cost, err = s.applyAdditionalCostInput(r, current.id, cost, input, false)
	if err != nil {
		s.additionalCostError(w, err, true)
		return
	}
	_, err = s.db.ExecContext(r.Context(), `
		UPDATE additional_costs c SET vehicle_id = $1, occurred_on = $2, kind = $3, cost = $4, comment = $5,
			updated_at = now() at time zone 'utc'
		WHERE c.id = $6 AND c.vehicle_id IN (SELECT id FROM vehicles WHERE user_id = $7)
		  AND EXISTS (SELECT 1 FROM vehicles v WHERE v.id = $1 AND v.user_id = $7)
	`, cost.VehicleID, cost.OccurredOn, costKinds[cost.Kind], cost.Cost, cost.Comment, cost.ID, current.id)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Dodatočný náklad sa nepodarilo upraviť.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"additional_cost": cost})
}

func (s *Server) deleteAdditionalCost(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "Dodatočný náklad sa nenašiel.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo odstrániť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM maintenance_notifications WHERE additional_cost_id = $1
	`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo odstrániť.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		DELETE FROM additional_costs c USING vehicles v
		WHERE c.id = $1 AND v.id = c.vehicle_id AND v.user_id = $2
	`, id, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo odstrániť.")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "Dodatočný náklad sa nenašiel.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Dodatočný náklad sa nepodarilo odstrániť.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) applyAdditionalCostInput(r *http.Request, userID int64, current additionalCostRecord, input additionalCostInput, required bool) (additionalCostRecord, error) {
	if input.VehicleID != nil {
		if *input.VehicleID < 1 {
			return current, errInvalidAdditionalCost
		}
		current.VehicleID = *input.VehicleID
	} else if required {
		return current, errInvalidAdditionalCost
	}
	if input.OccurredOn != nil {
		if _, err := time.Parse(time.DateOnly, *input.OccurredOn); err != nil {
			return current, errInvalidAdditionalCost
		}
		current.OccurredOn = *input.OccurredOn
	} else if required {
		return current, errInvalidAdditionalCost
	}
	if input.Kind != nil {
		if _, ok := costKinds[*input.Kind]; !ok {
			return current, errInvalidAdditionalCost
		}
		current.Kind = *input.Kind
	} else if required {
		return current, errInvalidAdditionalCost
	}
	if input.Cost != nil {
		current.Cost = *input.Cost
	} else if required {
		return current, errInvalidAdditionalCost
	}
	amount, err := positiveCost(current.Cost)
	if err != nil {
		return current, errInvalidAdditionalCost
	}
	current.Cost = amount.StringFixed(2)
	if input.Comment != nil {
		comment := strings.TrimSpace(*input.Comment)
		if utf8.RuneCountInString(comment) > 255 {
			return current, errInvalidAdditionalCost
		}
		if comment == "" {
			current.Comment = nil
		} else {
			current.Comment = &comment
		}
	}
	err = s.db.QueryRowContext(r.Context(), "SELECT name FROM vehicles WHERE id = $1 AND user_id = $2", current.VehicleID, userID).Scan(&current.VehicleName)
	if errors.Is(err, sql.ErrNoRows) {
		return current, errForeignVehicle
	}
	if err != nil {
		return current, err
	}
	current.KindLabel = costKindLabel(costKinds[current.Kind])
	return current, nil
}

func (s *Server) findAdditionalCost(r *http.Request, userID int64) (additionalCostRecord, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return additionalCostRecord{}, errInvalidID
	}
	var record additionalCostRecord
	var kind int
	var amount decimal.Decimal
	err = s.db.QueryRowContext(r.Context(), `
		SELECT c.id, c.vehicle_id, v.name, c.occurred_on, c.kind, c.comment, c.cost
		FROM additional_costs c JOIN vehicles v ON v.id = c.vehicle_id
		WHERE c.id = $1 AND v.user_id = $2
	`, id, userID).Scan(&record.ID, &record.VehicleID, &record.VehicleName, &record.OccurredOn, &kind, &record.Comment, &amount)
	if err != nil {
		return additionalCostRecord{}, err
	}
	if kind < 0 || kind >= len(costKindNames) {
		return additionalCostRecord{}, errors.New("invalid additional cost kind in database")
	}
	record.Kind = costKindNames[kind]
	record.KindLabel = costKindLabel(kind)
	record.Cost = amount.StringFixed(2)
	return record, nil
}

func (s *Server) additionalCostError(w http.ResponseWriter, err error, update bool) {
	if errors.Is(err, errForeignVehicle) {
		if update {
			writeError(w, http.StatusUnprocessableEntity, "Nemôžete upravovať náklady iného používateľa.")
		} else {
			writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať náklady pre iného používateľa.")
		}
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "Dodatočný náklad sa nepodarilo uložiť.")
}

func positiveCost(raw string) (decimal.Decimal, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || !amount.IsPositive() || amount.GreaterThan(decimal.RequireFromString("99999999.99")) {
		return decimal.Zero, errInvalidAdditionalCost
	}
	return amount, nil
}

func costKindOptions() []map[string]string {
	options := make([]map[string]string, 0, len(costKindNames))
	for index, kind := range costKindNames {
		options = append(options, map[string]string{"value": kind, "label": costKindLabel(index)})
	}
	return options
}

func costKindLabel(kind int) string {
	labels := []string{
		"Výmena oleja", "Výmena palivového filtra", "Technická kontrola", "Emisná kontrola", "Prezutie pneumatík", "Oprava", "Iné",
	}
	if kind < 0 || kind >= len(labels) {
		return ""
	}
	return labels[kind]
}

var errInvalidAdditionalCost = errors.New("invalid additional cost")
