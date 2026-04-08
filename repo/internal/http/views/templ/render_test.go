package templ

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestLayoutHTMXIsLocal verifies HTMX is served from local static path, not CDN.
func TestLayoutHTMXIsLocal(t *testing.T) {
	pd := PageData{Title: "Test", UserName: "Admin", RoleName: "admin", CSRFToken: "t"}
	component := DashboardPage(pd, DashboardSummary{})
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	html := buf.String()
	if strings.Contains(html, "unpkg.com") || strings.Contains(html, "cdn.") {
		t.Error("layout should not reference external CDN for HTMX")
	}
	if !strings.Contains(html, "/static/js/htmx.min.js") {
		t.Error("layout should reference local HTMX at /static/js/htmx.min.js")
	}
}

// TestLayoutRenders verifies the Templ layout renders without error.
func TestLayoutRenders(t *testing.T) {
	pd := PageData{
		Title:       "Test",
		ActiveNav:   "dashboard",
		UserName:    "Admin User",
		RoleName:    "administrator",
		CSRFToken:   "test-token-123",
		Permissions: map[string]bool{"metric.read": true, "audit.read": true},
		UnreadCount: 5,
	}
	component := DashboardPage(pd, DashboardSummary{OpenCarts: 3, ActiveOrders: 7})
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "FleetCommerce") {
		t.Error("expected FleetCommerce in output")
	}
	if !strings.Contains(html, "Admin User") {
		t.Error("expected user name in output")
	}
	if !strings.Contains(html, "Dashboard") {
		t.Error("expected Dashboard in output")
	}
}

// TestLoginPageRenders verifies the login page renders correctly.
func TestLoginPageRenders(t *testing.T) {
	pd := PageData{Title: "Login", CSRFToken: "abc123"}
	component := LoginPage(pd, "", "")
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "csrf_token") {
		t.Error("expected csrf_token in login form")
	}
	if !strings.Contains(html, "Sign In") {
		t.Error("expected Sign In button")
	}
}

// TestLoginPageWithError verifies error message appears.
func TestLoginPageWithError(t *testing.T) {
	pd := PageData{Title: "Login", CSRFToken: "abc123"}
	component := LoginPage(pd, "Invalid credentials", "admin")
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	html := buf.String()
	if !strings.Contains(html, "Invalid credentials") {
		t.Error("expected error message in output")
	}
}

// TestCatalogPageRenders verifies catalog page with models.
func TestCatalogPageRenders(t *testing.T) {
	pd := PageData{Title: "Catalog", ActiveNav: "catalog", UserName: "Admin", RoleName: "admin", Permissions: map[string]bool{"catalog.write": true, "catalog.import": true}}
	models := []CatalogModel{
		{ID: 1, ModelCode: "TOY-1", ModelName: "Camry", BrandName: "Toyota", SeriesName: "Sedan", Year: 2025, StockQuantity: 10, PublicationStatus: "published"},
	}
	component := CatalogPage(pd, nil, models, Pagination{Page: 1, TotalPages: 1}, "", "")
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Camry") {
		t.Error("expected Camry in catalog output")
	}
	if !strings.Contains(html, "TOY-1") {
		t.Error("expected model code")
	}
}

// TestCartListPageRenders verifies cart list page.
func TestCartListPageRenders(t *testing.T) {
	pd := PageData{Title: "Cart", ActiveNav: "cart", UserName: "Sales", RoleName: "sales", Permissions: map[string]bool{"cart.write": true}}
	carts := []CartView{{ID: 1, CustomerName: "Acme", ItemCount: 3, Status: "open", CreatedAt: "2025-01-01"}}
	component := CartListPage(pd, carts)
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "Acme") {
		t.Error("expected customer name")
	}
}

// TestOrdersPageRenders verifies orders page.
func TestOrdersPageRenders(t *testing.T) {
	pd := PageData{Title: "Orders", ActiveNav: "orders", UserName: "Admin", RoleName: "admin"}
	orders := []OrderView{{ID: 1, OrderNumber: "ORD-001", CustomerName: "Test", Status: "created", CreatedAt: "2025-01-01"}}
	component := OrdersPage(pd, orders, Pagination{Page: 1, TotalPages: 1}, "", "")
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "ORD-001") {
		t.Error("expected order number")
	}
}

// TestAlertsPageRenders verifies alerts page.
func TestAlertsPageRenders(t *testing.T) {
	pd := PageData{Title: "Alerts", ActiveNav: "alerts", UserName: "Admin", RoleName: "admin", Permissions: map[string]bool{"alert.manage": true}}
	alerts := []AlertView{{ID: 1, Title: "Low Stock", Severity: "warning", Status: "open", EntityType: "vehicle_model", EntityID: 5}}
	component := AlertsPage(pd, alerts, "")
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "Low Stock") {
		t.Error("expected alert title")
	}
}

// ===== Notification Center tab tests =====

func notifPD() PageData {
	return PageData{Title: "Notifications", ActiveNav: "notifications", UserName: "Admin", RoleName: "admin",
		CSRFToken: "test-csrf", Permissions: map[string]bool{"notification.read": true}}
}

