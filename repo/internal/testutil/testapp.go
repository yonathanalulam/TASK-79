package testutil

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"fleetcommerce/internal/alerts"
	"fleetcommerce/internal/audit"
	"fleetcommerce/internal/auth"
	"fleetcommerce/internal/cart"
	"fleetcommerce/internal/catalog"
	"fleetcommerce/internal/db"
	"fleetcommerce/internal/http/handlers"
	"fleetcommerce/internal/http/middleware"
	"fleetcommerce/internal/http/views"
	"fleetcommerce/internal/imports"
	"fleetcommerce/internal/metrics"
	"fleetcommerce/internal/notifications"
	"fleetcommerce/internal/orders"
	"fleetcommerce/internal/rbac"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestApp wraps a fully bootstrapped application for integration testing.
// It mirrors the production wiring in cmd/server/main.go.
type TestApp struct {
	Pool   *pgxpool.Pool
	Router *gin.Engine
	Server *httptest.Server
}

// TestEntities holds IDs of seeded test data for parameterized route tests.
type TestEntities struct {
	AdminUserID    int
	ModelID        int
	DraftModelID   int
	CustomerID     int
	CartID         int
	CartID2        int
	CartItemID     int
	OrderID        int
	OrderLineID    int
	AlertID        int // open
	AlertID2       int // claimed
	AlertID3       int // processing
	NotificationID int
	AnnouncementID int
	ImportJobID    int
	MetricID       int
	MetricID2      int
	DimensionID    int
	FilterID       int
	DependencyID   int
}

// TestClient wraps an HTTP client with session and CSRF handling.
type TestClient struct {
	Client    *http.Client
	ServerURL string
	CSRFToken string
}

