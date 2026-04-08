package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ID           int
	EntityType   string
	EntityID     int
	Action       string
	ActorUserID  *int
	ActorRole    *string
	OccurredAt   time.Time
	BeforeJSON   json.RawMessage
	AfterJSON    json.RawMessage
	MetadataJSON json.RawMessage
	RequestID    string
	IPAddress    string
}

type LogParams struct {
	EntityType  string
	EntityID    int
	Action      string
	ActorUserID *int
	ActorRole   string
	Before      interface{}
	After       interface{}
	Metadata    interface{}
	RequestID   string
	IPAddress   string
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Log(ctx context.Context, p LogParams) error {
	return s.LogTx(ctx, nil, p)
}

func (s *Service) LogTx(ctx context.Context, tx pgx.Tx, p LogParams) error {
	beforeJSON, _ := marshalNullable(p.Before)
	afterJSON, _ := marshalNullable(p.After)
	metadataJSON, _ := marshalNullable(p.Metadata)

	var actorRole *string
	if p.ActorRole != "" {
		actorRole = &p.ActorRole
	}

	query := `INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, actor_role, before_json, after_json, metadata_json, request_id, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	args := []interface{}{p.EntityType, p.EntityID, p.Action, p.ActorUserID, actorRole, beforeJSON, afterJSON, metadataJSON, p.RequestID, p.IPAddress}

	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, query, args...)
	} else {
		_, err = s.pool.Exec(ctx, query, args...)
	}
	if err != nil {
		slog.Error("audit log failed", "entity_type", p.EntityType, "action", p.Action, "error", err)
	}
	return err
}

type ListParams struct {
	EntityType string
	EntityID   int
	Limit      int
	Offset     int
}

func (s *Service) List(ctx context.Context, p ListParams) ([]Entry, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if p.EntityType != "" {
		where += " AND entity_type = $" + itoa(argN)
		args = append(args, p.EntityType)
		argN++
	}
	if p.EntityID > 0 {
		where += " AND entity_id = $" + itoa(argN)
		args = append(args, p.EntityID)
		argN++
	}

	countQuery := "SELECT COUNT(*) FROM audit_log " + where
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if p.Limit <= 0 {
		p.Limit = 50
	}

	query := "SELECT id, entity_type, entity_id, action, actor_user_id, actor_role, occurred_at, before_json, after_json, metadata_json, request_id, ip_address FROM audit_log " + where + " ORDER BY occurred_at DESC LIMIT $" + itoa(argN) + " OFFSET $" + itoa(argN+1)
	args = append(args, p.Limit, p.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &e.ActorUserID, &e.ActorRole, &e.OccurredAt, &e.BeforeJSON, &e.AfterJSON, &e.MetadataJSON, &e.RequestID, &e.IPAddress); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, nil
}

func marshalNullable(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func itoa(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
