package orders

import (
	"context"
	"errors"
	"fmt"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSplitLineNotEligible = errors.New("one or more lines are not eligible for split")

type Service struct {
	repo     *Repository
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(repo *Repository, pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, pool: pool, auditSvc: auditSvc}
}

func (s *Service) ListOrders(ctx context.Context, p ListParams) ([]Order, int, error) {
	return s.repo.ListOrders(ctx, p)
}

func (s *Service) GetOrder(ctx context.Context, id int) (*Order, error) {
	return s.repo.GetOrder(ctx, id)
}

func (s *Service) CreateOrder(ctx context.Context, p CreateOrderParams) (int, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)

	id, orderNumber, err := s.repo.CreateOrder(ctx, tx, p)
	if err != nil {
		return 0, "", fmt.Errorf("create order: %w", err)
	}

	hasBackorder := false
	for _, line := range p.Lines {
		var currentStock int
		err := tx.QueryRow(ctx, `SELECT stock_quantity FROM vehicle_models WHERE id = $1 FOR UPDATE`, line.VehicleModelID).Scan(&currentStock)
		if err != nil {
			currentStock = 0
		}

		allocated := line.QuantityRequested
		backordered := 0
		lineStatus := "allocated"

		if currentStock < line.QuantityRequested {
			allocated = currentStock
			if allocated < 0 {
				allocated = 0
			}
			backordered = line.QuantityRequested - allocated
			if allocated == 0 {
				lineStatus = "backordered"
			} else {
				lineStatus = "partial"
			}
			hasBackorder = true
		}

		if allocated > 0 {
			if _, err := tx.Exec(ctx, `UPDATE vehicle_models SET stock_quantity = stock_quantity - $1, updated_at = NOW() WHERE id = $2`, allocated, line.VehicleModelID); err != nil {
				return 0, "", fmt.Errorf("deduct stock: %w", err)
			}
		}

		line.StockSnapshot = currentStock
		if err := s.repo.CreateOrderLineWithAllocation(ctx, tx, id, line, allocated, backordered, lineStatus); err != nil {
			return 0, "", fmt.Errorf("create order line: %w", err)
		}
	}

	// Mandatory: state history and audit writes must succeed or transaction rolls back
	if err := s.repo.CreateStateHistory(ctx, tx, id, "", StatusCreated, &p.CreatedBy, "user", "Order created from cart"); err != nil {
		return 0, "", fmt.Errorf("write state history: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "order", EntityID: id, Action: "created", ActorUserID: &p.CreatedBy,
		After: map[string]interface{}{"order_number": orderNumber, "source_cart_id": p.SourceCartID, "has_backorder": hasBackorder, "line_count": len(p.Lines)},
	}); err != nil {
		return 0, "", fmt.Errorf("write audit: %w", err)
	}

	if hasBackorder {
		if err := s.repo.SetOrderStatus(ctx, tx, id, StatusPartiallyBackordered); err != nil {
			return 0, "", fmt.Errorf("set backorder status: %w", err)
		}
		if err := s.repo.CreateStateHistory(ctx, tx, id, StatusCreated, StatusPartiallyBackordered, &p.CreatedBy, "system", "Order contains backordered lines due to insufficient stock"); err != nil {
			return 0, "", fmt.Errorf("write backorder history: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, "", err
	}
	return id, orderNumber, nil
}

func (s *Service) TransitionOrder(ctx context.Context, orderID int, toStatus string, actorID *int, actorType, reason string) error {
	if toStatus == StatusPaymentRecorded {
		return fmt.Errorf("%w: use the dedicated payment-recorded endpoint", ErrInvalidTransition)
	}

	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if err := ValidateTransition(order.Status, toStatus); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetOrderStatus(ctx, tx, orderID, toStatus); err != nil {
		return err
	}
	if err := s.repo.CreateStateHistory(ctx, tx, orderID, order.Status, toStatus, actorID, actorType, reason); err != nil {
		return fmt.Errorf("write state history: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "order", EntityID: orderID, Action: "transitioned", ActorUserID: actorID,
		Before: map[string]string{"status": order.Status}, After: map[string]string{"status": toStatus},
		Metadata: map[string]string{"reason": reason},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) RecordPayment(ctx context.Context, orderID int, actorID int) error {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if err := ValidateTransition(order.Status, StatusPaymentRecorded); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetPaymentRecorded(ctx, tx, orderID); err != nil {
		return err
	}
	if err := s.repo.CreatePaymentRecord(ctx, tx, orderID, actorID); err != nil {
		return fmt.Errorf("create payment record: %w", err)
	}
	if err := s.repo.CreateStateHistory(ctx, tx, orderID, order.Status, StatusPaymentRecorded, &actorID, "user", "Payment recorded"); err != nil {
		return fmt.Errorf("write state history: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "order", EntityID: orderID, Action: "payment_recorded", ActorUserID: &actorID,
		Before: map[string]string{"status": order.Status}, After: map[string]string{"status": StatusPaymentRecorded},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) AddNote(ctx context.Context, orderID int, noteType, content string, authorID int) (int, error) {
	return s.repo.CreateNote(ctx, orderID, noteType, content, authorID)
}

func (s *Service) ListOrderLines(ctx context.Context, orderID int) ([]OrderLine, error) {
	return s.repo.ListOrderLines(ctx, orderID)
}

func (s *Service) ListNotes(ctx context.Context, orderID int) ([]OrderNote, error) {
	return s.repo.ListNotes(ctx, orderID)
}

func (s *Service) ListHistory(ctx context.Context, orderID int) ([]OrderStateHistory, error) {
	return s.repo.ListHistory(ctx, orderID)
}

func (s *Service) GetAllowedTransitions(status string) []string {
	return GetAllowedTransitions(status)
}

// ProcessCutoffs transitions orders past cutoff time
func (s *Service) ProcessCutoffs(ctx context.Context, cutoffMinutes int) (int, error) {
	orders, err := s.repo.FindCreatedOrdersForCutoff(ctx, cutoffMinutes)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, order := range orders {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			continue
		}

		if err := s.repo.SetCutoff(ctx, tx, order.ID); err != nil {
			tx.Rollback(ctx)
			continue
		}
		if err := s.repo.CreateStateHistory(ctx, tx, order.ID, StatusCreated, StatusCutoff, nil, "system", "Automatic cutoff - payment not recorded within 30 minutes"); err != nil {
			tx.Rollback(ctx)
			continue
		}
		if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
			EntityType: "order", EntityID: order.ID, Action: "auto_cutoff",
			Before: map[string]string{"status": StatusCreated}, After: map[string]string{"status": StatusCutoff},
			Metadata: map[string]string{"reason": "Payment not recorded within cutoff window"},
		}); err != nil {
			tx.Rollback(ctx)
			continue
		}

		if err := tx.Commit(ctx); err == nil {
			count++
		}
	}
	return count, nil
}

// SplitOrder moves backordered lines to a new child order.
// Validates that all requested line IDs belong to the parent order
// and are eligible for split (quantity_backordered > 0).
func (s *Service) SplitOrder(ctx context.Context, orderID int, actorID int, backorderLineIDs []int) (int, error) {
	if len(backorderLineIDs) == 0 {
		return 0, errors.New("no lines selected for split")
	}

	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return 0, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// Validate all lines belong to this order and are backordered
	for _, lineID := range backorderLineIDs {
		var count int
		err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM order_lines WHERE id = $1 AND order_id = $2 AND quantity_backordered > 0`,
			lineID, orderID).Scan(&count)
		if err != nil || count == 0 {
			return 0, fmt.Errorf("%w: line %d does not belong to order %d or is not backordered", ErrSplitLineNotEligible, lineID, orderID)
		}
	}

	// Create child order
	childID, childNumber, err := s.repo.CreateOrder(ctx, tx, CreateOrderParams{
		CustomerAccountID: *order.CustomerAccountID,
		SourceCartID:      *order.SourceCartID,
		Location:          order.Location,
		CreatedBy:         actorID,
	})
	if err != nil {
		return 0, err
	}

	// Set parent link on child
	if _, err := tx.Exec(ctx, `UPDATE orders SET split_parent_order_id = $1 WHERE id = $2`, orderID, childID); err != nil {
		return 0, fmt.Errorf("set parent link: %w", err)
	}

	// Move validated lines — scoped by order_id for defense in depth
	for _, lineID := range backorderLineIDs {
		tag, err := tx.Exec(ctx, `UPDATE order_lines SET order_id = $1 WHERE id = $2 AND order_id = $3`, childID, lineID, orderID)
		if err != nil {
			return 0, fmt.Errorf("move line %d: %w", lineID, err)
		}
		if tag.RowsAffected() == 0 {
			return 0, fmt.Errorf("%w: line %d could not be moved", ErrSplitLineNotEligible, lineID)
		}
	}

	// Update parent status with mandatory audit
	if err := s.repo.SetOrderStatus(ctx, tx, orderID, StatusSplit); err != nil {
		return 0, err
	}
	if err := s.repo.CreateStateHistory(ctx, tx, orderID, order.Status, StatusSplit, &actorID, "user", fmt.Sprintf("Order split, child order: %s", childNumber)); err != nil {
		return 0, fmt.Errorf("write state history: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "order", EntityID: orderID, Action: "split", ActorUserID: &actorID,
		Before:   map[string]string{"status": order.Status},
		After:    map[string]string{"status": StatusSplit},
		Metadata: map[string]interface{}{"child_order_id": childID, "child_order_number": childNumber, "lines_moved": len(backorderLineIDs)},
	}); err != nil {
		return 0, fmt.Errorf("write audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return childID, nil
}

func (s *Service) CountActive(ctx context.Context) (int, error) {
	return s.repo.CountActive(ctx)
}

func (s *Service) Repo() *Repository {
	return s.repo
}
