package middleware

import (
	"context"
	"net/http"

	"fleetcommerce/internal/auth"
	"fleetcommerce/internal/rbac"

	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	UserKey        contextKey = "user"
	PermissionsKey contextKey = "permissions"
	RolesKey       contextKey = "roles"
	SessionIDKey   contextKey = "session_id"
)

func AuthMiddleware(authSvc *auth.Service, rbacSvc *rbac.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil || sessionID == "" {
			handleUnauth(c)
			return
		}

		user, err := authSvc.ValidateSession(c.Request.Context(), sessionID)
		if err != nil {
			c.SetCookie("session_id", "", -1, "/", "", false, true)
			handleUnauth(c)
			return
		}

		perms, _ := rbacSvc.UserPermissions(c.Request.Context(), user.ID)
		roles, _ := rbacSvc.UserRoles(c.Request.Context(), user.ID)

		permSet := make(map[string]bool)
		for _, p := range perms {
			permSet[p] = true
		}

		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, UserKey, user)
		ctx = context.WithValue(ctx, PermissionsKey, permSet)
		ctx = context.WithValue(ctx, RolesKey, roles)
		ctx = context.WithValue(ctx, SessionIDKey, sessionID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permSet := GetPermissions(c.Request.Context())
		if permSet == nil || !permSet[perm] {
			if isHTMX(c) || isAPI(c) {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "Insufficient permissions"})
			} else {
				c.HTML(http.StatusForbidden, "", nil)
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

func GetUser(ctx context.Context) *auth.User {
	if u, ok := ctx.Value(UserKey).(*auth.User); ok {
		return u
	}
	return nil
}

func GetPermissions(ctx context.Context) map[string]bool {
	if p, ok := ctx.Value(PermissionsKey).(map[string]bool); ok {
		return p
	}
	return nil
}

func GetRoles(ctx context.Context) []string {
	if r, ok := ctx.Value(RolesKey).([]string); ok {
		return r
	}
	return nil
}

func GetSessionID(ctx context.Context) string {
	if s, ok := ctx.Value(SessionIDKey).(string); ok {
		return s
	}
	return ""
}

func handleUnauth(c *gin.Context) {
	if isAPI(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "Authentication required"})
		c.Abort()
	} else {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	}
}

func isAPI(c *gin.Context) bool {
	return len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api"
}

func isHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}
