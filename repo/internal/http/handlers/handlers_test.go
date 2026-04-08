package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fleetcommerce/internal/auth"
	"fleetcommerce/internal/http/middleware"
	"fleetcommerce/internal/orders"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// injectUser creates a gin handler that injects a fake authenticated user
// with the given permissions into the request context.
func injectUser(userID int, fullName string, perms map[string]bool, roles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := &auth.User{ID: userID, Username: "testuser", FullName: fullName, IsActive: true}
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, middleware.UserKey, user)
		ctx = context.WithValue(ctx, middleware.PermissionsKey, perms)
		ctx = context.WithValue(ctx, middleware.RolesKey, roles)
		ctx = context.WithValue(ctx, middleware.SessionIDKey, "test-session")
		c.Request = c.Request.WithContext(ctx)
		// Set CSRF token for the handler to read
		c.Set(middleware.CSRFTokenKey, "test-csrf-token")
		c.Next()
	}
}

// TestNotificationsPage_ExportQueueDeniedForReadOnly verifies that a user
// with only notification.read CANNOT access export queue data via the page.
// The handler should fall back to inbox tab.
func TestNotificationsPage_ExportQueueDeniedForReadOnly(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"notification.read": true,
		"dashboard.read":    true,
	}, []string{"sales_associate"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications?tab=export-queue", nil)
	r.ServeHTTP(w, req)

	// Should get 200 (page renders) but NOT contain export queue content.
	// Handler falls back to inbox because user lacks notification.manage.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// Should NOT contain export queue tab link (hidden for unauthorized)
	if strings.Contains(body, "tab=export-queue") {
		t.Error("export queue tab link should not appear for read-only user")
	}
	// Should render inbox content (the fallback tab)
	if !strings.Contains(body, "Notification Center") {
		t.Error("expected page to render notification center")
	}
}

// TestNotificationsPage_ExportQueueAllowedForManager verifies that a user
// with notification.manage CAN see the export queue tab and data.
func TestNotificationsPage_ExportQueueAllowedForManager(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	r.Use(injectUser(1, "Admin User", map[string]bool{
		"notification.read":   true,
		"notification.manage": true,
		"dashboard.read":      true,
		"users.manage":        true,
	}, []string{"administrator"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications?tab=export-queue", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// Should contain export queue tab link
	if !strings.Contains(body, "tab=export-queue") {
		t.Error("expected export queue tab link for manager")
	}
	// Should show export queue content area (empty state is fine)
	if !strings.Contains(body, "No export queue items") && !strings.Contains(body, "Export Queue") {
		t.Error("expected export queue content area for manager")
	}
}

// TestNotificationsPage_InboxAccessibleForReadOnly verifies that inbox
// works for users with only notification.read.
func TestNotificationsPage_InboxAccessibleForReadOnly(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"notification.read": true,
		"dashboard.read":    true,
	}, []string{"sales_associate"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Notification Center") {
		t.Error("expected notification center heading")
	}
	if !strings.Contains(body, "Inbox") {
		t.Error("expected inbox tab")
	}
}

// TestNotificationsPage_AnnouncementsTab verifies announcements tab renders.
func TestNotificationsPage_AnnouncementsTab(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"notification.read": true,
		"dashboard.read":    true,
	}, []string{"sales_associate"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications?tab=announcements", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No active announcements") {
		t.Error("expected announcements empty state")
	}
}

// TestNotificationsPage_PreferencesTab verifies preferences tab renders.
func TestNotificationsPage_PreferencesTab(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"notification.read": true,
		"dashboard.read":    true,
	}, []string{"sales_associate"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications?tab=preferences", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Save Preferences") {
		t.Error("expected preferences form")
	}
}

