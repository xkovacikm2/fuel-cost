package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	maxImportBytes   = 10 * 1024 * 1024
	maxImportRecords = 100_000
	maxImportDepth   = 32
)

var importDecimalPattern = regexp.MustCompile(`^-?\d{1,8}(?:\.\d{1,2})?$`)
var importTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?(?:Z|[+-]\d{2}:\d{2})$`)

type importRoot struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Account    importAccount     `json:"account"`
	Vehicles   []json.RawMessage `json:"vehicles"`
}

type importAccount struct {
	EmailAddress string `json:"email_address"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type importVehicle struct {
	SourceID      int64
	Name          string
	FuelType      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Refuelings    []importRefueling
	Costs         []importCost
	Rules         []importRule
	Notifications []importNotification
}

type importRefueling struct {
	SourceID      int64     `json:"id"`
	Date          string    `json:"refueled_on"`
	Distance      string    `json:"distance_km"`
	Amount        string    `json:"amount"`
	Cost          string    `json:"cost"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
	CreatedAtTime time.Time `json:"-"`
	UpdatedAtTime time.Time `json:"-"`
}

type importCost struct {
	SourceID      int64     `json:"id"`
	Date          string    `json:"occurred_on"`
	Kind          string    `json:"kind"`
	Comment       *string   `json:"comment"`
	Cost          string    `json:"cost"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
	CreatedAtTime time.Time `json:"-"`
	UpdatedAtTime time.Time `json:"-"`
}

type importRule struct {
	SourceID   int64
	Kind       string
	Days       *int
	Kilometres *string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Leads      []importLead
}

type importLead struct {
	SourceID      int64     `json:"id"`
	RuleSourceID  int64     `json:"-"`
	Days          *int      `json:"days_before"`
	Kilometres    *string   `json:"kilometres_before"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
	CreatedAtTime time.Time `json:"-"`
	UpdatedAtTime time.Time `json:"-"`
}

type importNotification struct {
	SourceID      int64      `json:"id"`
	RuleID        int64      `json:"maintenance_reminder_rule_id"`
	LeadID        *int64     `json:"maintenance_reminder_lead_id"`
	CostID        int64      `json:"additional_cost_id"`
	Kind          string     `json:"notification_kind"`
	Condition     string     `json:"trigger_condition"`
	Status        string     `json:"status"`
	Days          *int       `json:"days_remaining"`
	Kilometres    *string    `json:"kilometres_remaining"`
	SentAt        *string    `json:"sent_at"`
	SentAtTime    *time.Time `json:"-"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
	CreatedAtTime time.Time  `json:"-"`
	UpdatedAtTime time.Time  `json:"-"`
}

type importResult struct {
	Imported                map[string]int `json:"imported"`
	Skipped                 map[string]int `json:"skipped"`
	Reasons                 map[string]int `json:"reasons"`
	NormalizedNotifications int            `json:"normalized_notifications"`
}

type importState struct {
	result importResult
	mapped map[string]map[int64]int64
}

type invalidImportError struct{ message string }

func (e invalidImportError) Error() string { return e.message }

func (s *Server) importUserData(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+1)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Import: vyberte JSON súbor do 10 MiB.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil || header.Size < 1 || header.Size > maxImportBytes {
		writeError(w, http.StatusUnprocessableEntity, "Import: vyberte JSON súbor do 10 MiB.")
		return
	}
	defer func() { _ = file.Close() }()
	contents, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil || len(contents) < 1 || len(contents) > maxImportBytes {
		writeError(w, http.StatusUnprocessableEntity, "Import: vyberte JSON súbor do 10 MiB.")
		return
	}
	graph, err := parseImport(contents)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Import zlyhal: "+err.Error())
		return
	}
	result, err := s.mergeImport(r.Context(), current.id, graph)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Import zlyhal. Žiadne dáta neboli zmenené. Skúste znova.")
		return
	}
	message := fmt.Sprintf("Import: %d nových, %d preskočených záznamov.", sumCounts(result.Imported), sumCounts(result.Skipped))
	reasonLabels := map[string]string{
		"duplicate": "Duplicity", "ambiguous_vehicle": "Nejednoznačné vozidlá",
		"rule_configuration": "Rozdielne nastavenia pripomienok", "dependency": "Závislé záznamy",
	}
	for _, reason := range []string{"duplicate", "ambiguous_vehicle", "rule_configuration", "dependency"} {
		if result.Reasons[reason] > 0 {
			message += fmt.Sprintf(" %s: %d.", reasonLabels[reason], result.Reasons[reason])
		}
	}
	if result.NormalizedNotifications > 0 {
		message += fmt.Sprintf(" Čakajúce upozornenia označené ako preskočené: %d.", result.NormalizedNotifications)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": message, "result": result})
}

