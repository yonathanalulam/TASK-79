package testutil

import (
	"context"
	"os"
	"testing"

	"fleetcommerce/internal/audit"
	"fleetcommerce/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MustDB returns a connected pgxpool.Pool for integration tests.
// Skips the test if FLEET_TEST_DB is not set.
// Runs migrations and truncates all tables before returning.
func MustDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("FLEET_TEST_DB")
	if dsn == "" {
		t.Skip("FLEET_TEST_DB not set, skipping integration test")
	}

	pool, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}

	if err := db.RunMigrations(dsn); err != nil {
		pool.Close()
		t.Fatalf("migrate test DB: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	truncateAll(t, pool)
	return pool
}

// MustAudit returns an audit.Service backed by the given pool.
func MustAudit(t *testing.T, pool *pgxpool.Pool) *audit.Service {
	t.Helper()
	return audit.NewService(pool)
}

func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"export_attempt_logs", "export_queue_items",
		"notification_recipients", "notifications",
		"notification_preferences", "announcement_reads", "announcements",
		"alert_events", "alerts", "alert_rules",
		"metric_activation_reviews", "metric_lineage_edges", "metric_dependencies",
		"metric_filters", "metric_dimensions", "metric_definition_versions",
		"metric_permissions", "metric_definitions",
		"chart_metric_dependencies", "charts",
		"csv_import_rows", "csv_import_jobs",
		"order_notes", "order_state_history", "order_payments",
		"order_lines", "orders",
		"cart_items", "carts",
		"vehicle_model_media", "media", "vehicle_models", "series", "brands",
		"audit_log",
		"user_roles", "sessions", "users", "roles",
	}
	for _, table := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			// table may not exist in some test schemas, ignore
		}
	}
}

// SeedUser creates a test user and role, returns userID.
func SeedUser(t *testing.T, pool *pgxpool.Pool, fullName, email string) int {
	t.Helper()
	ctx := context.Background()

	// Ensure a role exists
	var roleID int
	err := pool.QueryRow(ctx, `INSERT INTO roles (name, display_name) VALUES ('admin','Admin') ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&roleID)
	if err != nil {
		t.Fatalf("seed role: %v", err)
	}

	var userID int
	err = pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, full_name) VALUES ($1, 'test_hash', $2) RETURNING id`,
		email, fullName).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		t.Fatalf("seed user_role: %v", err)
	}
	return userID
}
