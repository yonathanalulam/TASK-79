package alerts_test

import (
	"context"
	"testing"

	"fleetcommerce/internal/alerts"
	"fleetcommerce/internal/testutil"
)

func TestClaimAlertPersistsStateAndAudit(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-claim@test.com")

	// Seed an alert rule and alert
	var ruleID int
	if err := pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('Test Rule','desc','low_stock','{"threshold":5}','medium') RETURNING id`).Scan(&ruleID); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	var alertID int
	if err := pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details) VALUES ($1,'vehicle_model',1,'medium','Test Alert','{}') RETURNING id`, ruleID).Scan(&alertID); err != nil {
		t.Fatalf("seed alert: %v", err)
	}

	// Claim
	if err := svc.ClaimAlert(ctx, alertID, userID); err != nil {
		t.Fatalf("ClaimAlert: %v", err)
	}

	// Verify status changed
	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "claimed" {
		t.Errorf("expected claimed, got %q", status)
	}

	// Verify audit log entry
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1 AND action='claimed'`, alertID).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for claim")
	}

	// Verify alert event
	var eventCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM alert_events WHERE alert_id=$1 AND event_type='claimed'`, alertID).Scan(&eventCount)
	if eventCount == 0 {
		t.Error("expected alert_event for claim")
	}
}

func TestProcessAlertPersistsStateAndAudit(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-process@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('Process Rule','desc','low_stock','{"threshold":5}','high') RETURNING id`).Scan(&ruleID)
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details, status, claimed_by, claimed_at) VALUES ($1,'vehicle_model',1,'high','Test','{}','claimed',$2,NOW()) RETURNING id`, ruleID, userID).Scan(&alertID)

	if err := svc.ProcessAlert(ctx, alertID, userID); err != nil {
		t.Fatalf("ProcessAlert: %v", err)
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "processing" {
		t.Errorf("expected processing, got %q", status)
	}

	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1 AND action='processing'`, alertID).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for processing")
	}
}

func TestCloseAlertRequiresResolutionNotes(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-close@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('Close Rule','desc','low_stock','{"threshold":5}','high') RETURNING id`).Scan(&ruleID)
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details, status) VALUES ($1,'vehicle_model',1,'high','Test','{}','processing') RETURNING id`, ruleID).Scan(&alertID)

	// Close without notes should fail
	err := svc.CloseAlert(ctx, alertID, userID, "")
	if err == nil {
		t.Fatal("expected error for empty resolution notes")
	}

	// Close with notes should succeed
	err = svc.CloseAlert(ctx, alertID, userID, "Restocked to adequate levels")
	if err != nil {
		t.Fatalf("CloseAlert: %v", err)
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "closed" {
		t.Errorf("expected closed, got %q", status)
	}

	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1 AND action='closed'`, alertID).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for close")
	}
}

func TestClaimAlertRejectsNonOpen(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-reject@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('Reject Rule','desc','low_stock','{"threshold":5}','high') RETURNING id`).Scan(&ruleID)
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details, status) VALUES ($1,'vehicle_model',1,'high','Test','{}','claimed') RETURNING id`, ruleID).Scan(&alertID)

	err := svc.ClaimAlert(ctx, alertID, userID)
	if err == nil {
		t.Fatal("expected error claiming non-open alert")
	}
}

// TestClaimAlertFailureDoesNotMutateState verifies that if ClaimAlert returns an error,
// the alert status is unchanged and no audit log or event was created.
func TestClaimAlertFailureDoesNotMutateState(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-noop@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('NoOp Rule','desc','low_stock','{"threshold":5}','low') RETURNING id`).Scan(&ruleID)

	// Create alert in "processing" status — claiming should fail (not open)
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details, status) VALUES ($1,'vehicle_model',1,'low','NoOp','{}','processing') RETURNING id`, ruleID).Scan(&alertID)

	err := svc.ClaimAlert(ctx, alertID, userID)
	if err == nil {
		t.Fatal("expected error claiming processing alert")
	}

	// Verify status is still "processing"
	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "processing" {
		t.Errorf("alert status should be unchanged, got %q", status)
	}

	// Verify no audit log was created
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1 AND action='claimed'`, alertID).Scan(&auditCount)
	if auditCount != 0 {
		t.Error("no audit log should exist for failed claim")
	}

	// Verify no alert event was created
	var eventCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM alert_events WHERE alert_id=$1 AND event_type='claimed'`, alertID).Scan(&eventCount)
	if eventCount != 0 {
		t.Error("no alert_event should exist for failed claim")
	}
}

// TestCloseAlertFailureLeavesStateUnchanged verifies close failure on wrong status
// does not modify the alert or create audit/event records.
func TestCloseAlertFailureLeavesStateUnchanged(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-close-fail@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('CloseFail','desc','low_stock','{"threshold":5}','medium') RETURNING id`).Scan(&ruleID)

	// Alert is "claimed" — close requires "processing"
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details, status, claimed_by, claimed_at) VALUES ($1,'vehicle_model',1,'medium','CloseFail','{}','claimed',$2,NOW()) RETURNING id`, ruleID, userID).Scan(&alertID)

	err := svc.CloseAlert(ctx, alertID, userID, "Should not close")
	if err == nil {
		t.Fatal("expected error closing claimed (not processing) alert")
	}

	// Status still claimed
	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "claimed" {
		t.Errorf("expected status unchanged at 'claimed', got %q", status)
	}

	// No audit or event for close
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1 AND action='closed'`, alertID).Scan(&auditCount)
	if auditCount != 0 {
		t.Error("no audit log should exist for failed close")
	}
}

// TestFullAlertLifecyclePersistsAllRecords tests the complete open->claimed->processing->closed
// lifecycle and verifies all audit and event records are created.
func TestFullAlertLifecyclePersistsAllRecords(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := alerts.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Alert User", "alert-lifecycle@test.com")

	var ruleID int
	pool.QueryRow(ctx, `INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES ('Lifecycle Rule','desc','low_stock','{"threshold":5}','critical') RETURNING id`).Scan(&ruleID)
	var alertID int
	pool.QueryRow(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details) VALUES ($1,'vehicle_model',1,'critical','Lifecycle Test','{}') RETURNING id`, ruleID).Scan(&alertID)

	// Claim
	if err := svc.ClaimAlert(ctx, alertID, userID); err != nil {
		t.Fatalf("ClaimAlert: %v", err)
	}

	// Process
	if err := svc.ProcessAlert(ctx, alertID, userID); err != nil {
		t.Fatalf("ProcessAlert: %v", err)
	}

	// Close
	if err := svc.CloseAlert(ctx, alertID, userID, "Issue resolved by restocking"); err != nil {
		t.Fatalf("CloseAlert: %v", err)
	}

	// Verify final status
	var status string
	pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if status != "closed" {
		t.Errorf("expected closed, got %q", status)
	}

	// Verify 3 audit entries (claimed, processing, closed)
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='alert' AND entity_id=$1`, alertID).Scan(&auditCount)
	if auditCount != 3 {
		t.Errorf("expected 3 audit entries, got %d", auditCount)
	}

	// Verify 3 alert events
	var eventCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM alert_events WHERE alert_id=$1`, alertID).Scan(&eventCount)
	if eventCount != 3 {
		t.Errorf("expected 3 alert events, got %d", eventCount)
	}

	// Verify resolution notes persisted
	var notes string
	pool.QueryRow(ctx, `SELECT COALESCE(resolution_notes,'') FROM alerts WHERE id=$1`, alertID).Scan(&notes)
	if notes != "Issue resolved by restocking" {
		t.Errorf("expected resolution notes, got %q", notes)
	}
}
