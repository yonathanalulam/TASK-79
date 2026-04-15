package endpointcoverage

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"fleetcommerce/internal/testutil"
)

// ---------- Route inventory (central source of truth for 88 endpoints) ----------

type endpoint struct{ Method, Path string }

var allEndpoints = []endpoint{
	// Public (2)
	{"GET", "/login"},
	{"POST", "/login"},
	// Authenticated HTML pages (21)
	{"POST", "/logout"},
	{"GET", "/"},
	{"GET", "/catalog"},
	{"GET", "/catalog/new"},
	{"GET", "/catalog/import"},
	{"GET", "/catalog/:id"},
	{"GET", "/catalog/:id/edit"},
	{"GET", "/cart"},
	{"GET", "/cart/:id"},
	{"GET", "/cart/:id/merge-modal"},
	{"GET", "/cart/:id/add-item"},
	{"GET", "/orders"},
	{"GET", "/orders/:id"},
	{"GET", "/orders/:id/split-modal"},
	{"GET", "/notifications"},
	{"GET", "/alerts"},
	{"GET", "/alerts/:id/close-modal"},
	{"GET", "/metrics"},
	{"GET", "/metrics/:id"},
	{"GET", "/metrics/new"},
	{"GET", "/audit"},
	// API (65)
	{"GET", "/api/me"},
	{"GET", "/api/dashboard/summary"},
	{"GET", "/api/catalog/brands"},
	{"GET", "/api/catalog/series"},
	{"GET", "/api/catalog/models"},
	{"GET", "/api/catalog/models/:id"},
	{"POST", "/api/catalog/models"},
	{"PUT", "/api/catalog/models/:id/draft"},
	{"POST", "/api/catalog/models/:id/draft"},
	{"POST", "/api/catalog/models/:id/publish"},
	{"POST", "/api/catalog/models/:id/unpublish"},
	{"POST", "/api/catalog/models/:id/media"},
	{"GET", "/api/catalog/export.csv"},
	{"POST", "/api/catalog/imports"},
	{"GET", "/api/catalog/imports/:job_id"},
	{"POST", "/api/catalog/imports/:job_id/commit"},
	{"GET", "/api/carts"},
	{"POST", "/api/carts"},
	{"GET", "/api/carts/:id"},
	{"POST", "/api/carts/:id/items"},
	{"PUT", "/api/carts/:id/items/:item_id"},
	{"DELETE", "/api/carts/:id/items/:item_id"},
	{"POST", "/api/carts/:id/merge"},
	{"POST", "/api/carts/:id/revalidate"},
	{"POST", "/api/carts/:id/checkout"},
	{"GET", "/api/orders"},
	{"GET", "/api/orders/:id"},
	{"POST", "/api/orders/:id/notes"},
	{"POST", "/api/orders/:id/payment-recorded"},
	{"POST", "/api/orders/:id/transition"},
	{"POST", "/api/orders/:id/split"},
	{"GET", "/api/orders/:id/timeline"},
	{"GET", "/api/notifications"},
	{"POST", "/api/notifications/:id/read"},
	{"POST", "/api/notifications/bulk-read"},
	{"GET", "/api/announcements"},
	{"POST", "/api/announcements/:id/read"},
	{"GET", "/api/notification-preferences"},
	{"PUT", "/api/notification-preferences"},
	{"POST", "/api/notification-preferences"},
	{"GET", "/api/export-queue"},
	{"GET", "/api/alerts"},
	{"POST", "/api/alerts/:id/claim"},
	{"POST", "/api/alerts/:id/process"},
	{"POST", "/api/alerts/:id/close"},
	{"POST", "/api/alerts/evaluate"},
	{"GET", "/api/metrics"},
	{"POST", "/api/metrics"},
	{"GET", "/api/metrics/:id"},
	{"PUT", "/api/metrics/:id"},
	{"GET", "/api/metrics/:id/versions"},
	{"GET", "/api/metrics/:id/dimensions"},
	{"POST", "/api/metrics/:id/dimensions"},
	{"DELETE", "/api/metrics/:id/dimensions/:dim_id"},
	{"GET", "/api/metrics/:id/filters"},
	{"POST", "/api/metrics/:id/filters"},
	{"DELETE", "/api/metrics/:id/filters/:filter_id"},
	{"GET", "/api/metrics/:id/dependencies"},
	{"POST", "/api/metrics/:id/dependencies"},
	{"DELETE", "/api/metrics/:id/dependencies/:dep_id"},
	{"POST", "/api/metrics/:id/impact-analysis"},
	{"POST", "/api/metrics/:id/activate"},
	{"GET", "/api/metrics/:id/lineage"},
	{"GET", "/api/audit"},
	{"GET", "/api/audit/:entity_type/:entity_id"},
}

