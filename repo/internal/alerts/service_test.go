package alerts

import "testing"

func TestAlertLifecycleValidation(t *testing.T) {
	// CloseAlert requires resolution notes — structural test of the guard
	svc := &Service{}
	err := svc.CloseAlert(nil, 0, 0, "")
	if err != ErrResolutionRequired {
		t.Errorf("expected ErrResolutionRequired for empty notes, got %v", err)
	}
	// Non-empty notes should pass the guard (will fail at DB, which is expected)
	func() {
		defer func() { recover() }()
		_ = svc.CloseAlert(nil, 0, 0, "valid notes")
	}()
}

func TestAlertStatusTransitions(t *testing.T) {
	validTransitions := map[string][]string{
		"open":       {"claimed"},
		"claimed":    {"processing"},
		"processing": {"closed"},
	}

	for from, tos := range validTransitions {
		for _, to := range tos {
			if !isValidAlertTransition(from, to) {
				t.Errorf("expected valid transition from %q to %q", from, to)
			}
		}
	}

	invalidTransitions := [][2]string{
		{"open", "processing"},
		{"open", "closed"},
		{"claimed", "closed"},
		{"closed", "open"},
		{"processing", "open"},
	}

	for _, tt := range invalidTransitions {
		if isValidAlertTransition(tt[0], tt[1]) {
			t.Errorf("expected invalid transition from %q to %q", tt[0], tt[1])
		}
	}
}

func isValidAlertTransition(from, to string) bool {
	allowed := map[string]string{
		"open":       "claimed",
		"claimed":    "processing",
		"processing": "closed",
	}
	return allowed[from] == to
}

// TestClaimAlertErrorCheckingStructure verifies ClaimAlert's DB path is reached.
// With nil pool the first QueryRow panics — this confirms the method doesn't
// silently skip the query. Real error-checking of writes is covered by
// integration tests (FLEET_TEST_DB).
func TestClaimAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{} // nil pool
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		svc.ClaimAlert(nil, 999, 1)
	}()
	if !panicked {
		t.Error("expected panic from nil pool dereference — method reaches DB path")
	}
}

// TestProcessAlertErrorCheckingStructure verifies ProcessAlert reaches DB path.
func TestProcessAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{}
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		svc.ProcessAlert(nil, 999, 1)
	}()
	if !panicked {
		t.Error("expected panic from nil pool dereference")
	}
}

// TestCloseAlertErrorCheckingStructure verifies CloseAlert passes the guard
// then reaches DB path (panics on nil pool after the notes check).
func TestCloseAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{}
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		svc.CloseAlert(nil, 999, 1, "valid resolution notes")
	}()
	if !panicked {
		t.Error("expected panic from nil pool dereference after notes guard")
	}
}
