package notifications

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	ID         int       `json:"id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	EntityType string    `json:"entity_type"`
	EntityID   int       `json:"entity_id"`
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

type Announcement struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Priority  string    `json:"priority"`
	IsActive  bool      `json:"is_active"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type Template struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

type Preference struct {
	Channel   string `json:"channel"`
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
}

type ExportQueueItem struct {
	ID          int       `json:"id"`
	Channel     string    `json:"channel"`
	Recipient   string    `json:"recipient"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	CreatedAt   time.Time `json:"created_at"`
}

type Service struct {
	pool       *pgxpool.Pool
	auditSvc   *audit.Service
	exportsDir string
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service, exportsDir string) *Service {
	return &Service{pool: pool, auditSvc: auditSvc, exportsDir: exportsDir}
}

func (s *Service) CreateNotification(ctx context.Context, nType, title, body, entityType string, entityID int, recipientUserIDs []int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var notifID int
	err = tx.QueryRow(ctx, `INSERT INTO notifications (type, title, body, entity_type, entity_id) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		nType, title, body, entityType, entityID).Scan(&notifID)
	if err != nil {
		return err
	}

	for _, uid := range recipientUserIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO notification_recipients (notification_id, user_id) VALUES ($1, $2)`, notifID, uid); err != nil {
			return fmt.Errorf("insert recipient %d: %w", uid, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *Service) ListForUser(ctx context.Context, userID int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `SELECT n.id, n.type, n.title, COALESCE(n.body,''), COALESCE(n.entity_type,''), COALESCE(n.entity_id,0), nr.is_read, n.created_at
		FROM notifications n JOIN notification_recipients nr ON nr.notification_id = n.id
		WHERE nr.user_id = $1 ORDER BY n.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.EntityType, &n.EntityID, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifs = append(notifs, n)
	}
	return notifs, nil
}

func (s *Service) MarkRead(ctx context.Context, notificationID, userID int) error {
	_, err := s.pool.Exec(ctx, `UPDATE notification_recipients SET is_read=TRUE, read_at=NOW() WHERE notification_id=$1 AND user_id=$2`,
		notificationID, userID)
	return err
}

func (s *Service) BulkMarkRead(ctx context.Context, userID int) error {
	_, err := s.pool.Exec(ctx, `UPDATE notification_recipients SET is_read=TRUE, read_at=NOW() WHERE user_id=$1 AND is_read=FALSE`, userID)
	return err
}

func (s *Service) CountUnread(ctx context.Context, userID int) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_recipients WHERE user_id=$1 AND is_read=FALSE`, userID).Scan(&count)
	return count, err
}

func (s *Service) ListAnnouncements(ctx context.Context, userID int) ([]Announcement, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.title, a.body, a.priority, a.is_active,
		CASE WHEN ar.user_id IS NOT NULL THEN TRUE ELSE FALSE END,
		a.created_at
		FROM announcements a
		LEFT JOIN announcement_reads ar ON ar.announcement_id = a.id AND ar.user_id = $1
		WHERE a.is_active=TRUE AND (a.expires_at IS NULL OR a.expires_at > NOW())
		ORDER BY a.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anns []Announcement
	for rows.Next() {
		var a Announcement
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Priority, &a.IsActive, &a.IsRead, &a.CreatedAt); err != nil {
			return nil, err
		}
		anns = append(anns, a)
	}
	return anns, nil
}

func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID, userID int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO announcement_reads (announcement_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		announcementID, userID)
	return err
}

func (s *Service) GetPreferences(ctx context.Context, userID int) ([]Preference, error) {
	rows, err := s.pool.Query(ctx, `SELECT channel, event_type, enabled FROM notification_preferences WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []Preference
	for rows.Next() {
		var p Preference
		if err := rows.Scan(&p.Channel, &p.EventType, &p.Enabled); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int, prefs []Preference) error {
	if len(prefs) == 0 {
		return fmt.Errorf("cannot update with empty preference list")
	}

	// Capture before state for audit
	before, _ := s.GetPreferences(ctx, userID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM notification_preferences WHERE user_id=$1`, userID); err != nil {
		return fmt.Errorf("delete preferences: %w", err)
	}
	for _, p := range prefs {
		if _, err := tx.Exec(ctx, `INSERT INTO notification_preferences (user_id, channel, event_type, enabled) VALUES ($1,$2,$3,$4)`,
			userID, p.Channel, p.EventType, p.Enabled); err != nil {
			return fmt.Errorf("insert preference: %w", err)
		}
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "notification_preference",
		EntityID:    userID,
		Action:      "updated",
		ActorUserID: &userID,
		Before:      before,
		After:       prefs,
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) ListExportQueue(ctx context.Context) ([]ExportQueueItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, channel, recipient, COALESCE(subject,''), body, status, attempts, max_attempts, created_at
		FROM export_queue_items ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ExportQueueItem
	for rows.Next() {
		var item ExportQueueItem
		if err := rows.Scan(&item.ID, &item.Channel, &item.Recipient, &item.Subject, &item.Body, &item.Status, &item.Attempts, &item.MaxAttempts, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) CreateExportQueueItem(ctx context.Context, channel, recipient, subject, body string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO export_queue_items (channel, recipient, subject, body) VALUES ($1,$2,$3,$4)`,
		channel, recipient, subject, body)
	return err
}

