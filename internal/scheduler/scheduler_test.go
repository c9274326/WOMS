package scheduler

import (
	"testing"
	"time"

	"github.com/c9274326/woms/internal/domain"
)

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}

func TestPlanSplitsOrderAcrossDays(t *testing.T) {
	result, err := Plan(Request{
		LineID:         "A",
		CapacityPerDay: 10000,
		StartDate:      mustDate(t, "2026-05-01"),
		Orders: []OrderInput{{
			ID:       "ORD-1",
			LineID:   "A",
			Quantity: 25000,
			Priority: domain.PriorityLow,
			DueDate:  mustDate(t, "2026-05-03"),
		}},
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(result.Allocations) != 3 {
		t.Fatalf("expected 3 allocations, got %d", len(result.Allocations))
	}
	if result.Allocations[0].Quantity != 10000 || result.Allocations[1].Quantity != 10000 || result.Allocations[2].Quantity != 5000 {
		t.Fatalf("unexpected allocations: %+v", result.Allocations)
	}
	if !result.FinishDate.Equal(mustDate(t, "2026-05-03")) {
		t.Fatalf("unexpected finish date: %s", result.FinishDate)
	}
}

func TestPlanUsesEarliestAvailableDatesBeforeDueDate(t *testing.T) {
	result, err := Plan(Request{
		LineID:         "A",
		CapacityPerDay: 10000,
		StartDate:      mustDate(t, "2026-04-30"),
		Orders: []OrderInput{{
			ID:       "ORD-EARLY",
			LineID:   "A",
			Quantity: 20000,
			Priority: domain.PriorityLow,
			DueDate:  mustDate(t, "2026-05-02"),
		}},
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(result.Allocations) != 2 {
		t.Fatalf("expected 2 allocations, got %+v", result.Allocations)
	}
	if !result.Allocations[0].Date.Equal(mustDate(t, "2026-04-30")) || !result.Allocations[1].Date.Equal(mustDate(t, "2026-05-01")) {
		t.Fatalf("expected earliest available dates before due date, got %+v", result.Allocations)
	}
	if !result.FinishDate.Equal(mustDate(t, "2026-05-01")) {
		t.Fatalf("unexpected finish date: %s", result.FinishDate)
	}
}

func TestPlanDoesNotMoveExistingHighPriorityAllocations(t *testing.T) {
	result, err := Plan(Request{
		LineID:         "A",
		CapacityPerDay: 10000,
		StartDate:      mustDate(t, "2026-05-01"),
		ExistingAllocations: []ExistingAllocation{{
			OrderID:  "HIGH-LOCKED",
			LineID:   "A",
			Date:     mustDate(t, "2026-05-01"),
			Quantity: 9000,
			Priority: domain.PriorityHigh,
			Locked:   true,
		}},
		Orders: []OrderInput{{
			ID:       "NEW-HIGH",
			LineID:   "A",
			Quantity: 2000,
			Priority: domain.PriorityHigh,
			DueDate:  mustDate(t, "2026-05-02"),
		}},
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(result.Allocations) != 2 {
		t.Fatalf("expected two allocations around locked high priority capacity, got %+v", result.Allocations)
	}
	if result.Allocations[0].Date.Equal(mustDate(t, "2026-05-01")) && result.Allocations[0].Quantity > 1000 {
		t.Fatalf("allocated through locked high priority capacity: %+v", result.Allocations)
	}
}

func TestHighPriorityOrderReportsLowPriorityDisplacement(t *testing.T) {
	result, err := Plan(Request{
		LineID:         "A",
		CapacityPerDay: 10000,
		StartDate:      mustDate(t, "2026-05-01"),
		ExistingAllocations: []ExistingAllocation{{
			OrderID:  "LOW-1",
			LineID:   "A",
			Date:     mustDate(t, "2026-05-01"),
			Quantity: 9000,
			Priority: domain.PriorityLow,
		}},
		Orders: []OrderInput{{
			ID:       "HIGH-1",
			LineID:   "A",
			Quantity: 5000,
			Priority: domain.PriorityHigh,
			DueDate:  mustDate(t, "2026-05-01"),
		}},
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(result.Allocations) != 1 || result.Allocations[0].Date != mustDate(t, "2026-05-01") {
		t.Fatalf("expected high-priority allocation on first day, got %+v", result.Allocations)
	}
	if len(result.Conflicts) == 0 || len(result.Conflicts[0].AffectedOrderIDs) != 1 || result.Conflicts[0].AffectedOrderIDs[0] != "LOW-1" {
		t.Fatalf("expected affected low-priority order conflict, got %+v", result.Conflicts)
	}
}

func TestPlanReportsEarliestFinishWhenDueDateCannotBeMet(t *testing.T) {
	result, err := Plan(Request{
		LineID:         "A",
		CapacityPerDay: 10000,
		StartDate:      mustDate(t, "2026-05-01"),
		Orders: []OrderInput{{
			ID:       "ORD-LATE",
			LineID:   "A",
			Quantity: 25000,
			Priority: domain.PriorityLow,
			DueDate:  mustDate(t, "2026-05-02"),
		}},
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected one conflict, got %+v", result.Conflicts)
	}
	if !result.Conflicts[0].EarliestFinishDate.Equal(mustDate(t, "2026-05-03")) {
		t.Fatalf("unexpected earliest finish date: %s", result.Conflicts[0].EarliestFinishDate)
	}
}

func TestConfirmProductionCreatesPendingRemainder(t *testing.T) {
	now := mustDate(t, "2026-05-01")
	result, err := ConfirmProduction(domain.Order{
		ID:       "ORD-1",
		LineID:   "A",
		Quantity: 2500,
		Priority: domain.PriorityLow,
		Status:   domain.StatusInProgress,
		DueDate:  mustDate(t, "2026-05-03"),
	}, 1500, now)
	if err != nil {
		t.Fatalf("ConfirmProduction returned error: %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result")
	}
	if result.Remainder == nil || result.Remainder.Quantity != 1000 || result.Remainder.Status != domain.StatusPending || result.Remainder.SourceOrder != "ORD-1" {
		t.Fatalf("unexpected remainder: %+v", result.Remainder)
	}
}
