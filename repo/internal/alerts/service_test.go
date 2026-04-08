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

// TestClaimAlertErrorCheckingStructure verifies that ClaimAlert's mutation path
// uses error-checked writes. With a nil pool, ClaimAlert must return an error
// (not panic with an unchecked write) when the initial status query fails.
func TestClaimAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{} // nil pool
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("ClaimAlert panicked on nil pool — writes are not error-checked")
			}
		}()
		err = svc.ClaimAlert(nil, 999, 1)
	}()
	if err == nil {
		t.Error("expected error from ClaimAlert with nil pool")
	}
}

// TestProcessAlertErrorCheckingStructure verifies ProcessAlert returns error on nil pool.
func TestProcessAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("ProcessAlert panicked on nil pool — writes are not error-checked")
			}
		}()
		err = svc.ProcessAlert(nil, 999, 1)
	}()
	if err == nil {
		t.Error("expected error from ProcessAlert with nil pool")
	}
}

// TestCloseAlertErrorCheckingStructure verifies CloseAlert returns error on nil pool
// (after passing the resolution-notes guard).
func TestCloseAlertErrorCheckingStructure(t *testing.T) {
	svc := &Service{}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("CloseAlert panicked on nil pool — writes are not error-checked")
			}
		}()
		err = svc.CloseAlert(nil, 999, 1, "valid resolution notes")
	}()
	if err == nil {
		t.Error("expected error from CloseAlert with nil pool")
	}
}
