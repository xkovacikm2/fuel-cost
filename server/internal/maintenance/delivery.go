package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"spotreba/server/internal/mail"

	"github.com/shopspring/decimal"
)

func CheckAndDeliver(ctx context.Context, database *sql.DB, today time.Time, sender mail.Sender) ([]int64, error) {
	if _, err := Check(ctx, database, today); err != nil {
		return nil, err
	}
	rows, err := database.QueryContext(ctx, "SELECT id FROM maintenance_notifications WHERE status=0 ORDER BY id")
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) > 0 && sender == nil {
		return nil, errors.New("SMTP sender is required for queued maintenance notifications")
	}
	delivered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if err := deliverNotification(ctx, database, id, sender); err != nil {
			return delivered, err
		}
		delivered = append(delivered, id)
	}
	return delivered, nil
}

func DeliverQueued(ctx context.Context, database *sql.DB, notificationID int64, sender mail.Sender) error {
	if sender == nil {
		return errors.New("SMTP sender is required for queued maintenance notifications")
	}
	return deliverNotification(ctx, database, notificationID, sender)
}

func deliverNotification(ctx context.Context, database *sql.DB, notificationID int64, sender mail.Sender) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		sent, err := deliverAttempt(ctx, database, notificationID, sender)
		if err == nil {
			_ = sent
			return nil
		}
		lastErr = err
		if attempt == 2 {
			if _, updateErr := database.ExecContext(ctx, `UPDATE maintenance_notifications SET status=3,updated_at=now() at time zone 'utc' WHERE id=$1 AND status=0`, notificationID); updateErr != nil {
				return errors.Join(lastErr, updateErr)
			}
			return fmt.Errorf("send maintenance notification %d after three attempts: %w", notificationID, lastErr)
		}
		delay := time.Duration(250*(1<<attempt)) * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func deliverAttempt(ctx context.Context, database *sql.DB, notificationID int64, sender mail.Sender) (bool, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var notificationKind, status, maintenanceKind int
	var vehicleName, email string
	var costDate time.Time
	var days sql.NullInt64
	var kilometres sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT n.notification_kind,n.status,v.name,u.email_address,c.occurred_on,n.days_remaining,n.kilometres_remaining,rr.kind
		FROM maintenance_notifications n
		JOIN maintenance_reminder_rules rr ON rr.id=n.maintenance_reminder_rule_id
		JOIN vehicles v ON v.id=rr.vehicle_id
		JOIN users u ON u.id=v.user_id
		JOIN additional_costs c ON c.id=n.additional_cost_id
		WHERE n.id=$1 FOR UPDATE OF n
	`, notificationID).Scan(&notificationKind, &status, &vehicleName, &email, &costDate, &days, &kilometres, &maintenanceKind)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != 0 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	label := reminderKindLabel(maintenanceKind)
	due := notificationKind == 1
	subject := "Blíži sa údržba vozidla: " + label
	firstLine := "Blíži sa údržba " + label + " pre vozidlo " + vehicleName + "."
	if due {
		subject = "Údržba vozidla je potrebná: " + label
		firstLine = "Údržba " + label + " pre vozidlo " + vehicleName + " je potrebná."
	}
	body := "Dobrý deň,\n\n" + firstLine + "\n\nPosledný zaznamenaný náklad bol " + costDate.Format(time.DateOnly) + ".\n"
	if days.Valid {
		body += fmt.Sprintf("Do termínu zostáva %d dní.\n", days.Int64)
	}
	if kilometres.Valid {
		value, err := decimal.NewFromString(kilometres.String)
		if err != nil {
			return false, err
		}
		body += "Do kilometrového limitu zostáva " + strings.TrimRight(strings.TrimRight(value.StringFixed(2), "0"), ".") + " km.\n"
	}
	body += "\nNaplánujte si prosím vhodný termín.\n"
	if err := sender.Send(email, subject, body); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE maintenance_notifications SET status=1,sent_at=now() at time zone 'utc',updated_at=now() at time zone 'utc' WHERE id=$1`, notificationID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func reminderKindLabel(kind int) string {
	labels := []string{"Výmena oleja", "Výmena palivového filtra", "Technická kontrola", "Emisná kontrola", "Prezutie pneumatík", "Oprava", "Iné"}
	if kind < 0 || kind >= len(labels) {
		return ""
	}
	return labels[kind]
}
