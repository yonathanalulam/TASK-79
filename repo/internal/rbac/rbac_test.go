package rbac

import "testing"

// TestRolePermissionMatrix verifies the static role/permission matrix.
func TestRolePermissionMatrix(t *testing.T) {
	// Administrator must have all permissions
	adminPerms := RolePermissions[RoleAdmin]
	if len(adminPerms) < 20 {
		t.Errorf("admin should have 20+ permissions, got %d", len(adminPerms))
	}

	// Sales associate must NOT have payment permission by default
	salesPerms := make(map[string]bool)
	for _, p := range RolePermissions[RoleSalesAssociate] {
		salesPerms[p] = true
	}
	if salesPerms[PermOrderPayment] {
		t.Error("sales associate should NOT have order.payment permission by default")
	}
	if salesPerms[PermOrderTransition] {
		t.Error("sales associate should NOT have order.transition permission")
	}
	if !salesPerms[PermCartWrite] {
		t.Error("sales associate should have cart.write permission")
	}
	if !salesPerms[PermOrderCreate] {
		t.Error("sales associate should have order.create permission")
	}

	// Auditor must be read-only
	auditorPerms := make(map[string]bool)
	for _, p := range RolePermissions[RoleAuditor] {
		auditorPerms[p] = true
	}
	writePerms := []string{
		PermCatalogWrite, PermCatalogPublish, PermCartWrite, PermCartMerge,
		PermOrderCreate, PermOrderTransition, PermOrderPayment, PermOrderSplit,
		PermAlertManage, PermMetricWrite, PermMetricActivate, PermUserManage,
	}
	for _, wp := range writePerms {
		if auditorPerms[wp] {
			t.Errorf("auditor should NOT have write permission %q", wp)
		}
	}
	if !auditorPerms[PermAuditRead] {
		t.Error("auditor should have audit.read permission")
	}

	// Inventory manager should have alert.manage but not order.payment
	invPerms := make(map[string]bool)
	for _, p := range RolePermissions[RoleInventoryManager] {
		invPerms[p] = true
	}
	if !invPerms[PermAlertManage] {
		t.Error("inventory manager should have alert.manage")
	}
	if invPerms[PermOrderPayment] {
		t.Error("inventory manager should NOT have order.payment")
	}
}

// TestAllRolesAreDefined verifies all expected roles exist in the matrix.
func TestAllRolesAreDefined(t *testing.T) {
	expected := []string{RoleAdmin, RoleInventoryManager, RoleSalesAssociate, RoleAuditor}
	for _, role := range expected {
		if _, ok := RolePermissions[role]; !ok {
			t.Errorf("role %q not found in RolePermissions", role)
		}
	}
}
