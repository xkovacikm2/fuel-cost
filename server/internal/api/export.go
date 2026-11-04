package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type exportRoot struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	ExportedAt string          `json:"exported_at"`
	Account    exportAccount   `json:"account"`
	Vehicles   []exportVehicle `json:"vehicles"`
}

type exportAccount struct {
	EmailAddress string `json:"email_address"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type exportVehicle struct {
	ID                       int64                  `json:"id"`
	Name                     string                 `json:"name"`
	FuelType                 string                 `json:"fuel_type"`
	CreatedAt                string                 `json:"created_at"`
	UpdatedAt                string                 `json:"updated_at"`
	Refuelings               []exportRefueling      `json:"refuelings"`
	AdditionalCosts          []exportAdditionalCost `json:"additional_costs"`
	MaintenanceReminderRules []exportReminderRule   `json:"maintenance_reminder_rules"`
	MaintenanceNotifications []exportNotification   `json:"maintenance_notifications"`
}

type exportRefueling struct {
	ID         int64  `json:"id"`
	RefueledOn string `json:"refueled_on"`
	DistanceKM string `json:"distance_km"`
	Amount     string `json:"amount"`
	Cost       string `json:"cost"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type exportAdditionalCost struct {
	ID         int64   `json:"id"`
	OccurredOn string  `json:"occurred_on"`
	Kind       string  `json:"kind"`
	Comment    *string `json:"comment"`
	Cost       string  `json:"cost"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type exportReminderRule struct {
	ID           int64        `json:"id"`
	Kind         string       `json:"kind"`
	IntervalDays *int         `json:"interval_days"`
	IntervalKM   *string      `json:"interval_km"`
	Active       bool         `json:"active"`
	CreatedAt    string       `json:"created_at"`
	UpdatedAt    string       `json:"updated_at"`
	Leads        []exportLead `json:"maintenance_reminder_leads"`
}

type exportLead struct {
	ID               int64   `json:"id"`
	DaysBefore       *int    `json:"days_before"`
	KilometresBefore *string `json:"kilometres_before"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type exportNotification struct {
	ID                        int64   `json:"id"`
	MaintenanceReminderRuleID int64   `json:"maintenance_reminder_rule_id"`
	MaintenanceReminderLeadID *int64  `json:"maintenance_reminder_lead_id"`
	AdditionalCostID          int64   `json:"additional_cost_id"`
	NotificationKind          string  `json:"notification_kind"`
	TriggerCondition          string  `json:"trigger_condition"`
	Status                    string  `json:"status"`
	DaysRemaining             *int    `json:"days_remaining"`
	KilometresRemaining       *string `json:"kilometres_remaining"`
	SentAt                    *string `json:"sent_at"`
	CreatedAt                 string  `json:"created_at"`
	UpdatedAt                 string  `json:"updated_at"`
}

type exportQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Server) exportUserData(w http.ResponseWriter, r *http.Request) {
	current := r.Context().Value(userContextKey).(user)
	data, err := s.buildExport(r.Context(), current.id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Export údajov sa nepodaril.")
		return
	}
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Export údajov sa nepodaril.")
		return
	}
	filename := "spotreba-" + time.Now().UTC().Format("20060102-150405") + ".json"
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contents)
}

