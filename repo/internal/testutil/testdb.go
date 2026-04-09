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

	// Drop audit_log rules temporarily so TRUNCATE works
	pool.Exec(ctx, "DROP RULE IF EXISTS audit_log_no_delete ON audit_log")
	pool.Exec(ctx, "DROP RULE IF EXISTS audit_log_no_update ON audit_log")

	// TRUNCATE CASCADE is the most reliable way to clean all tables
	_, err := pool.Exec(ctx, `TRUNCATE
		export_attempt_logs, export_queue_items,
		notification_recipients, notifications,
		notification_preferences, announcement_reads, announcements,
		notification_templates,
		alert_events, alerts, alert_rules,
		metric_activation_reviews, metric_lineage_edges, metric_dependencies,
		metric_filters, metric_dimensions, metric_definition_versions,
		metric_permissions, metric_definitions,
		chart_metric_dependencies, charts,
		csv_import_rows, csv_import_jobs,
		order_events, order_notes, order_state_history, payment_records,
		order_lines, orders,
		cart_events, cart_items, carts,
		customer_accounts,
		vehicle_media, vehicle_model_versions, vehicle_models,
		series, brands,
		sessions, user_roles, role_permissions, permissions,
		users, roles,
		audit_log
		CASCADE`)
	if err != nil {
		// If bulk TRUNCATE fails, try individual tables
		tables := []string{
			"audit_log",
			"export_attempt_logs", "export_queue_items",
			"notification_recipients", "notifications",
			"notification_preferences", "announcement_reads", "announcements",
			"notification_templates",
			"alert_events", "alerts", "alert_rules",
			"metric_activation_reviews", "metric_lineage_edges", "metric_dependencies",
			"metric_filters", "metric_dimensions", "metric_definition_versions",
			"metric_permissions", "metric_definitions",
			"chart_metric_dependencies", "charts",
			"csv_import_rows", "csv_import_jobs",
			"order_events", "order_notes", "order_state_history", "payment_records",
			"order_lines", "orders",
			"cart_events", "cart_items", "carts",
			"customer_accounts",
			"vehicle_media", "vehicle_model_versions", "vehicle_models",
			"series", "brands",
			"sessions", "user_roles", "role_permissions", "permissions",
			"users", "roles",
		}
		for _, table := range tables {
			pool.Exec(ctx, "TRUNCATE "+table+" CASCADE")
		}
	}

	// Re-create audit rules
	pool.Exec(ctx, "CREATE OR REPLACE RULE audit_log_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING")
	pool.Exec(ctx, "CREATE OR REPLACE RULE audit_log_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING")
}

// SeedUser creates a test user and returns userID.
// fullName is used as the display name; uniqueKey is used to generate a unique username.
// Creates a minimal role if needed.
func SeedUser(t *testing.T, pool *pgxpool.Pool, fullName, uniqueKey string) int {
	t.Helper()
	ctx := context.Background()

	// Ensure a role exists
	var roleID int
	err := pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ('test_admin', 'Test admin role') ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`,
	).Scan(&roleID)
	if err != nil {
		t.Fatalf("seed role: %v", err)
	}

	// Use uniqueKey as username (callers pass unique strings like emails)
	var userID int
	err = pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, full_name) VALUES ($1, 'test_hash', $2) RETURNING id`,
		uniqueKey, fullName,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		t.Fatalf("seed user_role: %v", err)
	}
	return userID
}