func (s *Service) ProcessExportRetries(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, channel, recipient, subject, body, attempts, max_attempts FROM export_queue_items
		WHERE status IN ('pending','retrying') AND attempts < max_attempts`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var item ExportQueueItem
		if err := rows.Scan(&item.ID, &item.Channel, &item.Recipient, &item.Subject, &item.Body, &item.Attempts, &item.MaxAttempts); err != nil {
			continue
		}

		newAttempt := item.Attempts + 1

		// Write actual local artifact file
		filename := fmt.Sprintf("export_%d_%s_%d.txt", item.ID, item.Channel, newAttempt)
		exportPath := filepath.Join(s.exportsDir, filename)
		payload := fmt.Sprintf("Channel: %s\nRecipient: %s\nSubject: %s\n\n%s", item.Channel, item.Recipient, item.Subject, item.Body)

		exportStatus := "exported"
		if err := os.WriteFile(exportPath, []byte(payload), 0644); err != nil {
			exportStatus = "retrying"
			exportPath = ""
		}

		if _, err := s.pool.Exec(ctx, `UPDATE export_queue_items SET status=$1, attempts=$2, last_attempt_at=NOW(), updated_at=NOW() WHERE id=$3`,
			exportStatus, newAttempt, item.ID); err != nil {
			return count, fmt.Errorf("update export queue item %d: %w", item.ID, err)
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO export_attempt_logs (queue_item_id, attempt_number, status, exported_path) VALUES ($1,$2,$3,$4)`,
			item.ID, newAttempt, exportStatus, exportPath); err != nil {
			return count, fmt.Errorf("insert export attempt log %d: %w", item.ID, err)
		}
		if exportStatus == "exported" {
			count++
		}
	}
	return count, nil
}

// RenderTemplate performs simple variable substitution in templates
func (s *Service) RenderTemplate(ctx context.Context, templateCode string, vars map[string]string) (string, string, error) {
	var tmpl Template
	err := s.pool.QueryRow(ctx, `SELECT id, code, name, subject, body FROM notification_templates WHERE code=$1`, templateCode).
		Scan(&tmpl.ID, &tmpl.Code, &tmpl.Name, &tmpl.Subject, &tmpl.Body)
	if err != nil {
		return "", "", err
	}

	subject := tmpl.Subject
	body := tmpl.Body
	for k, v := range vars {
		subject = strings.ReplaceAll(subject, "{{"+k+"}}", v)
		body = strings.ReplaceAll(body, "{{"+k+"}}", v)
	}

	return subject, body, nil
}
