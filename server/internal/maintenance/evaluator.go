package maintenance

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type Condition string

type NotificationKind string

type NotificationStatus string

const (
	ConditionDate       Condition          = "date"
	ConditionMileage    Condition          = "mileage"
	NotificationAdvance NotificationKind   = "advance"
	NotificationDue     NotificationKind   = "due"
	StatusQueued        NotificationStatus = "queued"
	StatusSent          NotificationStatus = "sent"
	StatusSkipped       NotificationStatus = "skipped"
	StatusFailed        NotificationStatus = "failed"
)

type Rule struct {
	IntervalDays *int
	IntervalKM   *decimal.Decimal
}

type Lead struct {
	ID               int64
	DaysBefore       *int
	KilometresBefore *decimal.Decimal
}

type Notification struct {
	ID        int64
	LeadID    *int64
	Kind      NotificationKind
	Status    NotificationStatus
	Condition Condition
}

type RefuelingDistance struct {
	Date     time.Time
	Distance decimal.Decimal
}

type Cycle struct {
	DaysRemaining       *int
	KilometresRemaining *decimal.Decimal
}

type PlannedNotification struct {
	LeadID              *int64
	Kind                NotificationKind
	Condition           Condition
	Status              NotificationStatus
	DaysRemaining       *int
	KilometresRemaining *decimal.Decimal
}

type Decision struct {
	Queued      *PlannedNotification
	Skipped     []PlannedNotification
	MarkSkipped []int64
}

type candidate struct {
	lead      Lead
	condition Condition
	progress  decimal.Decimal
	threshold decimal.Decimal
}

func CalculateCycle(baseline, today time.Time, intervalDays *int, intervalKM *decimal.Decimal, refuelings []RefuelingDistance) Cycle {
	travelled := decimal.Zero
	for _, refueling := range refuelings {
		if dateOnly(refueling.Date).After(dateOnly(baseline)) && !dateOnly(refueling.Date).After(dateOnly(today)) {
			travelled = travelled.Add(refueling.Distance)
		}
	}
	return CalculateCycleFromDistance(baseline, today, intervalDays, intervalKM, travelled)
}

func CalculateCycleFromDistance(baseline, today time.Time, intervalDays *int, intervalKM *decimal.Decimal, travelled decimal.Decimal) Cycle {
	remaining := Cycle{}
	if intervalDays != nil {
		value := *intervalDays - calendarDays(baseline, today)
		remaining.DaysRemaining = &value
	}
	if intervalKM != nil {
		value := intervalKM.Sub(travelled)
		remaining.KilometresRemaining = &value
	}
	return remaining
}

func Evaluate(rule Rule, cycle Cycle, leads []Lead, existing []Notification) Decision {
	decision := Decision{Skipped: make([]PlannedNotification, 0), MarkSkipped: make([]int64, 0)}
	if cycle.DaysRemaining == nil && cycle.KilometresRemaining == nil {
		return decision
	}
	due := (cycle.DaysRemaining != nil && *cycle.DaysRemaining <= 0) ||
		(cycle.KilometresRemaining != nil && cycle.KilometresRemaining.Cmp(decimal.Zero) <= 0)
	advanceLeadIDs := make(map[int64]bool)
	dueExists := false
	for _, notification := range existing {
		if notification.Kind == NotificationAdvance && notification.LeadID != nil {
			advanceLeadIDs[*notification.LeadID] = true
		}
		if notification.Kind == NotificationDue {
			dueExists = true
		}
	}
	if due {
		if dueExists {
			return decision
		}
		for _, notification := range existing {
			if notification.Kind == NotificationAdvance && notification.Status == StatusQueued {
				decision.MarkSkipped = append(decision.MarkSkipped, notification.ID)
			}
		}
		for _, lead := range leads {
			if advanceLeadIDs[lead.ID] {
				continue
			}
			condition := ConditionMileage
			if lead.DaysBefore != nil {
				condition = ConditionDate
			}
			decision.Skipped = append(decision.Skipped, planned(lead.ID, NotificationAdvance, condition, StatusSkipped, cycle))
		}
		condition := ConditionMileage
		if cycle.DaysRemaining != nil && *cycle.DaysRemaining <= 0 {
			condition = ConditionDate
		}
		decision.Queued = pointer(planned(0, NotificationDue, condition, StatusQueued, cycle))
		decision.Queued.LeadID = nil
		return decision
	}

	candidates := make([]candidate, 0)
	for _, lead := range leads {
		if advanceLeadIDs[lead.ID] {
			continue
		}
		if lead.DaysBefore != nil && cycle.DaysRemaining != nil && rule.IntervalDays != nil && *cycle.DaysRemaining <= *lead.DaysBefore {
			remaining := decimal.NewFromInt(int64(*cycle.DaysRemaining))
			interval := decimal.NewFromInt(int64(*rule.IntervalDays))
			candidates = append(candidates, candidate{lead: lead, condition: ConditionDate, progress: remaining.Div(interval), threshold: decimal.NewFromInt(int64(*lead.DaysBefore))})
		} else if lead.KilometresBefore != nil && cycle.KilometresRemaining != nil && rule.IntervalKM != nil && cycle.KilometresRemaining.Cmp(*lead.KilometresBefore) <= 0 {
			candidates = append(candidates, candidate{lead: lead, condition: ConditionMileage, progress: cycle.KilometresRemaining.Div(*rule.IntervalKM), threshold: *lead.KilometresBefore})
		}
	}
	if len(candidates) == 0 {
		return decision
	}
	sort.Slice(candidates, func(i, j int) bool {
		if comparison := candidates[i].progress.Cmp(candidates[j].progress); comparison != 0 {
			return comparison < 0
		}
		if comparison := candidates[i].threshold.Cmp(candidates[j].threshold); comparison != 0 {
			return comparison < 0
		}
		return candidates[i].lead.ID < candidates[j].lead.ID
	})
	selected := candidates[0]
	for _, skipped := range candidates[1:] {
		decision.Skipped = append(decision.Skipped, planned(skipped.lead.ID, NotificationAdvance, skipped.condition, StatusSkipped, cycle))
	}
	decision.Queued = pointer(planned(selected.lead.ID, NotificationAdvance, selected.condition, StatusQueued, cycle))
	return decision
}

func planned(leadID int64, kind NotificationKind, condition Condition, status NotificationStatus, cycle Cycle) PlannedNotification {
	var id *int64
	if leadID != 0 {
		id = &leadID
	}
	var km *decimal.Decimal
	if cycle.KilometresRemaining != nil {
		value := *cycle.KilometresRemaining
		km = &value
	}
	var days *int
	if cycle.DaysRemaining != nil {
		value := *cycle.DaysRemaining
		days = &value
	}
	return PlannedNotification{LeadID: id, Kind: kind, Condition: condition, Status: status, DaysRemaining: days, KilometresRemaining: km}
}

func pointer[T any](value T) *T { return &value }

func calendarDays(from, to time.Time) int {
	return int(dateOnly(to).Sub(dateOnly(from)).Hours() / 24)
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