// ---------- Route inventory assertion ----------

func TestRouteInventory(t *testing.T) {
	app := testutil.MustApp(t)

	registered := map[string]bool{}
	for _, ri := range app.Router.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	if len(allEndpoints) != 88 {
		t.Fatalf("expected 88 endpoint definitions, got %d", len(allEndpoints))
	}

	missing := 0
	for _, ep := range allEndpoints {
		key := ep.Method + " " + ep.Path
		if !registered[key] {
			t.Errorf("production route missing from router: %s", key)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d production routes not registered", missing)
	}
	t.Logf("all 88 endpoints registered in router")
}

// ---------- Full endpoint coverage ----------

type epTest struct {
	name       string
	method     string
	path       string
	form       url.Values
	json       string
	wantStatus int
	wantBody   string // substring that must appear in response body
}

func TestEndpointCoverage(t *testing.T) {
	app := testutil.MustApp(t)
	e := testutil.SeedTestEntities(t, app.Pool)
	client := app.AuthClient(t, "admin", "password123")

	tests := []epTest{
		// ========== PUBLIC ENDPOINTS (2) ==========
		{"GET /login", "GET", "/login", nil, "", http.StatusOK, "Login"},
		{"POST /login", "POST", "/login", url.Values{"username": {"admin"}, "password": {"password123"}}, "", http.StatusFound, ""},

		// ========== AUTHENTICATED HTML PAGES (20, logout tested separately) ==========
		{"GET / (dashboard)", "GET", "/", nil, "", http.StatusOK, "Dashboard"},
		{"GET /catalog", "GET", "/catalog", nil, "", http.StatusOK, "Catalog"},
		{"GET /catalog/new", "GET", "/catalog/new", nil, "", http.StatusOK, ""},
		{"GET /catalog/import", "GET", "/catalog/import", nil, "", http.StatusOK, ""},
		{"GET /catalog/:id", "GET", fmt.Sprintf("/catalog/%d", e.ModelID), nil, "", http.StatusOK, ""},
		{"GET /catalog/:id/edit", "GET", fmt.Sprintf("/catalog/%d/edit", e.DraftModelID), nil, "", http.StatusOK, ""},
		{"GET /cart", "GET", "/cart", nil, "", http.StatusOK, "Cart"},
		{"GET /cart/:id", "GET", fmt.Sprintf("/cart/%d", e.CartID), nil, "", http.StatusOK, "Cart"},
		{"GET /cart/:id/merge-modal", "GET", fmt.Sprintf("/cart/%d/merge-modal", e.CartID), nil, "", http.StatusOK, ""},
		{"GET /cart/:id/add-item", "GET", fmt.Sprintf("/cart/%d/add-item", e.CartID), nil, "", http.StatusOK, ""},
		{"GET /orders", "GET", "/orders", nil, "", http.StatusOK, "Order"},
		{"GET /orders/:id", "GET", fmt.Sprintf("/orders/%d", e.OrderID), nil, "", http.StatusOK, "Order"},
		{"GET /orders/:id/split-modal", "GET", fmt.Sprintf("/orders/%d/split-modal", e.OrderID), nil, "", http.StatusOK, ""},
		{"GET /notifications", "GET", "/notifications", nil, "", http.StatusOK, "Notification"},
		{"GET /alerts", "GET", "/alerts", nil, "", http.StatusOK, "Alert"},
		{"GET /alerts/:id/close-modal", "GET", fmt.Sprintf("/alerts/%d/close-modal", e.AlertID), nil, "", http.StatusOK, ""},
		{"GET /metrics", "GET", "/metrics", nil, "", http.StatusOK, ""},
		{"GET /metrics/:id", "GET", fmt.Sprintf("/metrics/%d", e.MetricID), nil, "", http.StatusOK, ""},
		{"GET /metrics/new", "GET", "/metrics/new", nil, "", http.StatusOK, ""},
		{"GET /audit", "GET", "/audit", nil, "", http.StatusOK, ""},

		// ========== API GET ENDPOINTS (27) ==========
		{"GET /api/me", "GET", "/api/me", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/dashboard/summary", "GET", "/api/dashboard/summary", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/catalog/brands", "GET", "/api/catalog/brands", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/catalog/series", "GET", "/api/catalog/series", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/catalog/models", "GET", "/api/catalog/models", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/catalog/models/:id", "GET", fmt.Sprintf("/api/catalog/models/%d", e.ModelID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/catalog/export.csv", "GET", "/api/catalog/export.csv", nil, "", http.StatusOK, "model_code"},
		{"GET /api/catalog/imports/:job_id", "GET", fmt.Sprintf("/api/catalog/imports/%d", e.ImportJobID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/carts", "GET", "/api/carts", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/carts/:id", "GET", fmt.Sprintf("/api/carts/%d", e.CartID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/orders", "GET", "/api/orders", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/orders/:id", "GET", fmt.Sprintf("/api/orders/%d", e.OrderID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/orders/:id/timeline", "GET", fmt.Sprintf("/api/orders/%d/timeline", e.OrderID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/notifications", "GET", "/api/notifications", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/announcements", "GET", "/api/announcements", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/notification-preferences", "GET", "/api/notification-preferences", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/export-queue", "GET", "/api/export-queue", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/alerts", "GET", "/api/alerts", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics", "GET", "/api/metrics", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id", "GET", fmt.Sprintf("/api/metrics/%d", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id/versions", "GET", fmt.Sprintf("/api/metrics/%d/versions", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id/dimensions", "GET", fmt.Sprintf("/api/metrics/%d/dimensions", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id/filters", "GET", fmt.Sprintf("/api/metrics/%d/filters", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id/dependencies", "GET", fmt.Sprintf("/api/metrics/%d/dependencies", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/metrics/:id/lineage", "GET", fmt.Sprintf("/api/metrics/%d/lineage", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/audit", "GET", "/api/audit", nil, "", http.StatusOK, `"ok":true`},
		{"GET /api/audit/:entity_type/:entity_id", "GET", fmt.Sprintf("/api/audit/order/%d", e.OrderID), nil, "", http.StatusOK, `"ok":true`},

		// ========== API MUTATION — JSON responses (14) ==========
		{"POST /api/notifications/:id/read", "POST", fmt.Sprintf("/api/notifications/%d/read", e.NotificationID), nil, "", http.StatusOK, `"ok":true`},
		{"POST /api/announcements/:id/read", "POST", fmt.Sprintf("/api/announcements/%d/read", e.AnnouncementID), nil, "", http.StatusOK, `"ok":true`},
		{"PUT /api/carts/:id/items/:item_id", "PUT",
			fmt.Sprintf("/api/carts/%d/items/%d", e.CartID, e.CartItemID),
			url.Values{"quantity": {"2"}}, "", http.StatusOK, `"ok":true`},
		// DELETE metric sub-objects BEFORE update/add (which may replace seeded objects)
		{"DELETE /api/metrics/:id/dimensions/:dim_id", "DELETE",
			fmt.Sprintf("/api/metrics/%d/dimensions/%d", e.MetricID, e.DimensionID), nil, "",
			http.StatusOK, `"ok":true`},
		{"DELETE /api/metrics/:id/filters/:filter_id", "DELETE",
			fmt.Sprintf("/api/metrics/%d/filters/%d", e.MetricID, e.FilterID), nil, "",
			http.StatusOK, `"ok":true`},
		{"DELETE /api/metrics/:id/dependencies/:dep_id", "DELETE",
			fmt.Sprintf("/api/metrics/%d/dependencies/%d", e.MetricID, e.DependencyID), nil, "",
			http.StatusOK, `"ok":true`},
		// Now add/update metric sub-objects
		{"PUT /api/metrics/:id", "PUT",
			fmt.Sprintf("/api/metrics/%d", e.MetricID), nil,
			`{"description":"updated via test","sql_expression":"SELECT 1","time_grain":"daily"}`,
			http.StatusOK, `"ok":true`},
		{"POST /api/metrics/:id/dimensions", "POST",
			fmt.Sprintf("/api/metrics/%d/dimensions", e.MetricID), nil,
			`{"name":"coverage_dim","description":"added by coverage test"}`,
			http.StatusOK, `"ok":true`},
		{"POST /api/metrics/:id/filters", "POST",
			fmt.Sprintf("/api/metrics/%d/filters", e.MetricID), nil,
			`{"name":"coverage_filter","expression":"x > 0"}`,
			http.StatusOK, `"ok":true`},
		{"POST /api/metrics/:id/dependencies", "POST",
			fmt.Sprintf("/api/metrics/%d/dependencies", e.MetricID2), nil,
			fmt.Sprintf(`{"depends_on_metric_id":%d}`, e.MetricID),
			http.StatusOK, `"ok":true`},
		{"POST /api/metrics/:id/impact-analysis", "POST",
			fmt.Sprintf("/api/metrics/%d/impact-analysis", e.MetricID), nil, "", http.StatusOK, `"ok":true`},
		// File-upload endpoints — handler-level error (no file) proves real handler reached
		{"POST /api/catalog/models/:id/media (no file)", "POST",
			fmt.Sprintf("/api/catalog/models/%d/media", e.ModelID), nil, "",
			http.StatusBadRequest, `"ok":false`},
		{"POST /api/catalog/imports (no file)", "POST",
			"/api/catalog/imports", nil, "",
			http.StatusBadRequest, `"ok":false`},
		// Note validation — empty content returns 400 from real handler
		{"POST /api/orders/:id/notes (empty)", "POST",
			fmt.Sprintf("/api/orders/%d/notes", e.OrderID), url.Values{}, "",
			http.StatusBadRequest, "Note content is required"},

		// ========== API MUTATION — Redirect responses (24) ==========
		{"POST /api/catalog/models", "POST", "/api/catalog/models",
			url.Values{"brand_id": {"1"}, "series_id": {"1"}, "model_code": {"COV-TEST-001"}, "model_name": {"Coverage Test"}, "year": {"2025"}, "stock_quantity": {"10"}},
			"", http.StatusFound, ""},
		{"PUT /api/catalog/models/:id/draft", "PUT",
			fmt.Sprintf("/api/catalog/models/%d/draft", e.DraftModelID),
			url.Values{"model_name": {"Draft Updated"}, "year": {"2025"}, "stock_quantity": {"5"}},
			"", http.StatusFound, ""},
		{"POST /api/catalog/models/:id/draft", "POST",
			fmt.Sprintf("/api/catalog/models/%d/draft", e.DraftModelID),
			url.Values{"model_name": {"Draft Updated 2"}, "year": {"2025"}, "stock_quantity": {"5"}},
			"", http.StatusFound, ""},
		{"POST /api/catalog/models/:id/publish", "POST",
			fmt.Sprintf("/api/catalog/models/%d/publish", e.DraftModelID), nil, "", http.StatusFound, ""},
		{"POST /api/catalog/models/:id/unpublish", "POST",
			fmt.Sprintf("/api/catalog/models/%d/unpublish", e.DraftModelID), nil, "", http.StatusFound, ""},
		{"POST /api/catalog/imports/:job_id/commit", "POST",
			fmt.Sprintf("/api/catalog/imports/%d/commit", e.ImportJobID), nil, "", http.StatusFound, ""},
		{"POST /api/carts", "POST", "/api/carts",
			url.Values{"customer_account_id": {fmt.Sprintf("%d", e.CustomerID)}},
			"", http.StatusFound, ""},
		{"POST /api/carts/:id/items", "POST",
			fmt.Sprintf("/api/carts/%d/items", e.CartID),
			url.Values{"vehicle_model_id": {fmt.Sprintf("%d", e.ModelID)}, "quantity": {"1"}},
			"", http.StatusFound, ""},
		{"POST /api/carts/:id/merge", "POST",
			fmt.Sprintf("/api/carts/%d/merge", e.CartID),
			url.Values{"source_cart_id": {fmt.Sprintf("%d", e.CartID2)}},
			"", http.StatusFound, ""},
		{"POST /api/carts/:id/revalidate", "POST",
			fmt.Sprintf("/api/carts/%d/revalidate", e.CartID), nil, "", http.StatusFound, ""},
		{"POST /api/carts/:id/checkout", "POST",
			fmt.Sprintf("/api/carts/%d/checkout", e.CartID), nil, "", http.StatusFound, ""},
		{"DELETE /api/carts/:id/items/:item_id", "DELETE",
			fmt.Sprintf("/api/carts/%d/items/%d", e.CartID, e.CartItemID), nil, "",
			http.StatusFound, ""},
		{"POST /api/orders/:id/notes (valid)", "POST",
			fmt.Sprintf("/api/orders/%d/notes", e.OrderID),
			url.Values{"note_type": {"internal"}, "content": {"Coverage test note"}},
			"", http.StatusFound, ""},
		{"POST /api/orders/:id/payment-recorded", "POST",
			fmt.Sprintf("/api/orders/%d/payment-recorded", e.OrderID), nil, "", http.StatusFound, ""},
		{"POST /api/orders/:id/transition", "POST",
			fmt.Sprintf("/api/orders/%d/transition", e.OrderID),
			url.Values{"to_status": {"payment_recorded"}}, "", http.StatusFound, ""},
		{"POST /api/orders/:id/split", "POST",
			fmt.Sprintf("/api/orders/%d/split", e.OrderID), nil, "", http.StatusFound, ""},
		{"POST /api/notifications/bulk-read", "POST", "/api/notifications/bulk-read", nil, "", http.StatusFound, ""},
		{"PUT /api/notification-preferences", "PUT", "/api/notification-preferences",
			url.Values{"in_app_order_created": {"on"}}, "", http.StatusFound, ""},
		{"POST /api/notification-preferences", "POST", "/api/notification-preferences",
			url.Values{"in_app_order_created": {"on"}}, "", http.StatusFound, ""},
		{"POST /api/alerts/:id/claim", "POST",
			fmt.Sprintf("/api/alerts/%d/claim", e.AlertID), nil, "", http.StatusFound, ""},
		{"POST /api/alerts/:id/process", "POST",
			fmt.Sprintf("/api/alerts/%d/process", e.AlertID2), nil, "", http.StatusFound, ""},
		{"POST /api/alerts/:id/close", "POST",
			fmt.Sprintf("/api/alerts/%d/close", e.AlertID3),
			url.Values{"resolution_notes": {"Resolved via coverage test"}},
			"", http.StatusFound, ""},
		{"POST /api/alerts/evaluate", "POST", "/api/alerts/evaluate", nil, "", http.StatusFound, ""},
		{"POST /api/metrics (form)", "POST", "/api/metrics",
			url.Values{"name": {"Coverage Test Metric"}, "description": {"metric from coverage test"}, "sql_expression": {"SELECT 1"}, "time_grain": {"daily"}},
			"", http.StatusFound, ""},
		{"POST /api/metrics/:id/activate", "POST",
			fmt.Sprintf("/api/metrics/%d/activate", e.MetricID), nil, "", http.StatusFound, ""},
	}

	if len(tests) != 88 {
		t.Fatalf("expected 88 test cases, got %d — update test table to match allEndpoints", len(tests))
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var resp *http.Response
			switch {
			case tc.json != "":
				if tc.method == "PUT" {
					resp = client.PutJSON(tc.path, tc.json)
				} else {
					resp = client.PostJSON(tc.path, tc.json)
				}
			case tc.form != nil && tc.method == "PUT":
				resp = client.PutForm(tc.path, tc.form)
			case tc.form != nil:
				resp = client.PostForm(tc.path, tc.form)
			case tc.method == "DELETE":
				resp = client.Delete(tc.path)
			case tc.method == "POST":
				resp = client.PostForm(tc.path, nil)
			default: // GET
				resp = client.GET(tc.path)
			}

			if resp == nil {
				t.Fatal("nil response — HTTP request failed")
			}
			body := testutil.ReadBody(resp)

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d\nbody (first 300 chars): %.300s", resp.StatusCode, tc.wantStatus, body)
			}
			if tc.wantBody != "" && !strings.Contains(body, tc.wantBody) {
				t.Errorf("body missing %q\nbody (first 500 chars): %.500s", tc.wantBody, body)
			}
		})
	}
}

// ---------- POST /logout (tested with dedicated client to preserve main session) ----------

func TestLogout(t *testing.T) {
	app := testutil.MustApp(t)
	lc := app.AuthClient(t, "admin", "password123")

	resp := lc.PostForm("/logout", nil)
	body := testutil.ReadBody(resp)

	if resp.StatusCode != http.StatusFound {
		t.Errorf("POST /logout: status = %d, want 302\nbody: %.200s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("POST /logout: Location = %q, want /login", loc)
	}
}

// ---------- Auth enforcement — unauthenticated access ----------

func TestUnauthenticatedAccess(t *testing.T) {
	app := testutil.MustApp(t)

	// Representative sample: one HTML page, one API endpoint
	endpoints := []struct {
		method, path string
		wantStatus   int
		desc         string
	}{
		{"GET", "/", http.StatusFound, "dashboard should redirect to login"},
		{"GET", "/api/me", http.StatusUnauthorized, "API should return 401"},
		{"GET", "/api/orders", http.StatusUnauthorized, "API orders should return 401"},
		{"GET", "/catalog", http.StatusFound, "catalog page should redirect to login"},
	}

	for _, ep := range endpoints {
		t.Run(ep.desc, func(t *testing.T) {
			// Fresh client with no session
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			var resp *http.Response
			var err error
			reqURL := app.Server.URL + ep.path
			if ep.method == "GET" {
				resp, err = client.Get(reqURL)
			} else {
				resp, err = client.Post(reqURL, "application/x-www-form-urlencoded", nil)
			}
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != ep.wantStatus {
				t.Errorf("unauthenticated %s %s: status = %d, want %d", ep.method, ep.path, resp.StatusCode, ep.wantStatus)
			}
		})
	}
}

// ---------- CSRF enforcement ----------

func TestCSRFEnforcement(t *testing.T) {
	app := testutil.MustApp(t)
	e := testutil.SeedTestEntities(t, app.Pool)
	client := app.AuthClient(t, "admin", "password123")

	// Attempt a mutation without CSRF token — should be rejected
	req, _ := http.NewRequest("POST", client.ServerURL+fmt.Sprintf("/api/alerts/%d/claim", e.AlertID), nil)
	// Deliberately do NOT set X-CSRF-Token or csrf_token form field
	resp, err := client.Client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("CSRF enforcement: expected 403, got %d", resp.StatusCode)
	}
}
