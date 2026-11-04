package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"spotreba/server/internal/maintenance"
)

func TestReminderCheckPersistsOneAdvanceNotificationAndDoesNotRepeat(t *testing.T) {
	_, db := newTestServer(t)
	today := dateAtUTC(time.Now())
	userID := insertTestUser(t, db, "reminders-check@example.com")
	vehicleID := insertTestVehicle(t, db, userID, "Service car")
	baseline := today.AddDate(0, 0, -336)
	ruleID := insertTestReminderRule(t, db, vehicleID, 365, "15000", true, "oil_change")
	dayLead := insertTestReminderLead(t, db, ruleID, intPointer(30), nil)
	insertTestReminderLead(t, db, ruleID, nil, stringPointer("1000"))
	costID := insertTestCost(t, db, vehicleID, baseline, "oil_change")

	ids, err := maintenance.Check(context.Background(), db, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("queued notification ids = %v, want one", ids)
	}
	var kind, condition, status int
	var leadID sql.NullInt64
	var days sql.NullInt64
	if err := db.QueryRow(`SELECT notification_kind,trigger_condition,status,maintenance_reminder_lead_id,days_remaining
		FROM maintenance_notifications WHERE id=$1`, ids[0]).Scan(&kind, &condition, &status, &leadID, &days); err != nil {
		t.Fatal(err)
	}
	if kind != 0 || condition != 0 || status != 0 || !leadID.Valid || leadID.Int64 != dayLead || !days.Valid || days.Int64 != 29 {
		t.Fatalf("notification state kind=%d condition=%d status=%d lead=%+v days=%+v", kind, condition, status, leadID, days)
	}
	var linkedCost int64
	if err := db.QueryRow("SELECT additional_cost_id FROM maintenance_notifications WHERE id=$1", ids[0]).Scan(&linkedCost); err != nil {
		t.Fatal(err)
	}
	if linkedCost != costID {
		t.Fatalf("notification cost = %d, want %d", linkedCost, costID)
	}
	again, err := maintenance.Check(context.Background(), db, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second check queued %v, want none", again)
	}
}

