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

// TestUpdatePreferencesErrorChecking verifies that UpdatePreferences returns
// an error (not a panic) when the DB pool is nil, proving write calls are
// error-checked rather than fire-and-forget.
func TestUpdatePreferencesErrorChecking(t *testing.T) {
	svc := &Service{} // nil pool, nil audit
	prefs := []Preference{
		{Channel: "in_app", EventType: "order_created", Enabled: true},
	}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("UpdatePreferences panicked on nil pool — write calls are not error-checked")
			}
		}()
		err = svc.UpdatePreferences(context.Background(), 1, prefs)
	}()
	if err == nil {
		t.Error("expected error from UpdatePreferences with nil pool")
	}
}

// TestProcessExportRetriesErrorChecking verifies that ProcessExportRetries
// returns an error (not a panic) when the DB pool is nil.
func TestProcessExportRetriesErrorChecking(t *testing.T) {
	svc := &Service{exportsDir: "/tmp/test"} // nil pool
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("ProcessExportRetries panicked on nil pool — write calls are not error-checked")
			}
		}()
		_, err = svc.ProcessExportRetries(context.Background())
	}()
	if err == nil {
		t.Error("expected error from ProcessExportRetries with nil pool")
	}
}
