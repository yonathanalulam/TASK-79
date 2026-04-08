package rbac

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Permission codes
const (
	PermUserManage         = "users.manage"
	PermCatalogRead        = "catalog.read"
	PermCatalogWrite       = "catalog.write"
	PermCatalogPublish     = "catalog.publish"
	PermCatalogImport      = "catalog.import"
	PermMediaUpload        = "media.upload"
	PermCartRead           = "cart.read"
	PermCartWrite          = "cart.write"
	PermCartMerge          = "cart.merge"
	PermOrderRead          = "order.read"
	PermOrderCreate        = "order.create"
	PermOrderTransition    = "order.transition"
	PermOrderNotes         = "order.notes"
	PermOrderPayment       = "order.payment"
	PermOrderSplit         = "order.split"
	PermNotificationRead   = "notification.read"
	PermNotificationManage = "notification.manage"
	PermAlertRead          = "alert.read"
	PermAlertManage        = "alert.manage"
	PermAuditRead          = "audit.read"
	PermMetricRead         = "metric.read"
	PermMetricWrite        = "metric.write"
	PermMetricActivate     = "metric.activate"
	PermDashboardRead      = "dashboard.read"
	PermSystemConfig       = "system.config"
)

// Roles
const (
	RoleAdmin            = "administrator"
	RoleInventoryManager = "inventory_manager"
	RoleSalesAssociate   = "sales_associate"
	RoleAuditor          = "auditor"
)

// RolePermissions defines the permission matrix
var RolePermissions = map[string][]string{
	RoleAdmin: {
		PermUserManage, PermCatalogRead, PermCatalogWrite, PermCatalogPublish, PermCatalogImport,
		PermMediaUpload, PermCartRead, PermCartWrite, PermCartMerge,
		PermOrderRead, PermOrderCreate, PermOrderTransition, PermOrderNotes, PermOrderPayment, PermOrderSplit,
		PermNotificationRead, PermNotificationManage,
		PermAlertRead, PermAlertManage, PermAuditRead,
		PermMetricRead, PermMetricWrite, PermMetricActivate,
		PermDashboardRead, PermSystemConfig,
	},
	RoleInventoryManager: {
		PermCatalogRead, PermCatalogWrite, PermCatalogPublish, PermCatalogImport,
		PermMediaUpload,
		PermOrderRead, PermOrderTransition,
		PermNotificationRead,
		PermAlertRead, PermAlertManage,
		PermDashboardRead,
	},
	RoleSalesAssociate: {
		PermCatalogRead,
		PermCartRead, PermCartWrite, PermCartMerge,
		PermOrderRead, PermOrderCreate, PermOrderNotes,
		PermNotificationRead,
		PermDashboardRead,
	},
	RoleAuditor: {
		PermCatalogRead,
		PermCartRead,
		PermOrderRead,
		PermNotificationRead,
		PermAlertRead,
		PermAuditRead,
		PermMetricRead,
		PermDashboardRead,
	},
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// UserHasPermission checks if a user has a specific permission via their roles
func (s *Service) UserHasPermission(ctx context.Context, userID int, permCode string) (bool, error) {
	query := `SELECT COUNT(*) FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1 AND p.code = $2`

	var count int
	err := s.pool.QueryRow(ctx, query, userID, permCode).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UserPermissions returns all permission codes for a user
func (s *Service) UserPermissions(ctx context.Context, userID int) ([]string, error) {
	query := `SELECT DISTINCT p.code FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}
	return perms, nil
}

// UserRoles returns the role names for a user
func (s *Service) UserRoles(ctx context.Context, userID int) ([]string, error) {
	query := `SELECT r.name FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, nil
}
