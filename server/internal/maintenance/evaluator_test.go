package maintenance

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCalculateCycleUsesCalendarDaysAndExcludesServiceDateRefueling(t *testing.T) {
	baseline := date("2026-01-01")
	intervalDays := 365
	intervalKM := decimalPtr("15000")
	cycle := CalculateCycle(baseline, date("2026-12-27"), &intervalDays, intervalKM, []RefuelingDistance{
		{Date: date("2026-01-01"), Distance: decimal.RequireFromString("15000")},
		{Date: date("2026-01-02"), Distance: decimal.RequireFromString("10000")},
		{Date: date("2026-12-27"), Distance: decimal.RequireFromString("250")},
	})
	if cycle.DaysRemaining == nil || *cycle.DaysRemaining != 5 || cycle.KilometresRemaining == nil || !cycle.KilometresRemaining.Equal(decimal.RequireFromString("4750")) {
		t.Fatalf("cycle = %+v", cycle)
	}
}

func TestAdvanceNotificationChoosesMostUrgentEligibleLeadOnce(t *testing.T) {
	rule := Rule{IntervalDays: intPtr(365), IntervalKM: decimalPtr("15000")}
	leads := []Lead{{ID: 1, DaysBefore: intPtr(30)}, {ID: 2, KilometresBefore: decimalPtr("1000")}, {ID: 3, DaysBefore: intPtr(7)}}
	cycle := Cycle{DaysRemaining: intPtr(6), KilometresRemaining: decimalPtr("900")}
	decision := Evaluate(rule, cycle, leads, nil)
	if decision.Queued == nil || decision.Queued.LeadID == nil || *decision.Queued.LeadID != 3 || decision.Queued.Condition != ConditionDate || len(decision.Skipped) != 2 {
		t.Fatalf("advance decision = %+v", decision)
	}
	alreadySent := []Notification{{LeadID: intPtr64(1), Kind: NotificationAdvance, Status: StatusSent}}
	second := Evaluate(rule, Cycle{DaysRemaining: intPtr(5), KilometresRemaining: decimalPtr("800")}, leads[:1], alreadySent)
	if second.Queued != nil || len(second.Skipped) != 0 {
		t.Fatalf("repeat decision = %+v", second)
	}
}

func TestDueNotificationSkipsQueuedAndMissingLeadsThenQueuesDueOnce(t *testing.T) {
	rule := Rule{IntervalDays: intPtr(30), IntervalKM: decimalPtr("1000")}
	leads := []Lead{{ID: 1, DaysBefore: intPtr(7)}, {ID: 2, KilometresBefore: decimalPtr("100")}}
	cycle := Cycle{DaysRemaining: intPtr(0), KilometresRemaining: decimalPtr("-20")}
	existing := []Notification{{ID: 10, LeadID: intPtr64(1), Kind: NotificationAdvance, Status: StatusQueued}}
	decision := Evaluate(rule, cycle, leads, existing)
	if decision.Queued == nil || decision.Queued.Kind != NotificationDue || decision.Queued.Condition != ConditionDate {
		t.Fatalf("due decision = %+v", decision)
	}
	if len(decision.MarkSkipped) != 1 || decision.MarkSkipped[0] != 10 || len(decision.Skipped) != 1 || decision.Skipped[0].LeadID == nil || *decision.Skipped[0].LeadID != 2 {
		t.Fatalf("due skipped state = %+v", decision)
	}
	existing = append(existing, Notification{Kind: NotificationDue, Status: StatusQueued})
	if repeat := Evaluate(rule, cycle, leads, existing); repeat.Queued != nil || len(repeat.Skipped) != 0 || len(repeat.MarkSkipped) != 0 {
		t.Fatalf("existing due decision = %+v", repeat)
	}
}

func TestDueMileageUsesMileageConditionWhenDateIsNotDue(t *testing.T) {
	rule := Rule{IntervalKM: decimalPtr("1000")}
	decision := Evaluate(rule, Cycle{KilometresRemaining: decimalPtr("0")}, nil, nil)
	if decision.Queued == nil || decision.Queued.Kind != NotificationDue || decision.Queued.Condition != ConditionMileage {
		t.Fatalf("mileage due decision = %+v", decision)
	}
}

func TestNoBaselineOrNoEligibleLeadDoesNotNotify(t *testing.T) {
	rule := Rule{IntervalDays: intPtr(180)}
	if got := Evaluate(rule, Cycle{}, []Lead{{ID: 1, DaysBefore: intPtr(30)}}, nil); got.Queued != nil || len(got.Skipped) != 0 {
		t.Fatalf("empty cycle decision = %+v", got)
	}
	if got := Evaluate(rule, Cycle{DaysRemaining: intPtr(100)}, []Lead{{ID: 1, DaysBefore: intPtr(30)}}, nil); got.Queued != nil {
		t.Fatalf("early cycle decision = %+v", got)
	}
}

func date(value string) time.Time {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func intPtr(value int) *int       { return &value }
func intPtr64(value int64) *int64 { return &value }
func decimalPtr(value string) *decimal.Decimal {
	parsed := decimal.RequireFromString(value)
	return &parsed
}