func TestReminderCheckSkipsQueuedAdvancesWhenDue(t *testing.T) {
	_, db := newTestServer(t)
	today := dateAtUTC(time.Now())
	userID := insertTestUser(t, db, "reminders-due@example.com")
	vehicleID := insertTestVehicle(t, db, userID, "Due car")
	baseline := today.AddDate(0, 0, -30)
	ruleID := insertTestReminderRule(t, db, vehicleID, 30, "1000", true, "oil_change")
	dayLead := insertTestReminderLead(t, db, ruleID, intPointer(7), nil)
	kmLead := insertTestReminderLead(t, db, ruleID, nil, stringPointer("100"))
	costID := insertTestCost(t, db, vehicleID, baseline, "oil_change")
	queuedAdvance := insertTestNotification(t, db, ruleID, dayLead, costID, 0, 0, 0, intPointer(7), stringPointer("1000"))
	if _, err := db.Exec(`INSERT INTO refuelings (vehicle_id,refueled_on,distance_km,amount,cost,created_at,updated_at)
		VALUES ($1,$2::date + 1,1200,80,100,now(),now())`, vehicleID, baseline.Format(time.DateOnly)); err != nil {
		t.Fatal(err)
	}

	queued, err := maintenance.Check(context.Background(), db, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 {
		t.Fatalf("due check queued ids = %v", queued)
	}
	var dueKind, dueCondition, dueStatus int
	var dueDays sql.NullInt64
	if err := db.QueryRow("SELECT notification_kind,trigger_condition,status,days_remaining FROM maintenance_notifications WHERE id=$1", queued[0]).Scan(&dueKind, &dueCondition, &dueStatus, &dueDays); err != nil {
		t.Fatal(err)
	}
	if dueKind != 1 || dueCondition != 0 || dueStatus != 0 || !dueDays.Valid || dueDays.Int64 != 0 {
		t.Fatalf("due notification = kind %d condition %d status %d days %+v", dueKind, dueCondition, dueStatus, dueDays)
	}
	var advanceStatus int
	if err := db.QueryRow("SELECT status FROM maintenance_notifications WHERE id=$1", queuedAdvance).Scan(&advanceStatus); err != nil {
		t.Fatal(err)
	}
	if advanceStatus != 2 {
		t.Fatalf("queued advance status = %d, want skipped", advanceStatus)
	}
	if err := db.QueryRow("SELECT status FROM maintenance_notifications WHERE maintenance_reminder_lead_id=$1", kmLead).Scan(&advanceStatus); err != nil {
		t.Fatal(err)
	}
	if advanceStatus != 2 {
		t.Fatalf("missing advance status = %d, want skipped", advanceStatus)
	}
}

func TestReminderCheckAndDeliverSendsOnceAndMarksNotificationSent(t *testing.T) {
	_, db := newTestServer(t)
	today := dateAtUTC(time.Now())
	userID := insertTestUser(t, db, "reminders-mail@example.com")
	vehicleID := insertTestVehicle(t, db, userID, "Mail car")
	baseline := today.AddDate(0, 0, -336)
	ruleID := insertTestReminderRule(t, db, vehicleID, 365, "15000", true, "oil_change")
	insertTestReminderLead(t, db, ruleID, intPointer(30), nil)
	insertTestCost(t, db, vehicleID, baseline, "oil_change")
	mailer := &recordingMailer{}

	ids, err := maintenance.CheckAndDeliver(context.Background(), db, today, mailer)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || len(mailer.messages) != 1 {
		t.Fatalf("delivered ids=%v messages=%+v", ids, mailer.messages)
	}
	message := mailer.messages[0]
	if message.to != "reminders-mail@example.com" || message.subject != "Blíži sa údržba vozidla: Výmena oleja" || !strings.Contains(message.body, "Blíži sa údržba Výmena oleja pre vozidlo Mail car.") || !strings.Contains(message.body, "Do termínu zostáva 29 dní.") {
		t.Fatalf("maintenance message = %+v", message)
	}
	var status int
	var sentAt sql.NullTime
	if err := db.QueryRow("SELECT status,sent_at FROM maintenance_notifications WHERE id=$1", ids[0]).Scan(&status, &sentAt); err != nil {
		t.Fatal(err)
	}
	if status != 1 || !sentAt.Valid {
		t.Fatalf("sent notification state = status %d, sent_at %+v", status, sentAt)
	}
	if _, err := maintenance.CheckAndDeliver(context.Background(), db, today, mailer); err != nil {
		t.Fatal(err)
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("repeat check sent %d emails, want 1", len(mailer.messages))
	}
}

type failingReminderMailer struct{ calls int }

func (m *failingReminderMailer) Send(string, string, string) error {
	m.calls++
	return errors.New("smtp unavailable")
}

func TestReminderDeliveryRetriesAndMarksFailedAfterThreeAttempts(t *testing.T) {
	_, db := newTestServer(t)
	today := dateAtUTC(time.Now())
	userID := insertTestUser(t, db, "reminders-failure@example.com")
	vehicleID := insertTestVehicle(t, db, userID, "Failure car")
	baseline := today.AddDate(0, 0, -336)
	ruleID := insertTestReminderRule(t, db, vehicleID, 365, "15000", true, "oil_change")
	insertTestReminderLead(t, db, ruleID, intPointer(30), nil)
	insertTestCost(t, db, vehicleID, baseline, "oil_change")
	ids, err := maintenance.Check(context.Background(), db, today)
	if err != nil || len(ids) != 1 {
		t.Fatalf("check ids=%v err=%v", ids, err)
	}
	mailer := &failingReminderMailer{}
	err = maintenance.DeliverQueued(context.Background(), db, ids[0], mailer)
	if err == nil || mailer.calls != 3 {
		t.Fatalf("delivery error=%v calls=%d, want an error after 3 tries", err, mailer.calls)
	}
	var status int
	if err := db.QueryRow("SELECT status FROM maintenance_notifications WHERE id=$1", ids[0]).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != 3 {
		t.Fatalf("notification status=%d, want failed (3)", status)
	}
}

func dateAtUTC(value time.Time) time.Time {
	parsed, err := time.Parse(time.DateOnly, value.UTC().Format(time.DateOnly))
	if err != nil {
		panic(err)
	}
	return parsed
}

func insertTestUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO users (email_address,password_digest,created_at,updated_at) VALUES ($1,'digest',now(),now()) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestVehicle(t *testing.T, db *sql.DB, userID int64, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO vehicles (user_id,name,fuel_type,created_at,updated_at) VALUES ($1,$2,0,now(),now()) RETURNING id`, userID, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestReminderRule(t *testing.T, db *sql.DB, vehicleID int64, days int, km string, active bool, kind string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_rules (vehicle_id,kind,interval_days,interval_km,active,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,now(),now()) RETURNING id`, vehicleID, costKinds[kind], days, km, active).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestReminderLead(t *testing.T, db *sql.DB, ruleID int64, days *int, km *string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO maintenance_reminder_leads (maintenance_reminder_rule_id,days_before,kilometres_before,created_at,updated_at)
		VALUES ($1,$2,$3,now(),now()) RETURNING id`, ruleID, days, km).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestCost(t *testing.T, db *sql.DB, vehicleID int64, date time.Time, kind string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO additional_costs (vehicle_id,occurred_on,kind,cost,created_at,updated_at)
		VALUES ($1,$2,$3,50,now(),now()) RETURNING id`, vehicleID, date.Format(time.DateOnly), costKinds[kind]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestNotification(t *testing.T, db *sql.DB, ruleID, leadID, costID int64, kind, condition, status int, days *int, km *string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO maintenance_notifications (maintenance_reminder_rule_id,maintenance_reminder_lead_id,additional_cost_id,notification_kind,trigger_condition,status,days_remaining,kilometres_remaining,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now()) RETURNING id`, ruleID, leadID, costID, kind, condition, status, days, km).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func intPointer(value int) *int { return &value }