func TestNotifInboxTab(t *testing.T) {
	notifs := []NotificationView{
		{ID: 1, Type: "order", Title: "Order Created", Body: "ORD-001 placed", IsRead: false, CreatedAt: "2025-01-01 10:00"},
	}
	component := NotificationsPage(notifPD(), "inbox", notifs, nil, nil, nil, nil, false)
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Order Created") {
		t.Error("expected notification title in inbox")
	}
	if !strings.Contains(html, "unread") {
		t.Error("expected unread class for unread notification")
	}
	if !strings.Contains(html, "Mark All Read") {
		t.Error("expected bulk mark-as-read button")
	}
}

func TestNotifAnnouncementsTab(t *testing.T) {
	anns := []AnnouncementView{
		{ID: 1, Title: "System Update", Body: "Scheduled maintenance tonight", Priority: "high", CreatedAt: "2025-01-01"},
	}
	component := NotificationsPage(notifPD(), "announcements", nil, anns, nil, nil, nil, false)
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "System Update") {
		t.Error("expected announcement title")
	}
	if !strings.Contains(html, "Scheduled maintenance tonight") {
		t.Error("expected announcement body")
	}
	if !strings.Contains(html, "priority-high") {
		t.Error("expected priority class")
	}
}

func TestNotifPreferencesTab(t *testing.T) {
	prefs := []PreferenceView{
		{Channel: "in_app", EventType: "order_created", Enabled: true},
		{Channel: "email", EventType: "alert_opened", Enabled: true},
	}
	types := []string{"order_created", "order_transition", "alert_opened"}
	component := NotificationsPage(notifPD(), "preferences", nil, nil, prefs, types, nil, false)
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Save Preferences") {
		t.Error("expected save button in preferences tab")
	}
	if !strings.Contains(html, "order_created") {
		t.Error("expected event type in preferences table")
	}
	if !strings.Contains(html, `name="in_app_order_created"`) {
		t.Error("expected checkbox field name")
	}
}

func TestNotifExportQueueTab_Authorized(t *testing.T) {
	queue := []ExportQueueView{
		{ID: 1, Channel: "email", Recipient: "user@example.com", Status: "pending", Attempts: 0, MaxAttempts: 3, CreatedAt: "2025-01-01"},
	}
	// canManageExports=true: export queue tab and data should render
	component := NotificationsPage(notifPD(), "export-queue", nil, nil, nil, nil, queue, true)
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "user@example.com") {
		t.Error("expected recipient in export queue")
	}
	if !strings.Contains(html, "0/3") {
		t.Error("expected attempts display")
	}
	if !strings.Contains(html, "Export Queue") {
		t.Error("expected Export Queue tab link for authorized user")
	}
}

func TestNotifExportQueueTab_Unauthorized_NoTab(t *testing.T) {
	// canManageExports=false: export queue tab link must NOT appear
	component := NotificationsPage(notifPD(), "inbox", nil, nil, nil, nil, nil, false)
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	html := buf.String()
	if strings.Contains(html, "tab=export-queue") {
		t.Error("export queue tab link should NOT be visible for unauthorized user")
	}
}

func TestNotifExportQueueTab_Authorized_TabVisible(t *testing.T) {
	// canManageExports=true: export queue tab link MUST appear
	component := NotificationsPage(notifPD(), "inbox", nil, nil, nil, nil, nil, true)
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	html := buf.String()
	if !strings.Contains(html, "tab=export-queue") {
		t.Error("export queue tab link should be visible for authorized user")
	}
}

func TestNotifExportQueueEmptyState(t *testing.T) {
	component := NotificationsPage(notifPD(), "export-queue", nil, nil, nil, nil, nil, true)
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	if !strings.Contains(buf.String(), "No export queue items") {
		t.Error("expected empty state for export queue")
	}
}

func TestNotifAnnouncementReadUnreadState(t *testing.T) {
	anns := []AnnouncementView{
		{ID: 1, Title: "Read One", IsRead: true, Priority: "normal", CreatedAt: "2025-01-01"},
		{ID: 2, Title: "Unread One", IsRead: false, Priority: "high", CreatedAt: "2025-01-02"},
	}
	component := NotificationsPage(notifPD(), "announcements", nil, anns, nil, nil, nil, false)
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	html := buf.String()
	if !strings.Contains(html, "Unread One") {
		t.Error("expected unread announcement")
	}
	// Unread announcement should have unread class
	if !strings.Contains(html, "unread") {
		t.Error("expected unread CSS class for unread announcement")
	}
	// Read announcement should not have the unread badge
	if strings.Count(html, `<span class="badge">unread</span>`) != 1 {
		t.Error("expected exactly one unread badge (for the unread announcement)")
	}
}

func TestNotifAnnouncementsEmptyState(t *testing.T) {
	component := NotificationsPage(notifPD(), "announcements", nil, nil, nil, nil, nil, false)
	var buf bytes.Buffer
	component.Render(context.Background(), &buf)
	if !strings.Contains(buf.String(), "No active announcements") {
		t.Error("expected empty state for announcements")
	}
}
