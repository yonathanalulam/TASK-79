package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAlertNotOpen       = errors.New("alert is not in open state")
	ErrAlertNotClaimed    = errors.New("alert is not claimed")
	ErrAlertNotProcessing = errors.New("alert is not in processing state")
	ErrResolutionRequired = errors.New("resolution notes are required to close an alert")
)

type AlertRule struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	RuleType    string          `json:"rule_type"`
	Condition   json.RawMessage `json:"condition"`
	Severity    string          `json:"severity"`
	IsActive    bool            `json:"is_active"`
}

type Alert struct {
	ID              int        `json:"id"`
	AlertRuleID     int        `json:"alert_rule_id"`
	EntityType      string     `json:"entity_type"`
	EntityID        int        `json:"entity_id"`
	Status          string     `json:"status"`
	Severity        string     `json:"severity"`
	Title           string     `json:"title"`
	Details         json.RawMessage `json:"details"`
	ClaimedBy       *int       `json:"claimed_by"`
	ClaimedByName   string     `json:"claimed_by_name"`
	ClaimedAt       *time.Time `json:"claimed_at"`
	ResolutionNotes string     `json:"resolution_notes"`
	ClosedAt        *time.Time `json:"closed_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Service struct {
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, auditSvc: auditSvc}
}

func (s *Service) ListAlerts(ctx context.Context, status string) ([]Alert, error) {
	query := `SELECT a.id, a.alert_rule_id, a.entity_type, a.entity_id, a.status, a.severity, a.title,
		a.details, a.claimed_by, COALESCE(u.full_name,''), a.claimed_at, a.resolution_notes, a.closed_at, a.created_at
		FROM alerts a LEFT JOIN users u ON u.id = a.claimed_by`
	var args []interface{}
	if status != "" {
		query += ` WHERE a.status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY a.created_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		var resNotes *string
		if err := rows.Scan(&a.ID, &a.AlertRuleID, &a.EntityType, &a.EntityID, &a.Status, &a.Severity, &a.Title,
			&a.Details, &a.ClaimedBy, &a.ClaimedByName, &a.ClaimedAt, &resNotes, &a.ClosedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		if resNotes != nil {
			a.ResolutionNotes = *resNotes
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Service) ClaimAlert(ctx context.Context, alertID, userID int) error {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if err != nil {
		return err
	}
	if status != "open" {
		return ErrAlertNotOpen
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE alerts SET status='claimed', claimed_by=$1, claimed_at=NOW(), updated_at=NOW() WHERE id=$2`, userID, alertID); err != nil {
		return fmt.Errorf("update alert status: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alert_events (alert_id, event_type, from_status, to_status, actor_id) VALUES ($1,'claimed','open','claimed',$2)`, alertID, userID); err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "alert",
		EntityID:    alertID,
		Action:      "claimed",
		ActorUserID: &userID,
		Before:      map[string]string{"status": "open"},
		After:       map[string]string{"status": "claimed"},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) ProcessAlert(ctx context.Context, alertID, userID int) error {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if err != nil {
		return err
	}
	if status != "claimed" {
		return ErrAlertNotClaimed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE alerts SET status='processing', updated_at=NOW() WHERE id=$1`, alertID); err != nil {
		return fmt.Errorf("update alert status: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alert_events (alert_id, event_type, from_status, to_status, actor_id) VALUES ($1,'processing','claimed','processing',$2)`, alertID, userID); err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "alert",
		EntityID:    alertID,
		Action:      "processing",
		ActorUserID: &userID,
		Before:      map[string]string{"status": "claimed"},
		After:       map[string]string{"status": "processing"},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) CloseAlert(ctx context.Context, alertID, userID int, resolutionNotes string) error {
	if resolutionNotes == "" {
		return ErrResolutionRequired
	}

	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM alerts WHERE id=$1`, alertID).Scan(&status)
	if err != nil {
		return err
	}
	if status != "processing" {
		return ErrAlertNotProcessing
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE alerts SET status='closed', resolution_notes=$1, closed_at=NOW(), updated_at=NOW() WHERE id=$2`, resolutionNotes, alertID); err != nil {
		return fmt.Errorf("update alert status: %w", err)
	}
	detailsJSON, _ := json.Marshal(map[string]string{"resolution_notes": resolutionNotes})
	if _, err := tx.Exec(ctx, `INSERT INTO alert_events (alert_id, event_type, from_status, to_status, actor_id, details) VALUES ($1,'closed','processing','closed',$2,$3)`,
		alertID, userID, detailsJSON); err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "alert",
		EntityID:    alertID,
		Action:      "closed",
		ActorUserID: &userID,
		Before:      map[string]string{"status": "processing"},
		After:       map[string]string{"status": "closed"},
		Metadata:    map[string]string{"resolution_notes": resolutionNotes},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

// EvaluateAlerts checks all active rules and creates alerts as needed
func (s *Service) EvaluateAlerts(ctx context.Context) (int, error) {
	rules, err := s.listActiveRules(ctx)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, rule := range rules {
		n, err := s.evaluateRule(ctx, rule)
		if err != nil {
			continue
		}
		created += n
	}
	return created, nil
}

func (s *Service) listActiveRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, rule_type, condition, severity, is_active FROM alert_rules WHERE is_active=TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.RuleType, &r.Condition, &r.Severity, &r.IsActive); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (s *Service) evaluateRule(ctx context.Context, rule AlertRule) (int, error) {
	var cond struct {
		Operator  string `json:"operator"`
		Threshold int    `json:"threshold"`
		Days      int    `json:"days"`
	}
	json.Unmarshal(rule.Condition, &cond)

	var query string
	switch rule.RuleType {
	case "low_stock":
		query = `SELECT id, model_name, stock_quantity FROM vehicle_models WHERE publication_status='published' AND stock_quantity < $1`
	case "overstock":
		query = `SELECT id, model_name, stock_quantity FROM vehicle_models WHERE publication_status='published' AND stock_quantity > $1`
	case "near_expiry":
		query = `SELECT id, model_name, stock_quantity FROM vehicle_models WHERE publication_status='published' AND expiry_date IS NOT NULL AND expiry_date <= CURRENT_DATE + ($1 || ' days')::interval`
	default:
		return 0, nil
	}

	rows, err := s.pool.Query(ctx, query, cond.Threshold)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	created := 0
	for rows.Next() {
		var modelID int
		var modelName string
		var stock int
		if err := rows.Scan(&modelID, &modelName, &stock); err != nil {
			continue
		}

		// Check if alert already exists (open) for this rule+entity combo
		var existing int
		s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE alert_rule_id=$1 AND entity_type='vehicle_model' AND entity_id=$2 AND status IN ('open','claimed','processing')`,
			rule.ID, modelID).Scan(&existing)
		if existing > 0 {
			continue
		}

		details, _ := json.Marshal(map[string]interface{}{
			"model_name":     modelName,
			"stock_quantity": stock,
			"rule_type":      rule.RuleType,
			"threshold":      cond.Threshold,
		})

		title := rule.Name + ": " + modelName
		_, err := s.pool.Exec(ctx, `INSERT INTO alerts (alert_rule_id, entity_type, entity_id, severity, title, details) VALUES ($1,'vehicle_model',$2,$3,$4,$5)`,
			rule.ID, modelID, rule.Severity, title, details)
		if err == nil {
			created++
		}
	}
	return created, nil
}

func (s *Service) CountActive(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE status IN ('open','claimed','processing')`).Scan(&count)
	return count, err
}
