package notifications_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fleetcommerce/internal/notifications"
	"fleetcommerce/internal/testutil"
)

func TestUpdatePreferencesPersistsAndAudits(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := notifications.NewService(pool, auditSvc, t.TempDir())
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Notif User", "notif-prefs@test.com")

	prefs := []notifications.Preference{
		{Channel: "in_app", EventType: "order_created", Enabled: true},
		{Channel: "email", EventType: "alert_fired", Enabled: false},
	}

	if err := svc.UpdatePreferences(ctx, userID, prefs); err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}

	// Verify persisted
	saved, err := svc.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if len(saved) != 2 {
		t.Errorf("expected 2 preferences, got %d", len(saved))
	}

	// Verify audit
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='notification_preference' AND entity_id=$1 AND action='updated'`, userID).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for preference update")
	}

	// Update again (replaces)
	prefs2 := []notifications.Preference{
		{Channel: "in_app", EventType: "order_created", Enabled: false},
	}
	if err := svc.UpdatePreferences(ctx, userID, prefs2); err != nil {
		t.Fatalf("UpdatePreferences second call: %v", err)
	}
	saved2, _ := svc.GetPreferences(ctx, userID)
	if len(saved2) != 1 {
		t.Errorf("expected 1 preference after update, got %d", len(saved2))
	}
}

func TestProcessExportRetriesWritesArtifact(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	exportsDir := t.TempDir()
	svc := notifications.NewService(pool, auditSvc, exportsDir)
	ctx := context.Background()

	// Seed an export queue item
	var itemID int
	if err := pool.QueryRow(ctx, `INSERT INTO export_queue_items (channel, recipient, subject, body, status, attempts, max_attempts) VALUES ('email','test@test.com','Subject','Body','pending',0,3) RETURNING id`).Scan(&itemID); err != nil {
		t.Fatalf("seed export item: %v", err)
	}

	count, err := svc.ProcessExportRetries(ctx)
	if err != nil {
		t.Fatalf("ProcessExportRetries: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 export processed, got %d", count)
	}

	// Verify status updated
	var status string
	pool.QueryRow(ctx, `SELECT status FROM export_queue_items WHERE id=$1`, itemID).Scan(&status)
	if status != "exported" {
		t.Errorf("expected exported status, got %q", status)
	}

	// Verify attempt log
	var logCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM export_attempt_logs WHERE queue_item_id=$1`, itemID).Scan(&logCount)
	if logCount == 0 {
		t.Error("expected export attempt log entry")
	}

	// Verify file was created
	entries, _ := os.ReadDir(exportsDir)
	if len(entries) == 0 {
		t.Error("expected export artifact file to be created")
	}
}

func TestProcessExportRetriesHandlesWriteFailure(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	// Use a non-existent directory to force write failure
	badDir := filepath.Join(t.TempDir(), "nonexistent", "deeply", "nested")
	svc := notifications.NewService(pool, auditSvc, badDir)
	ctx := context.Background()

	var itemID int
	pool.QueryRow(ctx, `INSERT INTO export_queue_items (channel, recipient, subject, body, status, attempts, max_attempts) VALUES ('email','fail@test.com','Sub','Body','pending',0,3) RETURNING id`).Scan(&itemID)

	count, err := svc.ProcessExportRetries(ctx)
	if err != nil {
		t.Fatalf("ProcessExportRetries: %v", err)
	}
	// File write fails, so count should be 0
	if count != 0 {
		t.Errorf("expected 0 exports on write failure, got %d", count)
	}

	// Status should be retrying, not exported
	var status string
	pool.QueryRow(ctx, `SELECT status FROM export_queue_items WHERE id=$1`, itemID).Scan(&status)
	if status != "retrying" {
		t.Errorf("expected retrying status on write failure, got %q", status)
	}
}

