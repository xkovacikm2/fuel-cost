package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

func Check(ctx context.Context, database *sql.DB, today time.Time) ([]int64, error) {
	rows, err := database.QueryContext(ctx, "SELECT id FROM maintenance_reminder_rules WHERE active=true ORDER BY id")
	if err != nil {
		return nil, err
	}
	ruleIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ruleIDs = append(ruleIDs, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	queued := make([]int64, 0)
	for _, id := range ruleIDs {
		notificationID, err := evaluateRule(ctx, database, id, today)
		if err != nil {
			return nil, err
		}
		if notificationID != nil {
			queued = append(queued, *notificationID)
		}
	}
	return queued, nil
}

func evaluateRule(ctx context.Context, database *sql.DB, ruleID int64, today time.Time) (*int64, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var vehicleID int64
	var kind int
	var intervalDays sql.NullInt64
	var intervalKM sql.NullString
	var active bool
	err = tx.QueryRowContext(ctx, `
		SELECT vehicle_id,kind,interval_days,interval_km,active
		FROM maintenance_reminder_rules WHERE id=$1 FOR UPDATE
	`, ruleID).Scan(&vehicleID, &kind, &intervalDays, &intervalKM, &active)
	if errors.Is(err, sql.ErrNoRows) || !active {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	if kind < 0 || kind >= 7 {
		return nil, fmt.Errorf("maintenance reminder %d has invalid kind %d", ruleID, kind)
	}

	date := today.Format(time.DateOnly)
	var baselineID int64
	var baselineDate time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id,occurred_on FROM additional_costs
		WHERE vehicle_id=$1 AND kind=$2 AND occurred_on <= $3
		ORDER BY occurred_on DESC,created_at DESC LIMIT 1
	`, vehicleID, kind, date).Scan(&baselineID, &baselineDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}

	var travelled decimal.Decimal
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(distance_km),0) FROM refuelings
		WHERE vehicle_id=$1 AND refueled_on > $2 AND refueled_on <= $3
	`, vehicleID, baselineDate.Format(time.DateOnly), date).Scan(&travelled); err != nil {
		return nil, err
	}

	leads, err := loadLeads(ctx, tx, ruleID)
	if err != nil {
		return nil, err
	}
	existing, err := loadNotifications(ctx, tx, ruleID, baselineID)
	if err != nil {
		return nil, err
	}
	var days *int
	if intervalDays.Valid {
		value := int(intervalDays.Int64)
		days = &value
	}
	var kilometres *decimal.Decimal
	if intervalKM.Valid {
		value, err := decimal.NewFromString(intervalKM.String)
		if err != nil {
			return nil, err
		}
		kilometres = &value
	}
	cycle := CalculateCycleFromDistance(baselineDate, today, days, kilometres, travelled)
	decision := Evaluate(Rule{IntervalDays: days, IntervalKM: kilometres}, cycle, leads, existing)

	for _, id := range decision.MarkSkipped {
		if _, err := tx.ExecContext(ctx, "UPDATE maintenance_notifications SET status=2,updated_at=now() at time zone 'utc' WHERE id=$1 AND status=0", id); err != nil {
			return nil, err
		}
	}
	for _, notification := range decision.Skipped {
		if _, err := insertNotification(ctx, tx, ruleID, baselineID, notification); err != nil {
			return nil, err
		}
	}
	var queuedID *int64
	if decision.Queued != nil {
		id, err := insertNotification(ctx, tx, ruleID, baselineID, *decision.Queued)
		if err != nil {
			return nil, err
		}
		queuedID = &id
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return queuedID, nil
}

func loadLeads(ctx context.Context, tx *sql.Tx, ruleID int64) ([]Lead, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id,days_before,kilometres_before FROM maintenance_reminder_leads
		WHERE maintenance_reminder_rule_id=$1 ORDER BY days_before DESC NULLS LAST,kilometres_before DESC NULLS LAST,id
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	leads := make([]Lead, 0)
	for rows.Next() {
		var lead Lead
		var days sql.NullInt64
		var kilometres sql.NullString
		if err := rows.Scan(&lead.ID, &days, &kilometres); err != nil {
			return nil, err
		}
		if days.Valid {
			value := int(days.Int64)
			lead.DaysBefore = &value
		}
		if kilometres.Valid {
			value, err := decimal.NewFromString(kilometres.String)
			if err != nil {
				return nil, err
			}
			lead.KilometresBefore = &value
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func loadNotifications(ctx context.Context, tx *sql.Tx, ruleID, costID int64) ([]Notification, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id,maintenance_reminder_lead_id,notification_kind,status
		FROM maintenance_notifications WHERE maintenance_reminder_rule_id=$1 AND additional_cost_id=$2
	`, ruleID, costID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	notifications := make([]Notification, 0)
	for rows.Next() {
		var notification Notification
		var leadID sql.NullInt64
		var kind, status int
		if err := rows.Scan(&notification.ID, &leadID, &kind, &status); err != nil {
			return nil, err
		}
		if leadID.Valid {
			value := leadID.Int64
			notification.LeadID = &value
		}
		if kind == 0 {
			notification.Kind = NotificationAdvance
		} else {
			notification.Kind = NotificationDue
		}
		switch status {
		case 0:
			notification.Status = StatusQueued
		case 1:
			notification.Status = StatusSent
		case 2:
			notification.Status = StatusSkipped
		case 3:
			notification.Status = StatusFailed
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func insertNotification(ctx context.Context, tx *sql.Tx, ruleID, baselineID int64, notification PlannedNotification) (int64, error) {
	kind := 0
	if notification.Kind == NotificationDue {
		kind = 1
	}
	condition := 0
	if notification.Condition == ConditionMileage {
		condition = 1
	}
	status := 0
	switch notification.Status {
	case StatusSent:
		status = 1
	case StatusSkipped:
		status = 2
	case StatusFailed:
		status = 3
	}
	var leadID any
	if notification.LeadID != nil {
		leadID = *notification.LeadID
	}
	var days any
	if notification.DaysRemaining != nil {
		days = *notification.DaysRemaining
	}
	var kilometres any
	if notification.KilometresRemaining != nil {
		kilometres = notification.KilometresRemaining.StringFixed(2)
	}
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO maintenance_notifications (maintenance_reminder_rule_id,maintenance_reminder_lead_id,additional_cost_id,
			notification_kind,trigger_condition,status,days_remaining,kilometres_remaining,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now() at time zone 'utc',now() at time zone 'utc') RETURNING id
	`, ruleID, leadID, baselineID, kind, condition, status, days, kilometres).Scan(&id)
	return id, err
}
