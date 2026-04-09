package notifications

import (
	"context"
	"testing"
)

// TestUpdatePreferencesRejectsEmpty ensures that calling UpdatePreferences
// with an empty slice does not silently delete all user preferences.
func TestUpdatePreferencesRejectsEmpty(t *testing.T) {
	svc := &Service{} // nil pool — we test the guard before DB access
	err := svc.UpdatePreferences(context.Background(), 1, nil)
	if err == nil {
		t.Fatal("expected error when updating with nil preferences")
	}

	err = svc.UpdatePreferences(context.Background(), 1, []Preference{})
	if err == nil {
		t.Fatal("expected error when updating with empty preferences")
	}
}

// TestUpdatePreferencesErrorChecking verifies that UpdatePreferences reaches
// the DB path. With nil pool, the first query panics — confirming the method
// doesn't silently skip writes. Real durability is tested via FLEET_TEST_DB.
func TestUpdatePreferencesErrorChecking(t *testing.T) {
	svc := &Service{} // nil pool
	prefs := []Preference{
		{Channel: "in_app", EventType: "order_created", Enabled: true},
	}
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		svc.UpdatePreferences(context.Background(), 1, prefs)
	}()
	if !panicked {
		t.Error("expected panic from nil pool — method reaches DB path")
	}
}

// TestProcessExportRetriesErrorChecking verifies ProcessExportRetries reaches DB path.
func TestProcessExportRetriesErrorChecking(t *testing.T) {
	svc := &Service{exportsDir: "/tmp/test"} // nil pool
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		svc.ProcessExportRetries(context.Background())
	}()
	if !panicked {
		t.Error("expected panic from nil pool — method reaches DB path")
	}
}