// MustApp bootstraps the full application stack for integration testing.
// Skips the test if FLEET_TEST_DB is not set.
func MustApp(t *testing.T) *TestApp {
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
		t.Fatalf("run migrations: %v", err)
	}

	// Clean state for test isolation
	truncateAll(t, pool)

	// Re-seed from migration 002
	ctx := context.Background()
	seedSQL, err := db.MigrationsFS.ReadFile("migrations/002_seed_data.up.sql")
	if err != nil {
		pool.Close()
		t.Fatalf("read seed SQL: %v", err)
	}
	if _, err := pool.Exec(ctx, string(seedSQL)); err != nil {
		pool.Close()
		t.Fatalf("execute seed SQL: %v", err)
	}

	// Update password hashes so real login works
	db.RunSeeds(ctx, pool, "0123456789abcdef0123456789abcdef")

	// Temp dirs for uploads/exports
	tmpDir := t.TempDir()
	uploadsDir := tmpDir + "/uploads"
	exportsDir := tmpDir + "/exports"
	os.MkdirAll(uploadsDir, 0755)
	os.MkdirAll(exportsDir, 0755)

	// Services — same wiring as cmd/server/main.go
	auditSvc := audit.NewService(pool)
	authSvc := auth.NewService(pool)
	rbacSvc := rbac.NewService(pool)
	catalogRepo := catalog.NewRepository(pool)
	catalogSvc := catalog.NewService(catalogRepo, pool, auditSvc)
	cartRepo := cart.NewRepository(pool)
	cartSvc := cart.NewService(cartRepo, pool, auditSvc)
	orderRepo := orders.NewRepository(pool)
	orderSvc := orders.NewService(orderRepo, pool, auditSvc)
	notifSvc := notifications.NewService(pool, auditSvc, exportsDir)
	alertSvc := alerts.NewService(pool, auditSvc)
	metricSvc := metrics.NewService(pool, auditSvc)
	importSvc := imports.NewService(pool, auditSvc)

	renderer := views.NewRenderer(true)
	h := handlers.New(authSvc, catalogSvc, cartSvc, orderSvc, notifSvc, alertSvc,
		metricSvc, auditSvc, importSvc, renderer, uploadsDir, 25*1024*1024)

	// Router — same middleware chain and route table as cmd/server/main.go
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = 25 * 1024 * 1024
	r.Use(middleware.CSRFMiddleware())

	// Public routes
	r.GET("/login", h.LoginPage)
	r.POST("/login", h.LoginPost)

	// Authenticated routes — every route gets explicit permission middleware
	authed := r.Group("/")
	authed.Use(middleware.AuthMiddleware(authSvc, rbacSvc))
	{
		authed.POST("/logout", h.Logout)
		authed.GET("/", middleware.RequirePermission(rbac.PermDashboardRead), h.DashboardPage)

		authed.GET("/catalog", middleware.RequirePermission(rbac.PermCatalogRead), h.CatalogPage)
		authed.GET("/catalog/new", middleware.RequirePermission(rbac.PermCatalogWrite), h.CatalogEditPage)
		authed.GET("/catalog/import", middleware.RequirePermission(rbac.PermCatalogImport), h.ImportModal)
		authed.GET("/catalog/:id", middleware.RequirePermission(rbac.PermCatalogRead), h.CatalogDetailPage)
		authed.GET("/catalog/:id/edit", middleware.RequirePermission(rbac.PermCatalogWrite), h.CatalogEditPage)

		authed.GET("/cart", middleware.RequirePermission(rbac.PermCartRead), h.CartPage)
		authed.GET("/cart/:id", middleware.RequirePermission(rbac.PermCartRead), h.CartDetailPage)
		authed.GET("/cart/:id/merge-modal", middleware.RequirePermission(rbac.PermCartMerge), h.MergeCartModal)
		authed.GET("/cart/:id/add-item", middleware.RequirePermission(rbac.PermCartWrite), h.AddCartItemModal)

		authed.GET("/orders", middleware.RequirePermission(rbac.PermOrderRead), h.OrdersPage)
		authed.GET("/orders/:id", middleware.RequirePermission(rbac.PermOrderRead), h.OrderDetailPage)
		authed.GET("/orders/:id/split-modal", middleware.RequirePermission(rbac.PermOrderSplit), h.SplitOrderModal)

		authed.GET("/notifications", middleware.RequirePermission(rbac.PermNotificationRead), h.NotificationsPage)

		authed.GET("/alerts", middleware.RequirePermission(rbac.PermAlertRead), h.AlertsPage)
		authed.GET("/alerts/:id/close-modal", middleware.RequirePermission(rbac.PermAlertManage), h.CloseAlertModal)

		authed.GET("/metrics", middleware.RequirePermission(rbac.PermMetricRead), h.MetricsPage)
		authed.GET("/metrics/:id", middleware.RequirePermission(rbac.PermMetricRead), h.MetricDetailPage)
		authed.GET("/metrics/new", middleware.RequirePermission(rbac.PermMetricWrite), h.CreateMetricModal)

		authed.GET("/audit", middleware.RequirePermission(rbac.PermAuditRead), h.AuditPage)
	}

	// API routes — explicit permission on every endpoint
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(authSvc, rbacSvc))
	{
		api.GET("/me", h.GetMe)
		api.GET("/dashboard/summary", middleware.RequirePermission(rbac.PermDashboardRead), h.DashboardSummaryAPI)

		api.GET("/catalog/brands", middleware.RequirePermission(rbac.PermCatalogRead), h.ListBrandsAPI)
		api.GET("/catalog/series", middleware.RequirePermission(rbac.PermCatalogRead), h.ListSeriesAPI)
		api.GET("/catalog/models", middleware.RequirePermission(rbac.PermCatalogRead), h.ListModelsAPI)
		api.GET("/catalog/models/:id", middleware.RequirePermission(rbac.PermCatalogRead), h.GetModelAPI)
		api.POST("/catalog/models", middleware.RequirePermission(rbac.PermCatalogWrite), h.CreateModelAPI)
		api.PUT("/catalog/models/:id/draft", middleware.RequirePermission(rbac.PermCatalogWrite), h.UpdateDraftAPI)
		api.POST("/catalog/models/:id/draft", middleware.RequirePermission(rbac.PermCatalogWrite), h.UpdateDraftAPI)
		api.POST("/catalog/models/:id/publish", middleware.RequirePermission(rbac.PermCatalogPublish), h.PublishModelAPI)
		api.POST("/catalog/models/:id/unpublish", middleware.RequirePermission(rbac.PermCatalogPublish), h.UnpublishModelAPI)
		api.POST("/catalog/models/:id/media", middleware.RequirePermission(rbac.PermMediaUpload), h.UploadMediaAPI)
		api.GET("/catalog/export.csv", middleware.RequirePermission(rbac.PermCatalogRead), h.ExportCatalogCSV)
		api.POST("/catalog/imports", middleware.RequirePermission(rbac.PermCatalogImport), h.ImportCatalogCSV)
		api.GET("/catalog/imports/:job_id", middleware.RequirePermission(rbac.PermCatalogImport), h.GetImportJob)
		api.POST("/catalog/imports/:job_id/commit", middleware.RequirePermission(rbac.PermCatalogImport), h.CommitImportJob)

		api.GET("/carts", middleware.RequirePermission(rbac.PermCartRead), h.ListCartsAPI)
		api.POST("/carts", middleware.RequirePermission(rbac.PermCartWrite), h.CreateCartAPI)
		api.GET("/carts/:id", middleware.RequirePermission(rbac.PermCartRead), h.GetCartAPI)
		api.POST("/carts/:id/items", middleware.RequirePermission(rbac.PermCartWrite), h.AddCartItemAPI)
		api.PUT("/carts/:id/items/:item_id", middleware.RequirePermission(rbac.PermCartWrite), h.UpdateCartItemAPI)
		api.DELETE("/carts/:id/items/:item_id", middleware.RequirePermission(rbac.PermCartWrite), h.DeleteCartItemAPI)
		api.POST("/carts/:id/merge", middleware.RequirePermission(rbac.PermCartMerge), h.MergeCartAPI)
		api.POST("/carts/:id/revalidate", middleware.RequirePermission(rbac.PermCartWrite), h.RevalidateCartAPI)
		api.POST("/carts/:id/checkout", middleware.RequirePermission(rbac.PermOrderCreate), h.CheckoutCartAPI)

		api.GET("/orders", middleware.RequirePermission(rbac.PermOrderRead), h.ListOrdersAPI)
		api.GET("/orders/:id", middleware.RequirePermission(rbac.PermOrderRead), h.GetOrderAPI)
		api.POST("/orders/:id/notes", middleware.RequirePermission(rbac.PermOrderNotes), h.AddOrderNoteAPI)
		api.POST("/orders/:id/payment-recorded", middleware.RequirePermission(rbac.PermOrderPayment), h.RecordPaymentAPI)
		api.POST("/orders/:id/transition", middleware.RequirePermission(rbac.PermOrderTransition), h.TransitionOrderAPI)
		api.POST("/orders/:id/split", middleware.RequirePermission(rbac.PermOrderSplit), h.SplitOrderAPI)
		api.GET("/orders/:id/timeline", middleware.RequirePermission(rbac.PermOrderRead), h.OrderTimelineAPI)

		api.GET("/notifications", middleware.RequirePermission(rbac.PermNotificationRead), h.ListNotificationsAPI)
		api.POST("/notifications/:id/read", middleware.RequirePermission(rbac.PermNotificationRead), h.MarkNotificationReadAPI)
		api.POST("/notifications/bulk-read", middleware.RequirePermission(rbac.PermNotificationRead), h.BulkMarkReadAPI)
		api.GET("/announcements", middleware.RequirePermission(rbac.PermNotificationRead), h.ListAnnouncementsAPI)
		api.POST("/announcements/:id/read", middleware.RequirePermission(rbac.PermNotificationRead), h.MarkAnnouncementReadAPI)
		api.GET("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.GetPreferencesAPI)
		api.PUT("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.UpdatePreferencesAPI)
		api.POST("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.UpdatePreferencesAPI)
		api.GET("/export-queue", middleware.RequirePermission(rbac.PermNotificationManage), h.ListExportQueueAPI)

		api.GET("/alerts", middleware.RequirePermission(rbac.PermAlertRead), h.ListAlertsAPI)
		api.POST("/alerts/:id/claim", middleware.RequirePermission(rbac.PermAlertManage), h.ClaimAlertAPI)
		api.POST("/alerts/:id/process", middleware.RequirePermission(rbac.PermAlertManage), h.ProcessAlertAPI)
		api.POST("/alerts/:id/close", middleware.RequirePermission(rbac.PermAlertManage), h.CloseAlertAPI)
		api.POST("/alerts/evaluate", middleware.RequirePermission(rbac.PermAlertManage), h.EvaluateAlertsAPI)

		api.GET("/metrics", middleware.RequirePermission(rbac.PermMetricRead), h.ListMetricsAPI)
		api.POST("/metrics", middleware.RequirePermission(rbac.PermMetricWrite), h.CreateMetricAPI)
		api.GET("/metrics/:id", middleware.RequirePermission(rbac.PermMetricRead), h.GetMetricAPI)
		api.PUT("/metrics/:id", middleware.RequirePermission(rbac.PermMetricWrite), h.UpdateMetricAPI)
		api.GET("/metrics/:id/versions", middleware.RequirePermission(rbac.PermMetricRead), h.ListMetricVersionsAPI)
		api.GET("/metrics/:id/dimensions", middleware.RequirePermission(rbac.PermMetricRead), h.ListMetricDimensionsAPI)
		api.POST("/metrics/:id/dimensions", middleware.RequirePermission(rbac.PermMetricWrite), h.AddMetricDimensionAPI)
		api.DELETE("/metrics/:id/dimensions/:dim_id", middleware.RequirePermission(rbac.PermMetricWrite), h.RemoveMetricDimensionAPI)
		api.GET("/metrics/:id/filters", middleware.RequirePermission(rbac.PermMetricRead), h.ListMetricFiltersAPI)
		api.POST("/metrics/:id/filters", middleware.RequirePermission(rbac.PermMetricWrite), h.AddMetricFilterAPI)
		api.DELETE("/metrics/:id/filters/:filter_id", middleware.RequirePermission(rbac.PermMetricWrite), h.RemoveMetricFilterAPI)
		api.GET("/metrics/:id/dependencies", middleware.RequirePermission(rbac.PermMetricRead), h.ListMetricDependenciesAPI)
		api.POST("/metrics/:id/dependencies", middleware.RequirePermission(rbac.PermMetricWrite), h.AddMetricDependencyAPI)
		api.DELETE("/metrics/:id/dependencies/:dep_id", middleware.RequirePermission(rbac.PermMetricWrite), h.RemoveMetricDependencyAPI)
		api.POST("/metrics/:id/impact-analysis", middleware.RequirePermission(rbac.PermMetricActivate), h.ImpactAnalysisAPI)
		api.POST("/metrics/:id/activate", middleware.RequirePermission(rbac.PermMetricActivate), h.ActivateMetricAPI)
		api.GET("/metrics/:id/lineage", middleware.RequirePermission(rbac.PermMetricRead), h.MetricLineageAPI)

		api.GET("/audit", middleware.RequirePermission(rbac.PermAuditRead), h.ListAuditAPI)
		api.GET("/audit/:entity_type/:entity_id", middleware.RequirePermission(rbac.PermAuditRead), h.AuditByEntityAPI)
	}

	server := httptest.NewServer(r)
	t.Cleanup(func() {
		server.Close()
		pool.Close()
	})

	return &TestApp{Pool: pool, Router: r, Server: server}
}

// AuthClient returns a TestClient authenticated as the given user via real login flow.
func (app *TestApp) AuthClient(t *testing.T, username, password string) *TestClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// GET /login to obtain CSRF cookie
	resp, err := client.Get(app.Server.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()

	serverURL, _ := url.Parse(app.Server.URL)
	csrfToken := ""
	for _, c := range jar.Cookies(serverURL) {
		if c.Name == "csrf_token" {
			csrfToken = c.Value
			break
		}
	}
	if csrfToken == "" {
		t.Fatal("no csrf_token cookie from GET /login")
	}

	// POST /login with real credentials
	form := url.Values{
		"username":   {username},
		"password":   {password},
		"csrf_token": {csrfToken},
	}
	resp, err = client.PostForm(app.Server.URL+"/login", form)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login failed: expected 302, got %d", resp.StatusCode)
	}

	return &TestClient{Client: client, ServerURL: app.Server.URL, CSRFToken: csrfToken}
}

// SeedTestEntities creates additional entities needed for parameterized route tests.
func SeedTestEntities(t *testing.T, pool *pgxpool.Pool) *TestEntities {
	t.Helper()
	ctx := context.Background()
	e := &TestEntities{}

	pool.QueryRow(ctx, "SELECT id FROM users WHERE username='admin'").Scan(&e.AdminUserID)
	pool.QueryRow(ctx, "SELECT id FROM vehicle_models WHERE publication_status='published' LIMIT 1").Scan(&e.ModelID)
	pool.QueryRow(ctx, "SELECT id FROM vehicle_models WHERE publication_status='draft' LIMIT 1").Scan(&e.DraftModelID)
	pool.QueryRow(ctx, "SELECT id FROM customer_accounts LIMIT 1").Scan(&e.CustomerID)
	pool.QueryRow(ctx, "SELECT id FROM metric_definitions LIMIT 1").Scan(&e.MetricID)
	pool.QueryRow(ctx, "SELECT id FROM metric_definitions WHERE id != $1 LIMIT 1", e.MetricID).Scan(&e.MetricID2)

	if e.AdminUserID == 0 || e.ModelID == 0 || e.CustomerID == 0 || e.MetricID == 0 {
		t.Fatal("seed data missing: check that migration 002 ran successfully")
	}
	if e.DraftModelID == 0 {
		e.DraftModelID = e.ModelID
	}

	// Cart 1 — main test cart
	if err := pool.QueryRow(ctx,
		"INSERT INTO carts (customer_account_id, status, created_by) VALUES ($1, 'open', $2) RETURNING id",
		e.CustomerID, e.AdminUserID,
	).Scan(&e.CartID); err != nil {
		t.Fatalf("create cart: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO cart_items (cart_id, vehicle_model_id, quantity, validity_status) VALUES ($1, $2, 1, 'valid') RETURNING id",
		e.CartID, e.ModelID,
	).Scan(&e.CartItemID); err != nil {
		t.Fatalf("create cart item: %v", err)
	}

	// Cart 2 — merge source / checkout cart
	if err := pool.QueryRow(ctx,
		"INSERT INTO carts (customer_account_id, status, created_by) VALUES ($1, 'open', $2) RETURNING id",
		e.CustomerID, e.AdminUserID,
	).Scan(&e.CartID2); err != nil {
		t.Fatalf("create cart2: %v", err)
	}
	pool.Exec(ctx,
		"INSERT INTO cart_items (cart_id, vehicle_model_id, quantity, validity_status) VALUES ($1, $2, 1, 'valid')",
		e.CartID2, e.ModelID,
	)

	// Order
	if err := pool.QueryRow(ctx,
		"INSERT INTO orders (order_number, customer_account_id, status, created_by) VALUES ('TEST-ORD-001', $1, 'created', $2) RETURNING id",
		e.CustomerID, e.AdminUserID,
	).Scan(&e.OrderID); err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO order_lines (order_id, vehicle_model_id, quantity_requested, quantity_allocated, quantity_backordered, line_status, publication_snapshot) VALUES ($1, $2, 2, 2, 0, 'allocated', 'published') RETURNING id",
		e.OrderID, e.ModelID,
	).Scan(&e.OrderLineID); err != nil {
		t.Fatalf("create order line: %v", err)
	}

	// Alerts — three states for lifecycle tests
	var ruleID int
	pool.QueryRow(ctx, "SELECT id FROM alert_rules LIMIT 1").Scan(&ruleID)
	pool.QueryRow(ctx,
		`INSERT INTO alerts (alert_rule_id, entity_type, entity_id, status, severity, title, details) VALUES ($1, 'vehicle_model', $2, 'open', 'warning', 'Test Alert Open', '{"stock":3}') RETURNING id`,
		ruleID, e.ModelID,
	).Scan(&e.AlertID)
	pool.QueryRow(ctx,
		`INSERT INTO alerts (alert_rule_id, entity_type, entity_id, status, severity, title, details, claimed_by) VALUES ($1, 'vehicle_model', $2, 'claimed', 'warning', 'Test Alert Claimed', '{"stock":3}', $3) RETURNING id`,
		ruleID, e.ModelID, e.AdminUserID,
	).Scan(&e.AlertID2)
	pool.QueryRow(ctx,
		`INSERT INTO alerts (alert_rule_id, entity_type, entity_id, status, severity, title, details, claimed_by) VALUES ($1, 'vehicle_model', $2, 'processing', 'warning', 'Test Alert Processing', '{"stock":3}', $3) RETURNING id`,
		ruleID, e.ModelID, e.AdminUserID,
	).Scan(&e.AlertID3)

	// Notification
	pool.QueryRow(ctx,
		"INSERT INTO notifications (type, title, body, entity_type, entity_id) VALUES ('order_created', 'Test Notification', 'A test notification', 'order', $1) RETURNING id",
		e.OrderID,
	).Scan(&e.NotificationID)
	pool.Exec(ctx,
		"INSERT INTO notification_recipients (notification_id, user_id, is_read) VALUES ($1, $2, false)",
		e.NotificationID, e.AdminUserID,
	)

	// Announcement
	pool.QueryRow(ctx,
		"INSERT INTO announcements (title, body, priority, is_active) VALUES ('Test Announcement', 'Test body', 'normal', true) RETURNING id",
	).Scan(&e.AnnouncementID)

	// Import job
	pool.QueryRow(ctx,
		"INSERT INTO csv_import_jobs (filename, status, total_rows, valid_rows, invalid_rows, committed_rows, uploaded_by) VALUES ('test.csv', 'validated', 1, 1, 0, 0, $1) RETURNING id",
		e.AdminUserID,
	).Scan(&e.ImportJobID)
	pool.Exec(ctx,
		`INSERT INTO csv_import_rows (job_id, row_number, raw_data, status) VALUES ($1, 1, '{"model_code":"IMP-001","model_name":"Import Test","brand":"Toyota","series":"Sedan","year":"2025"}', 'valid')`,
		e.ImportJobID,
	)

	// Metric sub-objects
	pool.Exec(ctx, "INSERT INTO metric_permissions (metric_id, user_id, can_view, can_activate) VALUES ($1, $2, true, true)", e.MetricID, e.AdminUserID)
	pool.Exec(ctx, "INSERT INTO metric_permissions (metric_id, user_id, can_view, can_activate) VALUES ($1, $2, true, true)", e.MetricID2, e.AdminUserID)
	pool.QueryRow(ctx,
		"INSERT INTO metric_dimensions (metric_id, name, description) VALUES ($1, 'test_dim', 'Test dimension') RETURNING id", e.MetricID,
	).Scan(&e.DimensionID)
	pool.QueryRow(ctx,
		"INSERT INTO metric_filters (metric_id, name, expression) VALUES ($1, 'test_filter', 'status = active') RETURNING id", e.MetricID,
	).Scan(&e.FilterID)
	pool.QueryRow(ctx,
		"INSERT INTO metric_dependencies (metric_id, depends_on_metric) VALUES ($1, $2) RETURNING id", e.MetricID, e.MetricID2,
	).Scan(&e.DependencyID)

	return e
}

// --- TestClient HTTP helpers ---

func (tc *TestClient) GET(path string) *http.Response {
	resp, err := tc.Client.Get(tc.ServerURL + path)
	if err != nil {
		return nil
	}
	return resp
}

func (tc *TestClient) PostForm(path string, form url.Values) *http.Response {
	if form == nil {
		form = url.Values{}
	}
	form.Set("csrf_token", tc.CSRFToken)
	resp, err := tc.Client.PostForm(tc.ServerURL+path, form)
	if err != nil {
		return nil
	}
	return resp
}

func (tc *TestClient) PostJSON(path string, body string) *http.Response {
	req, _ := http.NewRequest("POST", tc.ServerURL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", tc.CSRFToken)
	resp, _ := tc.Client.Do(req)
	return resp
}

func (tc *TestClient) PutJSON(path string, body string) *http.Response {
	req, _ := http.NewRequest("PUT", tc.ServerURL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", tc.CSRFToken)
	resp, _ := tc.Client.Do(req)
	return resp
}

func (tc *TestClient) PutForm(path string, form url.Values) *http.Response {
	if form == nil {
		form = url.Values{}
	}
	form.Set("csrf_token", tc.CSRFToken)
	req, _ := http.NewRequest("PUT", tc.ServerURL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", tc.CSRFToken)
	resp, _ := tc.Client.Do(req)
	return resp
}

func (tc *TestClient) Delete(path string) *http.Response {
	req, _ := http.NewRequest("DELETE", tc.ServerURL+path, nil)
	req.Header.Set("X-CSRF-Token", tc.CSRFToken)
	resp, _ := tc.Client.Do(req)
	return resp
}

// ReadBody reads the response body and closes it.
func ReadBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