func parseImport(contents []byte) ([]importVehicle, error) {
	if len(contents) < 1 || len(contents) > maxImportBytes {
		return nil, invalidImportError{"file: súbor chýba alebo je príliš veľký."}
	}
	var generic any
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	if err := decoder.Decode(&generic); err != nil || jsonDepth(generic) > maxImportDepth {
		return nil, invalidImportError{"Neplatný JSON alebo príliš hlboké vnorenie."}
	}
	if err := requireKeys(generic, []string{"format", "version", "exported_at", "account", "vehicles"}, nil, "root"); err != nil {
		return nil, err
	}
	var root importRoot
	if err := decodeStrict(contents, &root); err != nil {
		return nil, invalidImportError{"Neplatný JSON alebo príliš hlboké vnorenie."}
	}
	if root.Format != "spotreba" || root.Version != 1 {
		return nil, invalidImportError{"version: nepodporovaný formát alebo verzia."}
	}
	if !validTimestamp(root.ExportedAt) {
		return nil, invalidImportError{"exported_at: neplatná hodnota."}
	}
	if err := requireKeys(objectValue(generic, "account"), []string{"email_address", "created_at", "updated_at"}, nil, "account"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(root.Account.EmailAddress) == "" || len(root.Account.EmailAddress) > 1000 || !validTimestamp(root.Account.CreatedAt) || !validTimestamp(root.Account.UpdatedAt) {
		return nil, invalidImportError{"account: neplatné alebo chýbajúce polia."}
	}
	if len(root.Vehicles) > maxImportRecords {
		return nil, invalidImportError{"vehicles: príliš veľa záznamov."}
	}
	graph := make([]importVehicle, 0, len(root.Vehicles))
	seen := make(map[string]map[int64]bool)
	count := 0
	for index, raw := range root.Vehicles {
		vehicle, err := parseVehicle(raw, index, seen, &count)
		if err != nil {
			return nil, err
		}
		graph = append(graph, vehicle)
	}
	return graph, nil
}

func parseVehicle(raw json.RawMessage, index int, seen map[string]map[int64]bool, count *int) (importVehicle, error) {
	path := fmt.Sprintf("vehicles[%d]", index)
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return importVehicle{}, invalidImportError{path + ": neplatná hodnota."}
	}
	if err := requireKeys(generic, []string{"id", "name", "fuel_type", "created_at", "updated_at", "refuelings", "additional_costs", "maintenance_reminder_rules", "maintenance_notifications"}, nil, path); err != nil {
		return importVehicle{}, err
	}
	var wire struct {
		ID            int64             `json:"id"`
		Name          string            `json:"name"`
		FuelType      string            `json:"fuel_type"`
		CreatedAt     string            `json:"created_at"`
		UpdatedAt     string            `json:"updated_at"`
		Refuelings    []json.RawMessage `json:"refuelings"`
		Costs         []json.RawMessage `json:"additional_costs"`
		Rules         []json.RawMessage `json:"maintenance_reminder_rules"`
		Notifications []json.RawMessage `json:"maintenance_notifications"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return importVehicle{}, invalidImportError{path + ": neplatná hodnota."}
	}
	if wire.ID <= 0 || strings.TrimSpace(wire.Name) == "" || len(wire.Name) > 1000 || !validEnum(wire.FuelType, fuelTypes) || !validTimestamp(wire.CreatedAt) || !validTimestamp(wire.UpdatedAt) {
		return importVehicle{}, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	vehicle := importVehicle{SourceID: wire.ID, Name: wire.Name, FuelType: wire.FuelType, CreatedAt: parseTimestamp(wire.CreatedAt), UpdatedAt: parseTimestamp(wire.UpdatedAt)}
	if err := registerSource(seen, "vehicles", wire.ID, path); err != nil {
		return vehicle, err
	}
	if err := countImportRecords(count, len(wire.Refuelings)+len(wire.Costs)+len(wire.Rules)+len(wire.Notifications)+1); err != nil {
		return vehicle, err
	}
	for i, child := range wire.Refuelings {
		item, err := parseRefueling(child, path+fmt.Sprintf(".refuelings[%d]", i), seen)
		if err != nil {
			return vehicle, err
		}
		vehicle.Refuelings = append(vehicle.Refuelings, item)
	}
	for i, child := range wire.Costs {
		item, err := parseCost(child, path+fmt.Sprintf(".additional_costs[%d]", i), seen)
		if err != nil {
			return vehicle, err
		}
		vehicle.Costs = append(vehicle.Costs, item)
	}
	for i, child := range wire.Rules {
		item, err := parseRule(child, path+fmt.Sprintf(".maintenance_reminder_rules[%d]", i), seen, count)
		if err != nil {
			return vehicle, err
		}
		vehicle.Rules = append(vehicle.Rules, item)
	}
	for i, child := range wire.Notifications {
		item, err := parseNotification(child, path+fmt.Sprintf(".maintenance_notifications[%d]", i), seen)
		if err != nil {
			return vehicle, err
		}
		vehicle.Notifications = append(vehicle.Notifications, item)
	}
	rulesByID := make(map[int64]importRule, len(vehicle.Rules))
	costsByID := make(map[int64]importCost, len(vehicle.Costs))
	leadsByID := make(map[int64]int64)
	for _, rule := range vehicle.Rules {
		rulesByID[rule.SourceID] = rule
		for _, lead := range rule.Leads {
			leadsByID[lead.SourceID] = rule.SourceID
		}
	}
	for _, cost := range vehicle.Costs {
		costsByID[cost.SourceID] = cost
	}
	for _, notification := range vehicle.Notifications {
		rule, ruleOK := rulesByID[notification.RuleID]
		cost, costOK := costsByID[notification.CostID]
		if !ruleOK || !costOK || rule.Kind != cost.Kind {
			return vehicle, invalidImportError{path + ".maintenance_notifications: neplatné prepojenie pravidla alebo nákladu."}
		}
		if notification.LeadID != nil && leadsByID[*notification.LeadID] != notification.RuleID {
			return vehicle, invalidImportError{path + ".maintenance_notifications: neplatné prepojenie predstihu."}
		}
	}
	return vehicle, nil
}

func parseRefueling(raw json.RawMessage, path string, seen map[string]map[int64]bool) (importRefueling, error) {
	keys := []string{"id", "refueled_on", "distance_km", "amount", "cost", "created_at", "updated_at"}
	if err := requireRawKeys(raw, keys, nil, path); err != nil {
		return importRefueling{}, err
	}
	var item importRefueling
	if err := json.Unmarshal(raw, &item); err != nil {
		return item, invalidImportError{path + ": neplatná hodnota."}
	}
	if item.SourceID <= 0 || !validDate(item.Date) || !validPositiveDecimal(item.Distance) || !validPositiveDecimal(item.Amount) || !validPositiveDecimal(item.Cost) || !validTimestamp(item.CreatedAt) || !validTimestamp(item.UpdatedAt) {
		return item, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	if err := registerSource(seen, "refuelings", item.SourceID, path); err != nil {
		return item, err
	}
	item.CreatedAtTime, item.UpdatedAtTime = parseTimestamp(item.CreatedAt), parseTimestamp(item.UpdatedAt)
	return item, nil
}

func parseCost(raw json.RawMessage, path string, seen map[string]map[int64]bool) (importCost, error) {
	keys := []string{"id", "occurred_on", "kind", "comment", "cost", "created_at", "updated_at"}
	if err := requireRawKeys(raw, keys, []string{"comment"}, path); err != nil {
		return importCost{}, err
	}
	var item importCost
	if err := json.Unmarshal(raw, &item); err != nil {
		return item, invalidImportError{path + ": neplatná hodnota."}
	}
	if item.SourceID <= 0 || !validDate(item.Date) || !validEnum(item.Kind, costKinds) || !validPositiveDecimal(item.Cost) || (item.Comment != nil && len([]rune(*item.Comment)) > 1000) || !validTimestamp(item.CreatedAt) || !validTimestamp(item.UpdatedAt) {
		return item, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	if err := registerSource(seen, "additional_costs", item.SourceID, path); err != nil {
		return item, err
	}
	item.CreatedAtTime, item.UpdatedAtTime = parseTimestamp(item.CreatedAt), parseTimestamp(item.UpdatedAt)
	return item, nil
}

func parseRule(raw json.RawMessage, path string, seen map[string]map[int64]bool, count *int) (importRule, error) {
	keys := []string{"id", "kind", "interval_days", "interval_km", "active", "created_at", "updated_at", "maintenance_reminder_leads"}
	if err := requireRawKeys(raw, keys, nil, path); err != nil {
		return importRule{}, err
	}
	var wire struct {
		ID         int64             `json:"id"`
		Kind       string            `json:"kind"`
		Days       *int              `json:"interval_days"`
		Kilometres *string           `json:"interval_km"`
		Active     bool              `json:"active"`
		CreatedAt  string            `json:"created_at"`
		UpdatedAt  string            `json:"updated_at"`
		Leads      []json.RawMessage `json:"maintenance_reminder_leads"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return importRule{}, invalidImportError{path + ": neplatná hodnota."}
	}
	if wire.ID <= 0 || !validEnum(wire.Kind, costKinds) || (wire.Days == nil && wire.Kilometres == nil) || (wire.Days != nil && *wire.Days <= 0) || (wire.Kilometres != nil && !validPositiveDecimal(*wire.Kilometres)) || !validTimestamp(wire.CreatedAt) || !validTimestamp(wire.UpdatedAt) {
		return importRule{}, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	if err := registerSource(seen, "maintenance_reminder_rules", wire.ID, path); err != nil {
		return importRule{}, err
	}
	if err := countImportRecords(count, len(wire.Leads)); err != nil {
		return importRule{}, err
	}
	item := importRule{SourceID: wire.ID, Kind: wire.Kind, Days: wire.Days, Kilometres: wire.Kilometres, Active: wire.Active, CreatedAt: parseTimestamp(wire.CreatedAt), UpdatedAt: parseTimestamp(wire.UpdatedAt)}
	for i, rawLead := range wire.Leads {
		lead, err := parseLead(rawLead, path+fmt.Sprintf(".maintenance_reminder_leads[%d]", i), seen, item)
		if err != nil {
			return item, err
		}
		item.Leads = append(item.Leads, lead)
	}
	return item, nil
}

func parseLead(raw json.RawMessage, path string, seen map[string]map[int64]bool, rule importRule) (importLead, error) {
	keys := []string{"id", "days_before", "kilometres_before", "created_at", "updated_at"}
	if err := requireRawKeys(raw, keys, nil, path); err != nil {
		return importLead{}, err
	}
	var item importLead
	if err := json.Unmarshal(raw, &item); err != nil {
		return item, invalidImportError{path + ": neplatná hodnota."}
	}
	if item.SourceID <= 0 || (item.Days == nil) == (item.Kilometres == nil) || (item.Days != nil && (*item.Days <= 0 || rule.Days == nil)) || (item.Kilometres != nil && (!validPositiveDecimal(*item.Kilometres) || rule.Kilometres == nil)) || !validTimestamp(item.CreatedAt) || !validTimestamp(item.UpdatedAt) {
		return item, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	if err := registerSource(seen, "maintenance_reminder_leads", item.SourceID, path); err != nil {
		return item, err
	}
	item.RuleSourceID = rule.SourceID
	item.CreatedAtTime, item.UpdatedAtTime = parseTimestamp(item.CreatedAt), parseTimestamp(item.UpdatedAt)
	return item, nil
}

func parseNotification(raw json.RawMessage, path string, seen map[string]map[int64]bool) (importNotification, error) {
	keys := []string{"id", "maintenance_reminder_rule_id", "maintenance_reminder_lead_id", "additional_cost_id", "notification_kind", "trigger_condition", "status", "days_remaining", "kilometres_remaining", "sent_at", "created_at", "updated_at"}
	if err := requireRawKeys(raw, keys, nil, path); err != nil {
		return importNotification{}, err
	}
	var item importNotification
	if err := json.Unmarshal(raw, &item); err != nil {
		return item, invalidImportError{path + ": neplatná hodnota."}
	}
	validKind := item.Kind == "advance" || item.Kind == "due"
	validCondition := item.Condition == "date" || item.Condition == "mileage"
	validStatus := item.Status == "queued" || item.Status == "sent" || item.Status == "skipped" || item.Status == "failed"
	if item.SourceID <= 0 || item.RuleID <= 0 || item.CostID <= 0 || !validKind || !validCondition || !validStatus || (item.Kind == "advance") != (item.LeadID != nil) || (item.LeadID != nil && *item.LeadID <= 0) || (item.Kilometres != nil && !validDecimal(*item.Kilometres)) || !validTimestamp(item.CreatedAt) || !validTimestamp(item.UpdatedAt) || (item.SentAt != nil && !validTimestamp(*item.SentAt)) {
		return item, invalidImportError{path + ": neplatné hodnoty záznamu."}
	}
	if err := registerSource(seen, "maintenance_notifications", item.SourceID, path); err != nil {
		return item, err
	}
	item.CreatedAtTime, item.UpdatedAtTime = parseTimestamp(item.CreatedAt), parseTimestamp(item.UpdatedAt)
	if item.SentAt != nil {
		value := parseTimestamp(*item.SentAt)
		item.SentAtTime = &value
	}
	return item, nil
}

func (s *Server) mergeImport(ctx context.Context, userID int64, vehicles []importVehicle) (importResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return importResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&lockedID); err != nil {
		return importResult{}, err
	}
	state := importState{result: importResult{Imported: map[string]int{}, Skipped: map[string]int{}, Reasons: map[string]int{}}, mapped: map[string]map[int64]int64{}}
	for _, vehicle := range vehicles {
		if err := s.mergeVehicle(ctx, tx, userID, vehicle, &state); err != nil {
			return importResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return importResult{}, err
	}
	return state.result, nil
}

func (s *Server) mergeVehicle(ctx context.Context, tx *sql.Tx, userID int64, vehicle importVehicle, state *importState) error {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM vehicles WHERE user_id=$1 AND name=$2 AND fuel_type=$3 ORDER BY id LIMIT 2", userID, vehicle.Name, fuelTypes[vehicle.FuelType])
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(ids) > 1 {
		state.skip("vehicles", "ambiguous_vehicle")
		for range vehicle.Refuelings {
			state.skip("refuelings", "ambiguous_vehicle")
		}
		for range vehicle.Costs {
			state.skip("additional_costs", "ambiguous_vehicle")
		}
		for _, rule := range vehicle.Rules {
			state.skip("maintenance_reminder_rules", "ambiguous_vehicle")
			for range rule.Leads {
				state.skip("maintenance_reminder_leads", "ambiguous_vehicle")
			}
		}
		for range vehicle.Notifications {
			state.skip("maintenance_notifications", "ambiguous_vehicle")
		}
		return nil
	}
	var vehicleID int64
	if len(ids) == 1 {
		vehicleID = ids[0]
		state.skip("vehicles", "duplicate")
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,$2,$3,$4,$5) RETURNING id`, userID, vehicle.Name, fuelTypes[vehicle.FuelType], vehicle.CreatedAt, vehicle.UpdatedAt).Scan(&vehicleID)
		if err != nil {
			return err
		}
		state.add("vehicles")
	}
	state.mapID("vehicles", vehicle.SourceID, vehicleID)
	costIDs := make(map[int64]int64)
	for _, refueling := range vehicle.Refuelings {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM refuelings WHERE vehicle_id=$1 AND refueled_on=$2 AND distance_km=$3 AND amount=$4 AND cost=$5 ORDER BY id LIMIT 1`, vehicleID, refueling.Date, refueling.Distance, refueling.Amount, refueling.Cost).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, vehicleID, refueling.Date, refueling.Distance, refueling.Amount, refueling.Cost, refueling.CreatedAt, refueling.UpdatedAt).Scan(&id)
			if err != nil {
				return err
			}
			state.add("refuelings")
		} else if err != nil {
			return err
		} else {
			state.skip("refuelings", "duplicate")
		}
		state.mapID("refuelings", refueling.SourceID, id)
	}
	for _, cost := range vehicle.Costs {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM additional_costs WHERE vehicle_id=$1 AND occurred_on=$2 AND kind=$3 AND cost=$4 ORDER BY id LIMIT 1`, vehicleID, cost.Date, costKinds[cost.Kind], cost.Cost).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,comment,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, vehicleID, cost.Date, costKinds[cost.Kind], cost.Cost, cost.Comment, cost.CreatedAt, cost.UpdatedAt).Scan(&id)
			if err != nil {
				return err
			}
			state.add("additional_costs")
		} else if err != nil {
			return err
		} else {
			state.skip("additional_costs", "duplicate")
		}
		state.mapID("additional_costs", cost.SourceID, id)
		costIDs[cost.SourceID] = id
	}
	ruleIDs := make(map[int64]int64)
	leadIDs := make(map[int64]int64)
	for _, rule := range vehicle.Rules {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id,interval_days,interval_km,active FROM maintenance_reminder_rules WHERE vehicle_id=$1 AND kind=$2 ORDER BY id LIMIT 1`, vehicleID, costKinds[rule.Kind]).Scan(&id, new(sql.NullInt64), new(sql.NullString), new(bool))
		if err == nil {
			var days sql.NullInt64
			var km sql.NullString
			var active bool
			if err := tx.QueryRowContext(ctx, `SELECT interval_days,interval_km,active FROM maintenance_reminder_rules WHERE id=$1`, id).Scan(&days, &km, &active); err != nil {
				return err
			}
			if !sameRuleConfig(days, km, active, rule) {
				state.skip("maintenance_reminder_rules", "rule_configuration")
				for range rule.Leads {
					state.skip("maintenance_reminder_leads", "dependency")
				}
				continue
			}
			state.skip("maintenance_reminder_rules", "duplicate")
		} else if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO maintenance_reminder_rules (vehicle_id,kind,interval_days,interval_km,active,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, vehicleID, costKinds[rule.Kind], rule.Days, rule.Kilometres, rule.Active, rule.CreatedAt, rule.UpdatedAt).Scan(&id)
			if err != nil {
				return err
			}
			state.add("maintenance_reminder_rules")
		} else {
			return err
		}
		state.mapID("maintenance_reminder_rules", rule.SourceID, id)
		ruleIDs[rule.SourceID] = id
		for _, lead := range rule.Leads {
			var leadID int64
			var query string
			var args []any
			if lead.Days != nil {
				query = `SELECT id FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id=$1 AND days_before=$2`
				args = []any{id, *lead.Days}
			} else {
				query = `SELECT id FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id=$1 AND kilometres_before=$2`
				args = []any{id, *lead.Kilometres}
			}
			err := tx.QueryRowContext(ctx, query, args...).Scan(&leadID)
			if errors.Is(err, sql.ErrNoRows) {
				err = tx.QueryRowContext(ctx, `INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,days_before,kilometres_before,created_at,updated_at) VALUES ($1,$2,$3,$4,$5) RETURNING id`, id, lead.Days, lead.Kilometres, lead.CreatedAt, lead.UpdatedAt).Scan(&leadID)
				if err != nil {
					return err
				}
				state.add("maintenance_reminder_leads")
			} else if err != nil {
				return err
			} else {
				state.skip("maintenance_reminder_leads", "duplicate")
			}
			state.mapID("maintenance_reminder_leads", lead.SourceID, leadID)
			leadIDs[lead.SourceID] = leadID
		}
	}
	for _, notification := range vehicle.Notifications {
		ruleID, ruleOK := ruleIDs[notification.RuleID]
		costID, costOK := costIDs[notification.CostID]
		var leadID any
		if notification.LeadID != nil {
			mapped, ok := leadIDs[*notification.LeadID]
			if !ok {
				state.skip("maintenance_notifications", "dependency")
				continue
			}
			leadID = mapped
		}
		if !ruleOK || !costOK {
			state.skip("maintenance_notifications", "dependency")
			continue
		}
		kind := 0
		if notification.Kind == "due" {
			kind = 1
		}
		var found int64
		var err error
		if notification.Kind == "advance" {
			err = tx.QueryRowContext(ctx, `SELECT id FROM maintenance_notifications WHERE maintenance_reminder_rule_id=$1 AND additional_cost_id=$2 AND maintenance_reminder_lead_id=$3 AND notification_kind=$4`, ruleID, costID, leadID, kind).Scan(&found)
		} else {
			err = tx.QueryRowContext(ctx, `SELECT id FROM maintenance_notifications WHERE maintenance_reminder_rule_id=$1 AND additional_cost_id=$2 AND notification_kind=$3`, ruleID, costID, kind).Scan(&found)
		}
		if err == nil {
			state.skip("maintenance_notifications", "duplicate")
			state.mapID("maintenance_notifications", notification.SourceID, found)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		status := map[string]int{"queued": 0, "sent": 1, "skipped": 2, "failed": 3}[notification.Status]
		if notification.Status == "queued" {
			status = 2
			state.result.NormalizedNotifications++
		}
		trigger := 0
		if notification.Condition == "mileage" {
			trigger = 1
		}
		var sent any
		if notification.SentAt != nil {
			sent = *notification.SentAt
		}
		var days any
		if notification.Days != nil {
			days = *notification.Days
		}
		var km any
		if notification.Kilometres != nil {
			km = *notification.Kilometres
		}
		var id int64
		err = tx.QueryRowContext(ctx, `INSERT INTO maintenance_notifications (maintenance_reminder_rule_id,maintenance_reminder_lead_id,additional_cost_id,notification_kind,trigger_condition,status,days_remaining,kilometres_remaining,sent_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, ruleID, leadID, costID, kind, trigger, status, days, km, sent, notification.CreatedAt, notification.UpdatedAt).Scan(&id)
		if err != nil {
			return err
		}
		state.add("maintenance_notifications")
		state.mapID("maintenance_notifications", notification.SourceID, id)
	}
	return nil
}

func (s *importState) add(collection string) { s.result.Imported[collection]++ }
func (s *importState) skip(collection, reason string) {
	s.result.Skipped[collection]++
	s.result.Reasons[reason]++
}
func (s *importState) mapID(collection string, source, target int64) {
	if s.mapped[collection] == nil {
		s.mapped[collection] = map[int64]int64{}
	}
	s.mapped[collection][source] = target
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func sameRuleConfig(days sql.NullInt64, km sql.NullString, active bool, rule importRule) bool {
	if days.Valid != (rule.Days != nil) || km.Valid != (rule.Kilometres != nil) || active != rule.Active {
		return false
	}
	if days.Valid && int(days.Int64) != *rule.Days {
		return false
	}
	if km.Valid && decimal.RequireFromString(km.String).Cmp(decimal.RequireFromString(*rule.Kilometres)) != 0 {
		return false
	}
	return true
}

func requireKeys(value any, required, optional []string, path string) error {
	object, ok := value.(map[string]any)
	if !ok {
		return invalidImportError{path + ": neplatné alebo chýbajúce polia."}
	}
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		allowed[key] = true
		if _, ok := object[key]; !ok {
			return invalidImportError{path + ": neplatné alebo chýbajúce polia."}
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range object {
		if !allowed[key] {
			return invalidImportError{path + ": neplatné alebo chýbajúce polia."}
		}
	}
	return nil
}

func requireRawKeys(raw json.RawMessage, required, optional []string, path string) error {
	var object any
	if err := json.Unmarshal(raw, &object); err != nil {
		return invalidImportError{path + ": neplatná hodnota."}
	}
	return requireKeys(object, required, optional, path)
}

func objectValue(value any, key string) any {
	if object, ok := value.(map[string]any); ok {
		return object[key]
	}
	return nil
}
func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}
func jsonDepth(value any) int {
	switch item := value.(type) {
	case map[string]any:
		maximum := 0
		for _, child := range item {
			if depth := jsonDepth(child); depth > maximum {
				maximum = depth
			}
		}
		return maximum + 1
	case []any:
		maximum := 0
		for _, child := range item {
			if depth := jsonDepth(child); depth > maximum {
				maximum = depth
			}
		}
		return maximum + 1
	default:
		return 0
	}
}
func validEnum(value string, values map[string]int) bool { _, ok := values[value]; return ok }
func validDate(value string) bool {
	parsed, err := time.Parse(time.DateOnly, value)
	return err == nil && parsed.Format(time.DateOnly) == value && parsed.Year() >= 1
}
func validTimestamp(value string) bool {
	if !importTimestampPattern.MatchString(value) {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.Year() >= 1 && parsed.Year() <= 9999
}
func parseTimestamp(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed.UTC()
}
func validDecimal(value string) bool {
	if !importDecimalPattern.MatchString(value) {
		return false
	}
	_, err := decimal.NewFromString(value)
	return err == nil
}
func validPositiveDecimal(value string) bool {
	if !validDecimal(value) {
		return false
	}
	number, _ := decimal.NewFromString(value)
	return number.IsPositive()
}
func registerSource(seen map[string]map[int64]bool, collection string, id int64, path string) error {
	if seen[collection] == nil {
		seen[collection] = map[int64]bool{}
	}
	if seen[collection][id] {
		return invalidImportError{path + ": duplicitné zdrojové ID."}
	}
	seen[collection][id] = true
	return nil
}
func countImportRecords(count *int, add int) error {
	*count += add
	if *count > maxImportRecords {
		return invalidImportError{"records: príliš veľa záznamov."}
	}
	return nil
}
