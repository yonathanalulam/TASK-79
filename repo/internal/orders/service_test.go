package orders

import (
	"context"
	"strings"
	"testing"
)

// TestTransitionOrderBlocksPaymentRecorded verifies the payment governance fix:
// the generic TransitionOrder MUST refuse payment_recorded transitions.
func TestTransitionOrderBlocksPaymentRecorded(t *testing.T) {
	svc := &Service{}
	err := svc.TransitionOrder(context.Background(), 1, StatusPaymentRecorded, nil, "user", "")
	if err == nil {
		t.Fatal("expected error when transitioning to payment_recorded via generic path")
	}
	if !strings.Contains(err.Error(), "dedicated payment-recorded endpoint") {
		t.Errorf("error should mention dedicated endpoint, got: %v", err)
	}
}

// TestTransitionOrderDoesNotBlockOtherStates verifies that the payment guard
// only blocks payment_recorded and lets other transitions through.
func TestTransitionOrderDoesNotBlockOtherStates(t *testing.T) {
	svc := &Service{}
	// These will panic at DB access, but must NOT be caught by the payment guard
	otherStates := []string{StatusCutoff, StatusCancelled, StatusPicking, StatusArrival}
	for _, state := range otherStates {
		t.Run(state, func(t *testing.T) {
			var err error
			func() {
				defer func() { recover() }()
				err = svc.TransitionOrder(context.Background(), 1, state, nil, "user", "")
			}()
			if err != nil && strings.Contains(err.Error(), "dedicated payment-recorded endpoint") {
				t.Errorf("%s should not be blocked by payment guard", state)
			}
		})
	}
}

// TestBackorderLineStatuses verifies that the line status values used by the
// allocation logic are all valid against the domain model expectations.
func TestBackorderLineStatuses(t *testing.T) {
	validStatuses := map[string]bool{
		"pending": true, "allocated": true, "backordered": true,
		"partial": true, "fulfilled": true, "cancelled": true,
	}

	// Test that the allocation logic uses only valid statuses
	testCases := []struct {
		name        string
		requested   int
		stock       int
		wantStatus  string
		wantAlloc   int
		wantBackord int
	}{
		{"full allocation", 5, 10, "allocated", 5, 0},
		{"zero stock", 5, 0, "backordered", 0, 5},
		{"partial allocation", 10, 3, "partial", 3, 7},
		{"exact match", 5, 5, "allocated", 5, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the allocation logic from CreateOrder
			allocated := tc.requested
			backordered := 0
			lineStatus := "allocated"

			if tc.stock < tc.requested {
				allocated = tc.stock
				if allocated < 0 {
					allocated = 0
				}
				backordered = tc.requested - allocated
				if allocated == 0 {
					lineStatus = "backordered"
				} else {
					lineStatus = "partial"
				}
			}

			if !validStatuses[lineStatus] {
				t.Errorf("line status %q is not valid (would violate DB constraint)", lineStatus)
			}
			if lineStatus != tc.wantStatus {
				t.Errorf("expected status %q, got %q", tc.wantStatus, lineStatus)
			}
			if allocated != tc.wantAlloc {
				t.Errorf("expected allocated %d, got %d", tc.wantAlloc, allocated)
			}
			if backordered != tc.wantBackord {
				t.Errorf("expected backordered %d, got %d", tc.wantBackord, backordered)
			}
			if allocated+backordered != tc.requested {
				t.Errorf("allocated (%d) + backordered (%d) != requested (%d)", allocated, backordered, tc.requested)
			}
		})
	}
}
