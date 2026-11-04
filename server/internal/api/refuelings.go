package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"spotreba/server/internal/domain"

	"github.com/shopspring/decimal"
)

type refuelingInput struct {
	VehicleID  *int64  `json:"vehicle_id"`
	RefueledOn *string `json:"refueled_on"`
	DistanceKM *string `json:"distance_km"`
	Amount     *string `json:"amount"`
	Cost       *string `json:"cost"`
}

type refuelingRecord struct {
	ID                  int64  `json:"id"`
	VehicleID           int64  `json:"vehicle_id"`
	VehicleName         string `json:"vehicle_name"`
	Unit                string `json:"unit"`
	RefueledOn          string `json:"refueled_on"`
	DistanceKM          string `json:"distance_km"`
	Amount              string `json:"amount"`
	Cost                string `json:"cost"`
	ConsumptionPer100KM string `json:"consumption_per_100km"`
	CostPer100KM        string `json:"cost_per_100km"`
}

func (s *Server) newRefueling(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicles, err := s.vehiclesForUser(r, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
		return
	}
	if len(vehicles) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"refueled_on": time.Now().In(s.appTZ).Format(time.DateOnly),
			"vehicle_id":  nil,
			"unit":        "",
			"vehicles":    vehicles,
		})
		return
	}

	selected := vehicles[0]
	var latestID int64
	err = s.db.QueryRowContext(r.Context(), `
		SELECT v.id FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id
		WHERE v.user_id = $1 ORDER BY r.refueled_on DESC, r.created_at DESC LIMIT 1
	`, current.id).Scan(&latestID)
	if err == nil {
		for _, vehicle := range vehicles {
			if vehicle.ID == latestID {
				selected = vehicle
				break
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "Posledné tankovanie sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"refueled_on": time.Now().In(s.appTZ).Format(time.DateOnly),
		"vehicle_id":  selected.ID,
		"unit":        selected.Unit,
		"vehicles":    vehicles,
	})
}

func (s *Server) createRefueling(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	var input refuelingInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	refueling, err := s.applyRefuelingInput(r, current.id, refuelingRecord{}, input, true)
	if err != nil {
		s.refuelingError(w, err)
		return
	}
	var id int64
	err = s.db.QueryRowContext(r.Context(), `
		INSERT INTO refuelings (vehicle_id, refueled_on, distance_km, amount, cost, created_at, updated_at)
		SELECT v.id, $2, $3, $4, $5, now() at time zone 'utc', now() at time zone 'utc'
		FROM vehicles v WHERE v.id = $1 AND v.user_id = $6
		RETURNING id
	`, refueling.VehicleID, refueling.RefueledOn, refueling.DistanceKM, refueling.Amount, refueling.Cost, current.id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať tankovania pre iného používateľa.")
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Tankovanie sa nepodarilo uložiť.")
		return
	}
	refueling.ID = id
	writeJSON(w, http.StatusCreated, map[string]any{"refueling": refueling})
}

func (s *Server) showRefueling(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	refueling, err := s.findRefueling(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Tankovanie sa nenašlo.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Tankovanie sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"refueling": refueling})
}

func (s *Server) updateRefueling(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	refueling, err := s.findRefueling(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Tankovanie sa nenašlo.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Tankovanie sa nepodarilo načítať.")
		return
	}
	var input refuelingInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	refueling, err = s.applyRefuelingInput(r, current.id, refueling, input, false)
	if err != nil {
		s.refuelingError(w, err)
		return
	}
	result, err := s.db.ExecContext(r.Context(), `
		UPDATE refuelings r SET vehicle_id = $1, refueled_on = $2, distance_km = $3, amount = $4, cost = $5,
			updated_at = now() at time zone 'utc'
		FROM vehicles v WHERE r.id = $6 AND v.id = $1 AND v.user_id = $7 AND r.vehicle_id IN
			(SELECT owned.id FROM vehicles owned WHERE owned.user_id = $7)
	`, refueling.VehicleID, refueling.RefueledOn, refueling.DistanceKM, refueling.Amount, refueling.Cost, refueling.ID, current.id)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Tankovanie sa nepodarilo upraviť.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete upravovať tankovania iného používateľa.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"refueling": refueling})
}