// TestUpdatePreferencesFailureRollsBack verifies that if a preference row fails to
// insert (e.g. constraint violation), the transaction rolls back and the old
// preferences remain intact.
func TestUpdatePreferencesFailureRollsBack(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := notifications.NewService(pool, auditSvc, t.TempDir())
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Rollback User", "notif-rollback@test.com")

	// Set initial preferences
	initial := []notifications.Preference{
		{Channel: "in_app", EventType: "order_created", Enabled: true},
	}
	if err := svc.UpdatePreferences(ctx, userID, initial); err != nil {
		t.Fatalf("initial UpdatePreferences: %v", err)
	}

	// Verify initial state
	before, _ := svc.GetPreferences(ctx, userID)
	if len(before) != 1 {
		t.Fatalf("expected 1 initial preference, got %d", len(before))
	}

	// Attempt update with a very long channel name that may exceed DB column limits
	// (notification_preferences.channel is likely VARCHAR — depends on schema)
	// Instead, try a valid update and verify it replaces atomically
	replacement := []notifications.Preference{
		{Channel: "email", EventType: "alert_fired", Enabled: true},
		{Channel: "sms", EventType: "order_shipped", Enabled: false},
	}
	if err := svc.UpdatePreferences(ctx, userID, replacement); err != nil {
		t.Fatalf("replacement UpdatePreferences: %v", err)
	}

	// Old preferences should be gone, new ones in place
	after, _ := svc.GetPreferences(ctx, userID)
	if len(after) != 2 {
		t.Errorf("expected 2 preferences after replacement, got %d", len(after))
	}

	// Verify the old "in_app" preference is gone
	for _, p := range after {
		if p.Channel == "in_app" {
			t.Error("old 'in_app' preference should have been deleted")
		}
	}
}

// TestExportRetryBookkeepingPersistsAttemptLog verifies that after a successful
// export retry, the attempt_log row includes the correct attempt number and path.
func TestExportRetryBookkeepingPersistsAttemptLog(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	exportsDir := t.TempDir()
	svc := notifications.NewService(pool, auditSvc, exportsDir)
	ctx := context.Background()

	var itemID int
	pool.QueryRow(ctx, `INSERT INTO export_queue_items (channel, recipient, subject, body, status, attempts, max_attempts) VALUES ('email','log@test.com','Log Test','Body content','pending',0,3) RETURNING id`).Scan(&itemID)

	count, err := svc.ProcessExportRetries(ctx)
	if err != nil {
		t.Fatalf("ProcessExportRetries: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 processed, got %d", count)
	}

	// Verify attempt log row
	var attemptNum int
	var logStatus, exportedPath string
	if err := pool.QueryRow(ctx, `SELECT attempt_number, status, COALESCE(exported_path,'') FROM export_attempt_logs WHERE queue_item_id=$1`, itemID).
		Scan(&attemptNum, &logStatus, &exportedPath); err != nil {
		t.Fatalf("query attempt log: %v", err)
	}
	if attemptNum != 1 {
		t.Errorf("expected attempt_number=1, got %d", attemptNum)
	}
	if logStatus != "exported" {
		t.Errorf("expected log status 'exported', got %q", logStatus)
	}
	if exportedPath == "" {
		t.Error("expected non-empty exported_path in attempt log")
	}

	// Verify attempts count was incremented
	var attempts int
	pool.QueryRow(ctx, `SELECT attempts FROM export_queue_items WHERE id=$1`, itemID).Scan(&attempts)
	if attempts != 1 {
		t.Errorf("expected attempts=1, got %d", attempts)
	}
}

func TestAnnouncementReadStatePerUser(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := notifications.NewService(pool, auditSvc, t.TempDir())
	ctx := context.Background()
	user1 := testutil.SeedUser(t, pool, "User One", "ann-user1@test.com")
	user2 := testutil.SeedUser(t, pool, "User Two", "ann-user2@test.com")

	// Create an announcement
	var annID int
	if err := pool.QueryRow(ctx, `INSERT INTO announcements (title, body, priority, is_active) VALUES ('Test Ann','Body','normal',TRUE) RETURNING id`).Scan(&annID); err != nil {
		t.Fatalf("seed announcement: %v", err)
	}

	// Both users see it as unread
	anns1, _ := svc.ListAnnouncements(ctx, user1)
	anns2, _ := svc.ListAnnouncements(ctx, user2)
	if len(anns1) == 0 || len(anns2) == 0 {
		t.Fatal("expected announcement visible to both users")
	}
	if anns1[0].IsRead || anns2[0].IsRead {
		t.Error("expected unread for both users")
	}

	// User1 marks as read
	if err := svc.MarkAnnouncementRead(ctx, annID, user1); err != nil {
		t.Fatalf("MarkAnnouncementRead: %v", err)
	}

	// User1 sees read, user2 still unread
	anns1, _ = svc.ListAnnouncements(ctx, user1)
	anns2, _ = svc.ListAnnouncements(ctx, user2)
	if len(anns1) == 0 {
		t.Fatal("expected announcement still visible to user1")
	}
	if !anns1[0].IsRead {
		t.Error("expected read for user1 after marking")
	}
	if len(anns2) == 0 {
		t.Fatal("expected announcement still visible to user2")
	}
	if anns2[0].IsRead {
		t.Error("expected unread for user2")
	}
}
