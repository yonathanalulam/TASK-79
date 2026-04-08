package orders

import (
	"testing"
)

func TestValidateTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{StatusCreated, StatusPaymentRecorded},
		{StatusCreated, StatusCutoff},
		{StatusCreated, StatusCancelled},
		{StatusPaymentRecorded, StatusPicking},
		{StatusPaymentRecorded, StatusCancelled},
		{StatusCutoff, StatusPicking},
		{StatusCutoff, StatusCancelled},
		{StatusPicking, StatusArrival},
		{StatusPicking, StatusPartiallyBackordered},
		{StatusPicking, StatusCancelled},
		{StatusArrival, StatusPickup},
		{StatusArrival, StatusDelivery},
		{StatusArrival, StatusCancelled},
		{StatusPickup, StatusCompleted},
		{StatusDelivery, StatusCompleted},
		{StatusPartiallyBackordered, StatusSplit},
		{StatusPartiallyBackordered, StatusPicking},
		{StatusPartiallyBackordered, StatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Errorf("expected valid transition from %q to %q, got error: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestValidateTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{StatusCreated, StatusPicking},
		{StatusCreated, StatusCompleted},
		{StatusPaymentRecorded, StatusDelivery},
		{StatusPicking, StatusPaymentRecorded},
		{StatusCompleted, StatusCreated},
		{StatusCancelled, StatusCreated},
		{StatusPickup, StatusPicking},
		{StatusDelivery, StatusPicking},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err == nil {
				t.Errorf("expected invalid transition from %q to %q, got nil", tt.from, tt.to)
			}
		})
	}
}

func TestGetAllowedTransitions(t *testing.T) {
	allowed := GetAllowedTransitions(StatusCreated)
	if len(allowed) != 3 {
		t.Errorf("expected 3 allowed transitions from created, got %d", len(allowed))
	}

	allowed = GetAllowedTransitions(StatusCompleted)
	if len(allowed) != 0 {
		t.Errorf("expected 0 allowed transitions from completed, got %d", len(allowed))
	}
}

func TestIsTerminal(t *testing.T) {
	if !IsTerminal(StatusCompleted) {
		t.Error("completed should be terminal")
	}
	if !IsTerminal(StatusCancelled) {
		t.Error("cancelled should be terminal")
	}
	if !IsTerminal(StatusSplit) {
		t.Error("split should be terminal")
	}
	if IsTerminal(StatusCreated) {
		t.Error("created should not be terminal")
	}
	if IsTerminal(StatusPicking) {
		t.Error("picking should not be terminal")
	}
}

func TestAutoCutoffRule(t *testing.T) {
	if err := ValidateTransition(StatusCreated, StatusCutoff); err != nil {
		t.Errorf("cutoff should be allowed from created: %v", err)
	}
	if err := ValidateTransition(StatusCutoff, StatusPicking); err != nil {
		t.Errorf("picking should be allowed from cutoff: %v", err)
	}
}

// Test that the payment_recorded transition is structurally valid in the state machine
// (the SERVICE blocks it from the generic transition path, but the state machine itself allows it)
func TestPaymentRecordedIsValidState(t *testing.T) {
	// State machine allows created -> payment_recorded
	if err := ValidateTransition(StatusCreated, StatusPaymentRecorded); err != nil {
		t.Errorf("state machine should allow created -> payment_recorded: %v", err)
	}
	// But NOT from other states
	if err := ValidateTransition(StatusPicking, StatusPaymentRecorded); err == nil {
		t.Error("state machine should NOT allow picking -> payment_recorded")
	}
}

// Test backorder-related states
func TestPartiallyBackorderedTransitions(t *testing.T) {
	allowed := GetAllowedTransitions(StatusPartiallyBackordered)
	if len(allowed) != 3 {
		t.Errorf("expected 3 transitions from partially_backordered, got %d: %v", len(allowed), allowed)
	}
	// Must include split, picking, cancelled
	expected := map[string]bool{StatusSplit: true, StatusPicking: true, StatusCancelled: true}
	for _, s := range allowed {
		if !expected[s] {
			t.Errorf("unexpected transition from partially_backordered: %s", s)
		}
	}
}