func (s *Server) deleteRefueling(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "Tankovanie sa nenašlo.")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `
		DELETE FROM refuelings r USING vehicles v
		WHERE r.id = $1 AND v.id = r.vehicle_id AND v.user_id = $2
	`, id, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Tankovanie sa nepodarilo odstrániť.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Tankovanie sa nenašlo.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) applyRefuelingInput(r *http.Request, userID int64, current refuelingRecord, input refuelingInput, required bool) (refuelingRecord, error) {
	if input.VehicleID != nil {
		if *input.VehicleID < 1 {
			return current, errInvalidRefueling
		}
		current.VehicleID = *input.VehicleID
	} else if required {
		return current, errInvalidRefueling
	}
	if input.RefueledOn != nil {
		if _, err := time.Parse(time.DateOnly, *input.RefueledOn); err != nil {
			return current, errInvalidRefueling
		}
		current.RefueledOn = *input.RefueledOn
	} else if required {
		return current, errInvalidRefueling
	}
	for _, value := range []*string{input.DistanceKM, input.Amount, input.Cost} {
		if value == nil && required {
			return current, errInvalidRefueling
		}
	}
	if input.DistanceKM != nil {
		current.DistanceKM = *input.DistanceKM
	}
	if input.Amount != nil {
		current.Amount = *input.Amount
	}
	if input.Cost != nil {
		current.Cost = *input.Cost
	}
	distance, err := positiveDecimal(current.DistanceKM)
	if err != nil {
		return current, errInvalidRefueling
	}
	amount, err := positiveDecimal(current.Amount)
	if err != nil {
		return current, errInvalidRefueling
	}
	cost, err := positiveDecimal(current.Cost)
	if err != nil {
		return current, errInvalidRefueling
	}
	var name string
	var fuelType int
	if err := s.db.QueryRowContext(r.Context(), "SELECT name, fuel_type FROM vehicles WHERE id = $1 AND user_id = $2", current.VehicleID, userID).Scan(&name, &fuelType); err != nil {
		return current, errForeignVehicle
	}
	current.VehicleName = name
	current.Unit = makeVehicleRecord(0, name, fuelType).Unit
	current.ConsumptionPer100KM = domain.ConsumptionPer100Km(amount, distance).StringFixed(2)
	current.CostPer100KM = domain.CostPer100Km(cost, distance).StringFixed(2)
	return current, nil
}

func (s *Server) findRefueling(r *http.Request, userID int64) (refuelingRecord, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return refuelingRecord{}, errInvalidID
	}
	var record refuelingRecord
	var amount, distance, cost decimal.Decimal
	var fuelType int
	err = s.db.QueryRowContext(r.Context(), `
		SELECT r.id, r.vehicle_id, v.name, v.fuel_type, r.refueled_on, r.distance_km, r.amount, r.cost
		FROM refuelings r JOIN vehicles v ON v.id = r.vehicle_id
		WHERE r.id = $1 AND v.user_id = $2
	`, id, userID).Scan(&record.ID, &record.VehicleID, &record.VehicleName, &fuelType, &record.RefueledOn, &distance, &amount, &cost)
	if err != nil {
		return refuelingRecord{}, err
	}
	record.Unit = makeVehicleRecord(0, record.VehicleName, fuelType).Unit
	record.DistanceKM = distance.StringFixed(2)
	record.Amount = amount.StringFixed(2)
	record.Cost = cost.StringFixed(2)
	record.ConsumptionPer100KM = domain.ConsumptionPer100Km(amount, distance).StringFixed(2)
	record.CostPer100KM = domain.CostPer100Km(cost, distance).StringFixed(2)
	return record, nil
}

func (s *Server) vehiclesForUser(r *http.Request, userID int64) ([]vehicleRecord, error) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT id, name, fuel_type FROM vehicles WHERE user_id = $1 ORDER BY name", userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	vehicles := make([]vehicleRecord, 0)
	for rows.Next() {
		var id int64
		var name string
		var fuelType int
		if err := rows.Scan(&id, &name, &fuelType); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, makeVehicleRecord(id, name, fuelType))
	}
	return vehicles, rows.Err()
}

var errInvalidRefueling = errors.New("invalid refueling")
var errForeignVehicle = errors.New("vehicle is not owned by current user")

func (s *Server) refuelingError(w http.ResponseWriter, err error) {
	if errors.Is(err, errForeignVehicle) {
		writeError(w, http.StatusUnprocessableEntity, "Nemôžete vytvárať tankovania pre iného používateľa.")
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "Tankovanie sa nepodarilo uložiť.")
}

func positiveDecimal(raw string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || !value.IsPositive() || value.GreaterThan(decimal.RequireFromString("99999999.99")) {
		return decimal.Zero, errInvalidRefueling
	}
	return value, nil
}
