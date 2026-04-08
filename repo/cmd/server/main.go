package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fleetcommerce/internal/alerts"
	"fleetcommerce/internal/app"
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
	"fleetcommerce/internal/scheduler"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := app.LoadConfig()

	// Wait for database to be reachable (critical for docker compose)
	var pool *pgxpool.Pool
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err = db.Connect(cfg.DatabaseURL)
		if err == nil {
			break
		}
		slog.Warn("database not ready, retrying...", "attempt", attempt, "error", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("database connection failed after retries", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Migrations
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// Seed demo data (idempotent — safe on every boot)
	db.RunSeeds(context.Background(), pool, cfg.EncryptionKey)

	// Services
	auditSvc := audit.NewService(pool)
	authSvc := auth.NewService(pool)
	rbacSvc := rbac.NewService(pool)
	catalogRepo := catalog.NewRepository(pool)
	catalogSvc := catalog.NewService(catalogRepo, pool, auditSvc)
	cartRepo := cart.NewRepository(pool)
	cartSvc := cart.NewService(cartRepo, pool, auditSvc)
	orderRepo := orders.NewRepository(pool)
	orderSvc := orders.NewService(orderRepo, pool, auditSvc)
	notifSvc := notifications.NewService(pool, auditSvc, cfg.ExportsDir)
	alertSvc := alerts.NewService(pool, auditSvc)
	metricSvc := metrics.NewService(pool, auditSvc)
	importSvc := imports.NewService(pool, auditSvc)

	// Renderer
	renderer := views.NewRenderer(true)

	// Handlers
	h := handlers.New(authSvc, catalogSvc, cartSvc, orderSvc, notifSvc, alertSvc, metricSvc, auditSvc, importSvc, renderer, cfg.UploadsDir, cfg.MaxUploadBytes)

	// Ensure directories
	os.MkdirAll(cfg.UploadsDir, 0755)
	os.MkdirAll(cfg.ExportsDir, 0755)

	// Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = cfg.MaxUploadBytes

	// Static files
	r.Static("/static", "./web/static")
	r.Static("/uploads", cfg.UploadsDir)

	// CSRF
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
		api.GET("/me", h.GetMe) // authenticated-only, returns own data
		api.GET("/dashboard/summary", middleware.RequirePermission(rbac.PermDashboardRead), h.DashboardSummaryAPI)

		// Catalog
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

		// Cart — object-level auth enforced in handlers
		api.GET("/carts", middleware.RequirePermission(rbac.PermCartRead), h.ListCartsAPI)
		api.POST("/carts", middleware.RequirePermission(rbac.PermCartWrite), h.CreateCartAPI)
		api.GET("/carts/:id", middleware.RequirePermission(rbac.PermCartRead), h.GetCartAPI)
		api.POST("/carts/:id/items", middleware.RequirePermission(rbac.PermCartWrite), h.AddCartItemAPI)
		api.PUT("/carts/:id/items/:item_id", middleware.RequirePermission(rbac.PermCartWrite), h.UpdateCartItemAPI)
		api.DELETE("/carts/:id/items/:item_id", middleware.RequirePermission(rbac.PermCartWrite), h.DeleteCartItemAPI)
		api.POST("/carts/:id/merge", middleware.RequirePermission(rbac.PermCartMerge), h.MergeCartAPI)
		api.POST("/carts/:id/revalidate", middleware.RequirePermission(rbac.PermCartWrite), h.RevalidateCartAPI)
		api.POST("/carts/:id/checkout", middleware.RequirePermission(rbac.PermOrderCreate), h.CheckoutCartAPI)

		// Orders — object-level auth enforced in handlers
		api.GET("/orders", middleware.RequirePermission(rbac.PermOrderRead), h.ListOrdersAPI)
		api.GET("/orders/:id", middleware.RequirePermission(rbac.PermOrderRead), h.GetOrderAPI)
		api.POST("/orders/:id/notes", middleware.RequirePermission(rbac.PermOrderNotes), h.AddOrderNoteAPI)
		api.POST("/orders/:id/payment-recorded", middleware.RequirePermission(rbac.PermOrderPayment), h.RecordPaymentAPI)
		api.POST("/orders/:id/transition", middleware.RequirePermission(rbac.PermOrderTransition), h.TransitionOrderAPI)
		api.POST("/orders/:id/split", middleware.RequirePermission(rbac.PermOrderSplit), h.SplitOrderAPI)
		api.GET("/orders/:id/timeline", middleware.RequirePermission(rbac.PermOrderRead), h.OrderTimelineAPI)

		// Notifications
		api.GET("/notifications", middleware.RequirePermission(rbac.PermNotificationRead), h.ListNotificationsAPI)
		api.POST("/notifications/:id/read", middleware.RequirePermission(rbac.PermNotificationRead), h.MarkNotificationReadAPI)
		api.POST("/notifications/bulk-read", middleware.RequirePermission(rbac.PermNotificationRead), h.BulkMarkReadAPI)
		api.GET("/announcements", middleware.RequirePermission(rbac.PermNotificationRead), h.ListAnnouncementsAPI)
		api.POST("/announcements/:id/read", middleware.RequirePermission(rbac.PermNotificationRead), h.MarkAnnouncementReadAPI)
		api.GET("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.GetPreferencesAPI)
		api.PUT("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.UpdatePreferencesAPI)
		api.POST("/notification-preferences", middleware.RequirePermission(rbac.PermNotificationRead), h.UpdatePreferencesAPI)
		api.GET("/export-queue", middleware.RequirePermission(rbac.PermNotificationManage), h.ListExportQueueAPI)

		// Alerts
		api.GET("/alerts", middleware.RequirePermission(rbac.PermAlertRead), h.ListAlertsAPI)
		api.POST("/alerts/:id/claim", middleware.RequirePermission(rbac.PermAlertManage), h.ClaimAlertAPI)
		api.POST("/alerts/:id/process", middleware.RequirePermission(rbac.PermAlertManage), h.ProcessAlertAPI)
		api.POST("/alerts/:id/close", middleware.RequirePermission(rbac.PermAlertManage), h.CloseAlertAPI)
		api.POST("/alerts/evaluate", middleware.RequirePermission(rbac.PermAlertManage), h.EvaluateAlertsAPI)

		// Metrics
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

		// Audit
		api.GET("/audit", middleware.RequirePermission(rbac.PermAuditRead), h.ListAuditAPI)
		api.GET("/audit/:entity_type/:entity_id", middleware.RequirePermission(rbac.PermAuditRead), h.AuditByEntityAPI)
	}

	// Scheduler
	if cfg.SchedulerEnabled {
		sched := scheduler.New(orderSvc, alertSvc, notifSvc, cfg.CutoffInterval, cfg.AlertInterval, cfg.ExportRetryInt)
		sched.Start()
		defer sched.Stop()
	}

	// Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("server stopped")
}
