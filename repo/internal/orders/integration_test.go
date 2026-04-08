package orders_test

import (
	"context"
	"testing"

	"fleetcommerce/internal/orders"
	"fleetcommerce/internal/testutil"
)

func TestSplitOrderOnlyEligibleLines(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Order User", "split@test.com")

	// Seed brand, series, vehicle model
	var brandID, seriesID, modelID int
	pool.QueryRow(ctx, `INSERT INTO brands (name) VALUES ('SplitBrand') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&brandID)
	pool.QueryRow(ctx, `INSERT INTO series (brand_id, name) VALUES ($1,'SplitSeries') ON CONFLICT(brand_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, brandID).Scan(&seriesID)
	pool.QueryRow(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, publication_status, stock_quantity) VALUES ($1,$2,'SPLIT01','Split Model',2024,'published',2) RETURNING id`, brandID, seriesID).Scan(&modelID)

	// Seed customer account
	var accountID int
	pool.QueryRow(ctx, `INSERT INTO customer_accounts (name, email, phone) VALUES ('Test Customer','cust@test.com','555-1234') RETURNING id`).Scan(&accountID)

	// Seed cart
	var cartID int
	pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, status) VALUES ($1, 'active') RETURNING id`, accountID).Scan(&cartID)

	// Create an order with a backordered line (request more than stock)
	orderID, _, err := svc.CreateOrder(ctx, orders.CreateOrderParams{
		CustomerAccountID: accountID,
		SourceCartID:      cartID,
		Location:          "warehouse",
		CreatedBy:         userID,
		Lines: []orders.CreateOrderLineParams{
			{VehicleModelID: modelID, QuantityRequested: 5},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// Get lines to find the backordered one
	lines, err := svc.ListOrderLines(ctx, orderID)
	if err != nil {
		t.Fatalf("ListOrderLines: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected at least 1 order line")
	}

	var backorderedLineID int
	for _, l := range lines {
		if l.QuantityBackordered > 0 {
			backorderedLineID = l.ID
		}
	}
	if backorderedLineID == 0 {
		t.Fatal("expected a backordered line")
	}

	// Split should succeed
	childID, err := svc.SplitOrder(ctx, orderID, userID, []int{backorderedLineID})
	if err != nil {
		t.Fatalf("SplitOrder: %v", err)
	}
	if childID == 0 {
		t.Error("expected non-zero child order ID")
	}

	// Verify parent status
	parent, _ := svc.GetOrder(ctx, orderID)
	if parent.Status != orders.StatusSplit {
		t.Errorf("expected parent status split, got %q", parent.Status)
	}

	// Verify audit
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='order' AND entity_id=$1 AND action='split'`, orderID).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for split")
	}
}

func TestSplitOrderRejectsNonBackorderedLine(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Order User", "split-reject@test.com")

	var brandID, seriesID, modelID int
	pool.QueryRow(ctx, `INSERT INTO brands (name) VALUES ('RejectBrand') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&brandID)
	pool.QueryRow(ctx, `INSERT INTO series (brand_id, name) VALUES ($1,'RejectSeries') ON CONFLICT(brand_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, brandID).Scan(&seriesID)
	pool.QueryRow(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, publication_status, stock_quantity) VALUES ($1,$2,'NOSPLIT01','No Split',2024,'published',100) RETURNING id`, brandID, seriesID).Scan(&modelID)

	var accountID int
	pool.QueryRow(ctx, `INSERT INTO customer_accounts (name, email, phone) VALUES ('NoSplit Customer','nosplit@test.com','555-0000') RETURNING id`).Scan(&accountID)
	var cartID int
	pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, status) VALUES ($1, 'active') RETURNING id`, accountID).Scan(&cartID)

	// Create order where stock is sufficient (no backorder)
	orderID, _, err := svc.CreateOrder(ctx, orders.CreateOrderParams{
		CustomerAccountID: accountID,
		SourceCartID:      cartID,
		Location:          "warehouse",
		CreatedBy:         userID,
		Lines: []orders.CreateOrderLineParams{
			{VehicleModelID: modelID, QuantityRequested: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	lines, _ := svc.ListOrderLines(ctx, orderID)
	if len(lines) == 0 {
		t.Fatal("expected order lines")
	}

	// This line is fully allocated, not backordered — split should fail
	_, err = svc.SplitOrder(ctx, orderID, userID, []int{lines[0].ID})
	if err == nil {
		t.Fatal("expected error splitting non-backordered line")
	}
}

func TestSplitOrderEmptyLinesFails(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, pool, auditSvc)

	_, err := svc.SplitOrder(context.Background(), 1, 1, nil)
	if err == nil {
		t.Fatal("expected error for nil lines")
	}
	_, err = svc.SplitOrder(context.Background(), 1, 1, []int{})
	if err == nil {
		t.Fatal("expected error for empty lines")
	}
}

// TestSplitOrderFailureLeavesDataUnchanged verifies that when a split fails
// (e.g. because lines are not backordered), no data is mutated.
func TestSplitOrderFailureLeavesDataUnchanged(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Order User", "split-unchanged@test.com")

	var brandID, seriesID, modelID int
	pool.QueryRow(ctx, `INSERT INTO brands (name) VALUES ('UnchangedBrand') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&brandID)
	pool.QueryRow(ctx, `INSERT INTO series (brand_id, name) VALUES ($1,'UnchangedSeries') ON CONFLICT(brand_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, brandID).Scan(&seriesID)
	pool.QueryRow(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, publication_status, stock_quantity) VALUES ($1,$2,'UNCH01','Unchanged Model',2024,'published',100) RETURNING id`, brandID, seriesID).Scan(&modelID)

	var accountID int
	pool.QueryRow(ctx, `INSERT INTO customer_accounts (name, email, phone) VALUES ('Unchanged Cust','unchanged@test.com','555-8888') RETURNING id`).Scan(&accountID)
	var cartID int
	pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, status) VALUES ($1, 'active') RETURNING id`, accountID).Scan(&cartID)

	orderID, _, err := svc.CreateOrder(ctx, orders.CreateOrderParams{
		CustomerAccountID: accountID,
		SourceCartID:      cartID,
		Location:          "warehouse",
		CreatedBy:         userID,
		Lines: []orders.CreateOrderLineParams{
			{VehicleModelID: modelID, QuantityRequested: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// Capture state before failed split
	orderBefore, _ := svc.GetOrder(ctx, orderID)
	linesBefore, _ := svc.ListOrderLines(ctx, orderID)

	// Try split with fully-allocated line — should fail
	_, err = svc.SplitOrder(ctx, orderID, userID, []int{linesBefore[0].ID})
	if err == nil {
		t.Fatal("expected split to fail for non-backordered line")
	}

	// Verify order status unchanged
	orderAfter, _ := svc.GetOrder(ctx, orderID)
	if orderAfter.Status != orderBefore.Status {
		t.Errorf("order status changed from %q to %q after failed split", orderBefore.Status, orderAfter.Status)
	}

	// Verify lines unchanged
	linesAfter, _ := svc.ListOrderLines(ctx, orderID)
	if len(linesAfter) != len(linesBefore) {
		t.Errorf("line count changed from %d to %d after failed split", len(linesBefore), len(linesAfter))
	}
	if len(linesAfter) > 0 && linesAfter[0].OrderID != orderID {
		t.Error("line order_id changed after failed split")
	}

	// No audit entry for split
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='order' AND entity_id=$1 AND action='split'`, orderID).Scan(&auditCount)
	if auditCount != 0 {
		t.Error("no audit log should exist for failed split")
	}
}

