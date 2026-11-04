package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

var fuelTypes = map[string]int{
	"petrol":      0,
	"diesel":      1,
	"electricity": 2,
}

var fuelTypeNames = []string{"petrol", "diesel", "electricity"}

type vehicleRecord struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	FuelType  string `json:"fuel_type"`
	FuelLabel string `json:"fuel_type_label"`
	Unit      string `json:"unit"`
}

type vehicleInput struct {
	Name     *string `json:"name"`
	FuelType *string `json:"fuel_type"`
}

func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, name, fuel_type FROM vehicles WHERE user_id = $1 ORDER BY name
	`, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
		return
	}
	defer func() { _ = rows.Close() }()
	vehicles := make([]vehicleRecord, 0)
	for rows.Next() {
		var id int64
		var name string
		var fuelType int
		if err := rows.Scan(&id, &name, &fuelType); err != nil {
			writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
			return
		}
		vehicles = append(vehicles, makeVehicleRecord(id, name, fuelType))
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlá sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": vehicles})
}

func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	var input vehicleInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	if input.Name == nil || strings.TrimSpace(*input.Name) == "" || input.FuelType == nil {
		writeError(w, http.StatusUnprocessableEntity, "Zadajte názov vozidla a typ paliva.")
		return
	}
	fuelType, ok := fuelTypes[*input.FuelType]
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "Typ paliva nie je platný.")
		return
	}
	var id int64
	err := s.db.QueryRowContext(r.Context(), `
		INSERT INTO vehicles (user_id, name, fuel_type, created_at, updated_at)
		VALUES ($1, $2, $3, now() at time zone 'utc', now() at time zone 'utc')
		RETURNING id
	`, current.id, *input.Name, fuelType).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo vytvoriť.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vehicle": makeVehicleRecord(id, *input.Name, fuelType)})
}

func (s *Server) showVehicle(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicle, err := s.findVehicle(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Vozidlo sa nenašlo.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo načítať.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicle": vehicle})
}

func (s *Server) updateVehicle(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicle, err := s.findVehicle(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Vozidlo sa nenašlo.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo načítať.")
		return
	}
	var input vehicleInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Neplatná požiadavka.")
		return
	}
	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == "" {
			writeError(w, http.StatusUnprocessableEntity, "Zadajte názov vozidla.")
			return
		}
		vehicle.Name = *input.Name
	}
	fuelType := fuelTypes[vehicle.FuelType]
	if input.FuelType != nil {
		value, ok := fuelTypes[*input.FuelType]
		if !ok {
			writeError(w, http.StatusUnprocessableEntity, "Typ paliva nie je platný.")
			return
		}
		fuelType = value
		vehicle.FuelType = *input.FuelType
		vehicle.FuelLabel, vehicle.Unit = fuelTypePresentation(value)
	}
	_, err = s.db.ExecContext(r.Context(), `
		UPDATE vehicles SET name = $1, fuel_type = $2, updated_at = now() at time zone 'utc'
		WHERE id = $3 AND user_id = $4
	`, vehicle.Name, fuelType, vehicle.ID, current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo upraviť.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicle": vehicle})
}

func (s *Server) deleteVehicle(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	vehicle, err := s.findVehicle(r, current.id)
	if errors.Is(err, sql.ErrNoRows) || err == errInvalidID {
		writeError(w, http.StatusNotFound, "Vozidlo sa nenašlo.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo načítať.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo odstrániť.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	statements := []string{
		`DELETE FROM maintenance_notifications WHERE maintenance_reminder_rule_id IN
			(SELECT id FROM maintenance_reminder_rules WHERE vehicle_id = $1) OR additional_cost_id IN
			(SELECT id FROM additional_costs WHERE vehicle_id = $1)`,
		`DELETE FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id IN
			(SELECT id FROM maintenance_reminder_rules WHERE vehicle_id = $1)`,
		`DELETE FROM maintenance_reminder_rules WHERE vehicle_id = $1`,
		`DELETE FROM additional_costs WHERE vehicle_id = $1`,
		`DELETE FROM refuelings WHERE vehicle_id = $1`,
		`DELETE FROM vehicles WHERE id = $1 AND user_id = $2`,
	}
	for i, statement := range statements {
		args := []any{vehicle.ID}
		if i == len(statements)-1 {
			args = append(args, current.id)
		}
		if _, err := tx.ExecContext(r.Context(), statement, args...); err != nil {
			writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo odstrániť.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Vozidlo sa nepodarilo odstrániť.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) findVehicle(r *http.Request, userID int64) (vehicleRecord, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return vehicleRecord{}, errInvalidID
	}
	var name string
	var fuelType int
	err = s.db.QueryRowContext(r.Context(), `
		SELECT id, name, fuel_type FROM vehicles WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&id, &name, &fuelType)
	if err != nil {
		return vehicleRecord{}, err
	}
	return makeVehicleRecord(id, name, fuelType), nil
}

var errInvalidID = errors.New("invalid id")

func makeVehicleRecord(id int64, name string, fuelType int) vehicleRecord {
	if fuelType < 0 || fuelType >= len(fuelTypeNames) {
		return vehicleRecord{ID: id, Name: name}
	}
	label, unit := fuelTypePresentation(fuelType)
	return vehicleRecord{ID: id, Name: name, FuelType: fuelTypeNames[fuelType], FuelLabel: label, Unit: unit}
}

func fuelTypePresentation(fuelType int) (string, string) {
	switch fuelType {
	case 0:
		return "Benzín", "L"
	case 1:
		return "Nafta", "L"
	case 2:
		return "Elektrina", "kWh"
	default:
		return "", ""
	}
}