// TestExportQueueTabFallback_DirectURLManipulation verifies that even if
// a read-only user manually crafts ?tab=export-queue, the handler does NOT
// expose export queue data and falls back to inbox.
func TestExportQueueTabFallback_DirectURLManipulation(t *testing.T) {
	h := &Handlers{}
	r := gin.New()
	// Auditor has notification.read but NOT notification.manage
	r.Use(injectUser(4, "Auditor", map[string]bool{
		"notification.read": true,
		"audit.read":        true,
		"dashboard.read":    true,
	}, []string{"auditor"}))
	r.GET("/notifications", h.NotificationsPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications?tab=export-queue", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// Must NOT contain export queue data or tab link
	if strings.Contains(body, "tab=export-queue") {
		t.Error("export queue tab should not appear for auditor")
	}
	// Should have fallen back to inbox
	if !strings.Contains(body, "Notification Center") {
		t.Error("expected notification center page")
	}
}

// ---------- Order Timeline Authorization Tests ----------

// timelineAuthHandler creates a test handler that exercises the real
// enforceOrderAccess authorization path with a pre-loaded order, mirroring
// how OrderTimelineAPI works after the loadAndAuthorizeOrder fix.
// This isolates the authorization logic from the database dependency.
func timelineAuthHandler(h *Handlers, order *orders.Order) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.enforceOrderAccess(c, order) {
			return
		}
		jsonOK(c, "ok", nil)
	}
}

func intPtr(i int) *int { return &i }

// TestOrderTimelineAPI_AuthorizedScopedAccess verifies that a sales associate
// who created the order can access its timeline.
func TestOrderTimelineAPI_AuthorizedScopedAccess(t *testing.T) {
	h := &Handlers{}
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(3)}

	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"order.read":     true,
		"dashboard.read": true,
	}, []string{"sales_associate"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for order creator, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["ok"] != true {
		t.Error("expected ok:true in response")
	}
}

// TestOrderTimelineAPI_UnauthorizedObjectAccess verifies that a sales associate
// who did NOT create the order is denied access to its timeline (IDOR prevention).
func TestOrderTimelineAPI_UnauthorizedObjectAccess(t *testing.T) {
	h := &Handlers{}
	// Order created by user 99, but request comes from user 3
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(99)}

	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"order.read":     true,
		"dashboard.read": true,
	}, []string{"sales_associate"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner sales associate, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["ok"] != false {
		t.Error("expected ok:false in denial response")
	}
}

