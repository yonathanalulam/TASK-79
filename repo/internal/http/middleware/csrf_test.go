package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupCSRFRouter() *gin.Engine {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.GET("/page", func(c *gin.Context) {
		c.JSON(200, gin.H{"csrf": GetCSRFToken(c)})
	})
	r.POST("/mutate", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.DELETE("/remove", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	return r
}

func TestCSRF_GetSetsToken(t *testing.T) {
	r := setupCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/page", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should have set-cookie for csrf_token
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "csrf_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected csrf_token cookie to be set on GET")
	}
}

func TestCSRF_PostWithoutTokenRejected(t *testing.T) {
	r := setupCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mutate", strings.NewReader("foo=bar"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403 for POST without CSRF token, got %d", w.Code)
	}
}

func TestCSRF_PostWithCookieButNoFormTokenRejected(t *testing.T) {
	r := setupCSRFRouter()

	// Step 1: GET to obtain token
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/page", nil))
	var token string
	for _, c := range w1.Result().Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}

	// Step 2: POST with cookie but no form token
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mutate", strings.NewReader("foo=bar"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	r.ServeHTTP(w2, req)

	if w2.Code != 403 {
		t.Errorf("expected 403 without form token, got %d", w2.Code)
	}
}

func TestCSRF_PostWithMatchingTokenAccepted(t *testing.T) {
	r := setupCSRFRouter()

	// Step 1: GET to obtain token
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/page", nil))
	var token string
	for _, c := range w1.Result().Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}

	// Step 2: POST with matching cookie + form token
	w2 := httptest.NewRecorder()
	body := "csrf_token=" + token + "&foo=bar"
	req := httptest.NewRequest("POST", "/mutate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	r.ServeHTTP(w2, req)

	if w2.Code != 200 {
		t.Errorf("expected 200 for valid CSRF, got %d", w2.Code)
	}
}

func TestCSRF_PostWithHeaderTokenAccepted(t *testing.T) {
	r := setupCSRFRouter()

	// GET to obtain token
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/page", nil))
	var token string
	for _, c := range w1.Result().Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}

	// POST with X-CSRF-Token header (HTMX path)
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mutate", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	r.ServeHTTP(w2, req)

	if w2.Code != 200 {
		t.Errorf("expected 200 for CSRF via header, got %d", w2.Code)
	}
}

func TestCSRF_DeleteWithoutTokenRejected(t *testing.T) {
	r := setupCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/remove", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403 for DELETE without CSRF, got %d", w.Code)
	}
}

func TestCSRF_MismatchedTokenRejected(t *testing.T) {
	r := setupCSRFRouter()

	w := httptest.NewRecorder()
	body := "csrf_token=wrong_token"
	req := httptest.NewRequest("POST", "/mutate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "real_token"})
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403 for mismatched tokens, got %d", w.Code)
	}
}
