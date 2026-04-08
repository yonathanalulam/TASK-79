package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const CSRFTokenKey = "csrf_token"

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			// Only generate a new token if one doesn't already exist.
			// This prevents race conditions with hx-boost navigation.
			token, err := c.Cookie("csrf_token")
			if err != nil || token == "" {
				token = generateCSRFToken()
				c.SetCookie("csrf_token", token, 3600, "/", "", false, false)
			}
			c.Set(CSRFTokenKey, token)
			c.Next()
			return
		}

		// Validate token for all mutating requests (POST/PUT/DELETE/PATCH)
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodDelete || c.Request.Method == http.MethodPatch {

			cookieToken, err := c.Cookie("csrf_token")
			if err != nil || cookieToken == "" {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "CSRF token missing"})
				c.Abort()
				return
			}

			// Accept token from form field or X-CSRF-Token header (for HTMX/JS)
			formToken := c.PostForm("csrf_token")
			if formToken == "" {
				formToken = c.GetHeader("X-CSRF-Token")
			}

			if formToken == "" || cookieToken != formToken {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "CSRF token invalid"})
				c.Abort()
				return
			}

			// Pass validated token into context for templates that need it
			c.Set(CSRFTokenKey, cookieToken)
		}

		c.Next()
	}
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GetCSRFToken(c *gin.Context) string {
	if v, ok := c.Get(CSRFTokenKey); ok {
		return v.(string)
	}
	return ""
}