func (s *Server) buildExport(ctx context.Context, userID int64) (exportRoot, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return exportRoot{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var root exportRoot
	root.Format, root.Version = "spotreba", 1
	root.ExportedAt = formatTimestamp(time.Now())
	var created, updated time.Time
	if err := tx.QueryRowContext(ctx, `SELECT email_address, created_at, updated_at FROM users WHERE id = $1`, userID).
		Scan(&root.Account.EmailAddress, &created, &updated); err != nil {
		return exportRoot{}, err
	}
	root.Account.CreatedAt, root.Account.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, fuel_type, created_at, updated_at FROM vehicles WHERE user_id = $1 ORDER BY id
	`, userID)
	if err != nil {
		return exportRoot{}, err
	}
	vehicles := make([]exportVehicle, 0)
	for rows.Next() {
		var vehicle exportVehicle
		var fuelType int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&vehicle.ID, &vehicle.Name, &fuelType, &createdAt, &updatedAt); err != nil {
			_ = rows.Close()
			return exportRoot{}, err
		}
		vehicle.FuelType = fuelTypeName(fuelType)
		vehicle.CreatedAt, vehicle.UpdatedAt = formatTimestamp(createdAt), formatTimestamp(updatedAt)
		vehicles = append(vehicles, vehicle)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return exportRoot{}, err
	}
	if err := rows.Close(); err != nil {
		return exportRoot{}, err
	}
	for i := range vehicles {
		if err := s.exportVehicleRecords(ctx, tx, &vehicles[i]); err != nil {
			return exportRoot{}, err
		}
	}
	root.Vehicles = vehicles
	if err := tx.Commit(); err != nil {
		return exportRoot{}, err
	}
	return root, nil
}

func (s *Server) exportVehicleRecords(ctx context.Context, q exportQueryer, vehicle *exportVehicle) error {
	rows, err := q.QueryContext(ctx, `
		SELECT id, refueled_on, distance_km, amount, cost, created_at, updated_at
		FROM refuelings WHERE vehicle_id = $1 ORDER BY id
	`, vehicle.ID)
	if err != nil {
		return err
	}
	vehicle.Refuelings = make([]exportRefueling, 0)
	for rows.Next() {
		var item exportRefueling
		var date time.Time
		var distance, amount, cost decimal.Decimal
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &date, &distance, &amount, &cost, &created, &updated); err != nil {
			_ = rows.Close()
			return err
		}
		item.RefueledOn = date.Format(time.DateOnly)
		item.DistanceKM, item.Amount, item.Cost = exportDecimal(distance), exportDecimal(amount), exportDecimal(cost)
		item.CreatedAt, item.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
		vehicle.Refuelings = append(vehicle.Refuelings, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	rows, err = q.QueryContext(ctx, `
		SELECT id, occurred_on, kind, comment, cost, created_at, updated_at
		FROM additional_costs WHERE vehicle_id = $1 ORDER BY id
	`, vehicle.ID)
	if err != nil {
		return err
	}
	vehicle.AdditionalCosts = make([]exportAdditionalCost, 0)
	for rows.Next() {
		var item exportAdditionalCost
		var date time.Time
		var kind int
		var comment sql.NullString
		var cost decimal.Decimal
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &date, &kind, &comment, &cost, &created, &updated); err != nil {
			_ = rows.Close()
			return err
		}
		item.OccurredOn = date.Format(time.DateOnly)
		item.Kind = costKindName(kind)
		if comment.Valid {
			item.Comment = &comment.String
		}
		item.Cost = exportDecimal(cost)
		item.CreatedAt, item.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
		vehicle.AdditionalCosts = append(vehicle.AdditionalCosts, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	rows, err = q.QueryContext(ctx, `
		SELECT id, kind, interval_days, interval_km, active, created_at, updated_at
		FROM maintenance_reminder_rules WHERE vehicle_id = $1 ORDER BY id
	`, vehicle.ID)
	if err != nil {
		return err
	}
	vehicle.MaintenanceReminderRules = make([]exportReminderRule, 0)
	ruleIDs := make([]int64, 0)
	for rows.Next() {
		var item exportReminderRule
		var kind int
		var days sql.NullInt64
		var kilometres sql.NullString
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &kind, &days, &kilometres, &item.Active, &created, &updated); err != nil {
			_ = rows.Close()
			return err
		}
		item.Kind = costKindName(kind)
		item.IntervalDays = nullInt(days)
		item.IntervalKM = exportNullDecimal(kilometres)
		item.CreatedAt, item.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
		vehicle.MaintenanceReminderRules = append(vehicle.MaintenanceReminderRules, item)
		ruleIDs = append(ruleIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for i := range vehicle.MaintenanceReminderRules {
		item := &vehicle.MaintenanceReminderRules[i]
		item.Leads, err = exportLeads(ctx, q, item.ID)
		if err != nil {
			return err
		}
	}
	vehicle.MaintenanceNotifications = make([]exportNotification, 0)
	if len(ruleIDs) == 0 {
		return nil
	}
	rows, err = q.QueryContext(ctx, `
		SELECT n.id, n.maintenance_reminder_rule_id, n.maintenance_reminder_lead_id, n.additional_cost_id,
		       n.notification_kind, n.trigger_condition, n.status, n.days_remaining, n.kilometres_remaining,
		       n.sent_at, n.created_at, n.updated_at
		FROM maintenance_notifications n JOIN maintenance_reminder_rules rr ON rr.id = n.maintenance_reminder_rule_id
		WHERE rr.vehicle_id = $1 ORDER BY n.id
	`, vehicle.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item exportNotification
		var kind, condition, status int
		var leadID, days sql.NullInt64
		var kilometres sql.NullString
		var sentAt sql.NullTime
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &item.MaintenanceReminderRuleID, &leadID, &item.AdditionalCostID,
			&kind, &condition, &status, &days, &kilometres, &sentAt, &created, &updated); err != nil {
			_ = rows.Close()
			return err
		}
		item.MaintenanceReminderLeadID = nullInt64(leadID)
		item.NotificationKind, item.TriggerCondition, item.Status = notificationKindName(kind), triggerConditionName(condition), notificationStatusName(status)
		item.DaysRemaining = nullInt(days)
		item.KilometresRemaining = exportNullDecimal(kilometres)
		if sentAt.Valid {
			value := formatTimestamp(sentAt.Time)
			item.SentAt = &value
		}
		item.CreatedAt, item.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
		vehicle.MaintenanceNotifications = append(vehicle.MaintenanceNotifications, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}

func exportLeads(ctx context.Context, q exportQueryer, ruleID int64) ([]exportLead, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, days_before, kilometres_before, created_at, updated_at
		FROM maintenance_reminder_leads WHERE maintenance_reminder_rule_id = $1 ORDER BY id
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	leads := make([]exportLead, 0)
	for rows.Next() {
		var item exportLead
		var days sql.NullInt64
		var kilometres sql.NullString
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &days, &kilometres, &created, &updated); err != nil {
			return nil, err
		}
		item.DaysBefore = nullInt(days)
		item.KilometresBefore = exportNullDecimal(kilometres)
		item.CreatedAt, item.UpdatedAt = formatTimestamp(created), formatTimestamp(updated)
		leads = append(leads, item)
	}
	return leads, rows.Err()
}

func exportDecimal(value decimal.Decimal) string {
	text := value.String()
	if !strings.Contains(text, ".") {
		return text + ".0"
	}
	return text
}

func exportNullDecimal(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := exportDecimal(decimal.RequireFromString(value.String))
	return &result
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000Z")
}

func nullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func costKindName(kind int) string {
	if kind < 0 || kind >= len(costKindNames) {
		return ""
	}
	return costKindNames[kind]
}

func fuelTypeName(fuelType int) string {
	if fuelType < 0 || fuelType >= len(fuelTypeNames) {
		return ""
	}
	return fuelTypeNames[fuelType]
}

func notificationKindName(kind int) string {
	if kind == 0 {
		return "advance"
	}
	if kind == 1 {
		return "due"
	}
	return ""
}

func triggerConditionName(condition int) string {
	if condition == 0 {
		return "date"
	}
	if condition == 1 {
		return "mileage"
	}
	return ""
}

func notificationStatusName(status int) string {
	statuses := []string{"queued", "sent", "skipped", "failed"}
	if status < 0 || status >= len(statuses) {
		return ""
	}
	return statuses[status]
}