func TestSplitOrderRejectsLineBelongingToAnotherOrder(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	repo := orders.NewRepository(pool)
	svc := orders.NewService(repo, pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Order User", "split-other@test.com")

	var brandID, seriesID, modelID int
	pool.QueryRow(ctx, `INSERT INTO brands (name) VALUES ('OtherBrand') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&brandID)
	pool.QueryRow(ctx, `INSERT INTO series (brand_id, name) VALUES ($1,'OtherSeries') ON CONFLICT(brand_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, brandID).Scan(&seriesID)
	pool.QueryRow(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, publication_status, stock_quantity) VALUES ($1,$2,'OTHER01','Other Model',2024,'published',1) RETURNING id`, brandID, seriesID).Scan(&modelID)

	var accountID int
	pool.QueryRow(ctx, `INSERT INTO customer_accounts (name, email, phone) VALUES ('Other Cust','other@test.com','555-9999') RETURNING id`).Scan(&accountID)
	var cartID int
	pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, status) VALUES ($1, 'active') RETURNING id`, accountID).Scan(&cartID)

	// Create two orders
	orderID1, _, _ := svc.CreateOrder(ctx, orders.CreateOrderParams{
		CustomerAccountID: accountID, SourceCartID: cartID, Location: "wh", CreatedBy: userID,
		Lines: []orders.CreateOrderLineParams{{VehicleModelID: modelID, QuantityRequested: 10}},
	})

	// Reset stock for second order
	pool.Exec(ctx, `UPDATE vehicle_models SET stock_quantity=1 WHERE id=$1`, modelID)
	var cartID2 int
	pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, status) VALUES ($1, 'active') RETURNING id`, accountID).Scan(&cartID2)
	orderID2, _, _ := svc.CreateOrder(ctx, orders.CreateOrderParams{
		CustomerAccountID: accountID, SourceCartID: cartID2, Location: "wh", CreatedBy: userID,
		Lines: []orders.CreateOrderLineParams{{VehicleModelID: modelID, QuantityRequested: 10}},
	})

	// Get a line from order2
	lines2, _ := svc.ListOrderLines(ctx, orderID2)
	if len(lines2) == 0 {
		t.Fatal("expected lines for order2")
	}

	// Try to split order1 using order2's line — should fail
	_, err := svc.SplitOrder(ctx, orderID1, userID, []int{lines2[0].ID})
	if err == nil {
		t.Fatalf("expected error splitting with another order's line (order1=%d, order2=%d)", orderID1, orderID2)
	}
}