// TestOrderTimelineAPI_GlobalReadAccess_Admin verifies that an administrator
// with global order-read scope can access any order's timeline.
func TestOrderTimelineAPI_GlobalReadAccess_Admin(t *testing.T) {
	h := &Handlers{}
	// Order created by user 99, but admin (user 1) should still have access
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(99)}

	r := gin.New()
	r.Use(injectUser(1, "Admin User", map[string]bool{
		"order.read":     true,
		"users.manage":   true,
		"dashboard.read": true,
	}, []string{"administrator"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin with global read, got %d", w.Code)
	}
}

// TestOrderTimelineAPI_GlobalReadAccess_Auditor verifies that an auditor
// with audit.read (global order-read scope) can access any order's timeline.
func TestOrderTimelineAPI_GlobalReadAccess_Auditor(t *testing.T) {
	h := &Handlers{}
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(99)}

	r := gin.New()
	r.Use(injectUser(4, "Auditor User", map[string]bool{
		"order.read":     true,
		"audit.read":     true,
		"dashboard.read": true,
	}, []string{"auditor"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for auditor with global read, got %d", w.Code)
	}
}

// TestOrderTimelineAPI_GlobalReadAccess_InventoryManager verifies that an
// inventory manager with order.transition (global order-read scope) can access
// any order's timeline.
func TestOrderTimelineAPI_GlobalReadAccess_InventoryManager(t *testing.T) {
	h := &Handlers{}
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(99)}

	r := gin.New()
	r.Use(injectUser(5, "Inventory Mgr", map[string]bool{
		"order.read":       true,
		"order.transition": true,
		"dashboard.read":   true,
	}, []string{"inventory_manager"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for inventory manager with global read, got %d", w.Code)
	}
}

// TestOrderTimeline_ConsistencyWithDetailEndpoint verifies that authorization
// decisions for timeline and detail endpoints are consistent: an order denied
// on detail is also denied on timeline, and vice versa.
func TestOrderTimeline_ConsistencyWithDetailEndpoint(t *testing.T) {
	h := &Handlers{}
	// Order created by user 99
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: intPtr(99)}

	// detailAuthHandler mirrors GetOrderAPI's authorization check
	detailAuthHandler := func(c *gin.Context) {
		if !h.enforceOrderAccess(c, order) {
			return
		}
		jsonOK(c, "ok", order)
	}

	tests := []struct {
		name       string
		userID     int
		perms      map[string]bool
		roles      []string
		wantCode   int
		wantDenied bool
	}{
		{
			name:       "owner allowed on both",
			userID:     99,
			perms:      map[string]bool{"order.read": true},
			roles:      []string{"sales_associate"},
			wantCode:   http.StatusOK,
			wantDenied: false,
		},
		{
			name:       "non-owner denied on both",
			userID:     3,
			perms:      map[string]bool{"order.read": true},
			roles:      []string{"sales_associate"},
			wantCode:   http.StatusForbidden,
			wantDenied: true,
		},
		{
			name:       "admin allowed on both",
			userID:     1,
			perms:      map[string]bool{"order.read": true, "users.manage": true},
			roles:      []string{"administrator"},
			wantCode:   http.StatusOK,
			wantDenied: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test timeline endpoint
			rTimeline := gin.New()
			rTimeline.Use(injectUser(tc.userID, "Test User", tc.perms, tc.roles))
			rTimeline.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

			wTimeline := httptest.NewRecorder()
			reqTimeline := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
			rTimeline.ServeHTTP(wTimeline, reqTimeline)

			// Test detail endpoint
			rDetail := gin.New()
			rDetail.Use(injectUser(tc.userID, "Test User", tc.perms, tc.roles))
			rDetail.GET("/api/orders/:id", detailAuthHandler)

			wDetail := httptest.NewRecorder()
			reqDetail := httptest.NewRequest("GET", "/api/orders/42", nil)
			rDetail.ServeHTTP(wDetail, reqDetail)

			// Verify both endpoints agree
			if wTimeline.Code != tc.wantCode {
				t.Errorf("timeline: expected %d, got %d", tc.wantCode, wTimeline.Code)
			}
			if wDetail.Code != tc.wantCode {
				t.Errorf("detail: expected %d, got %d", tc.wantCode, wDetail.Code)
			}
			if wTimeline.Code != wDetail.Code {
				t.Errorf("inconsistency: timeline returned %d but detail returned %d", wTimeline.Code, wDetail.Code)
			}
		})
	}
}

// TestOrderTimelineAPI_NilCreatedBy verifies that an order with nil CreatedBy
// is denied to scoped users (no owner match possible) but allowed for global readers.
func TestOrderTimelineAPI_NilCreatedBy(t *testing.T) {
	h := &Handlers{}
	order := &orders.Order{ID: 42, OrderNumber: "ORD-042", CreatedBy: nil}

	// Scoped user should be denied
	r := gin.New()
	r.Use(injectUser(3, "Sales User", map[string]bool{
		"order.read": true,
	}, []string{"sales_associate"}))
	r.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for scoped user on nil-creator order, got %d", w.Code)
	}

	// Admin should still be allowed
	r2 := gin.New()
	r2.Use(injectUser(1, "Admin", map[string]bool{
		"order.read":   true,
		"users.manage": true,
	}, []string{"administrator"}))
	r2.GET("/api/orders/:id/timeline", timelineAuthHandler(h, order))

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/api/orders/42/timeline", nil)
	r2.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin on nil-creator order, got %d", w2.Code)
	}
}
