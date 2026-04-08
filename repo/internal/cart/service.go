package cart

import (
	"context"
	"encoding/json"
	"errors"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCartNotOpen       = errors.New("cart is not open")
	ErrAllItemsInvalid   = errors.New("all cart items are invalid")
	ErrNoCustomer        = errors.New("cart has no customer account assigned")
	ErrMergeSameCart      = errors.New("cannot merge cart with itself")
	ErrMergeDiffCustomer = errors.New("cannot merge carts from different customers")
)

type Service struct {
	repo     *Repository
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(repo *Repository, pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, pool: pool, auditSvc: auditSvc}
}

func (s *Service) ListCarts(ctx context.Context, userID int, isAdmin bool) ([]Cart, error) {
	return s.repo.ListCarts(ctx, userID, isAdmin)
}

func (s *Service) GetCart(ctx context.Context, id int) (*Cart, error) {
	return s.repo.GetCart(ctx, id)
}

func (s *Service) CreateCart(ctx context.Context, customerAccountID *int, createdBy int) (int, error) {
	return s.repo.CreateCart(ctx, customerAccountID, createdBy)
}

func (s *Service) ListCartItems(ctx context.Context, cartID int) ([]CartItem, error) {
	return s.repo.ListCartItems(ctx, cartID)
}

func (s *Service) AddItem(ctx context.Context, cartID int, p AddItemParams) (int, error) {
	return s.repo.AddItem(ctx, cartID, p)
}

func (s *Service) UpdateItem(ctx context.Context, cartID, itemID int, quantity int) error {
	return s.repo.UpdateItem(ctx, cartID, itemID, quantity)
}

func (s *Service) DeleteItem(ctx context.Context, cartID, itemID int) error {
	return s.repo.DeleteItem(ctx, cartID, itemID)
}

func (s *Service) ListCustomerAccounts(ctx context.Context) ([]CustomerAccount, error) {
	return s.repo.ListCustomerAccounts(ctx)
}

// ValidateCart checks each item against current catalog/inventory state
func (s *Service) ValidateCart(ctx context.Context, cartID int) ([]CartItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	items, err := s.repo.ListCartItems(ctx, cartID)
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		var status, pubStatus string
		var stock int
		var discAt interface{}
		err := s.pool.QueryRow(ctx, `SELECT publication_status, stock_quantity, discontinued_at FROM vehicle_models WHERE id = $1`, item.VehicleModelID).Scan(&pubStatus, &stock, &discAt)
		if err != nil {
			items[i].ValidityStatus = "unpublished"
			items[i].ValidationMessage = "Vehicle not found"
			s.repo.UpdateItemValidity(ctx, tx, item.ID, "unpublished", "Vehicle not found")
			continue
		}

		status = "valid"
		msg := ""
		if discAt != nil {
			status = "discontinued"
			msg = "This vehicle has been discontinued"
		} else if pubStatus != "published" {
			status = "unpublished"
			msg = "This vehicle is not currently published"
		} else if stock <= 0 {
			status = "out_of_stock"
			msg = "This vehicle is out of stock"
		}

		items[i].ValidityStatus = status
		items[i].ValidationMessage = msg
		s.repo.UpdateItemValidity(ctx, tx, item.ID, status, msg)
	}

	return items, tx.Commit(ctx)
}

// MergeCart merges source cart items into target cart
func (s *Service) MergeCart(ctx context.Context, targetCartID, sourceCartID, actorID int) (*MergeResult, error) {
	if targetCartID == sourceCartID {
		return nil, ErrMergeSameCart
	}

	target, err := s.repo.GetCart(ctx, targetCartID)
	if err != nil {
		return nil, err
	}
	source, err := s.repo.GetCart(ctx, sourceCartID)
	if err != nil {
		return nil, err
	}

	if target.Status != "open" {
		return nil, ErrCartNotOpen
	}
	if target.CustomerAccountID == nil || source.CustomerAccountID == nil || *target.CustomerAccountID != *source.CustomerAccountID {
		return nil, ErrMergeDiffCustomer
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sourceItems, err := s.repo.ListCartItems(ctx, sourceCartID)
	if err != nil {
		return nil, err
	}
	targetItems, err := s.repo.ListCartItems(ctx, targetCartID)
	if err != nil {
		return nil, err
	}

	targetMap := make(map[int]*CartItem)
	for i := range targetItems {
		targetMap[targetItems[i].VehicleModelID] = &targetItems[i]
	}

	result := &MergeResult{}
	for _, si := range sourceItems {
		if existing, ok := targetMap[si.VehicleModelID]; ok {
			newQty := existing.Quantity + si.Quantity
			_, err := tx.Exec(ctx, `UPDATE cart_items SET quantity=$1, updated_at=NOW() WHERE id=$2`, newQty, existing.ID)
			if err != nil {
				return nil, err
			}
			result.ItemsMerged++
		} else {
			_, err := tx.Exec(ctx, `INSERT INTO cart_items (cart_id, vehicle_model_id, quantity, unit_price_snapshot) VALUES ($1,$2,$3,$4)`,
				targetCartID, si.VehicleModelID, si.Quantity, si.UnitPriceSnapshot)
			if err != nil {
				return nil, err
			}
			result.ItemsAdded++
		}
	}

	// Mark source as abandoned
	s.repo.SetCartStatus(ctx, tx, sourceCartID, "abandoned")

	// Cart event
	details, _ := json.Marshal(map[string]interface{}{
		"source_cart_id": sourceCartID,
		"items_merged":   result.ItemsMerged,
		"items_added":    result.ItemsAdded,
	})
	s.repo.CreateCartEvent(ctx, tx, targetCartID, "merge", details, actorID)

	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "cart",
		EntityID:    targetCartID,
		Action:      "merged",
		ActorUserID: &actorID,
		Metadata: map[string]interface{}{
			"source_cart_id": sourceCartID,
			"items_merged":   result.ItemsMerged,
			"items_added":    result.ItemsAdded,
		},
	})

	return result, tx.Commit(ctx)
}

// Checkout converts cart to order (returns cart items for order creation)
func (s *Service) Checkout(ctx context.Context, cartID, actorID int) ([]CartItem, *Cart, error) {
	cart, err := s.repo.GetCart(ctx, cartID)
	if err != nil {
		return nil, nil, err
	}
	if cart.Status != "open" {
		return nil, nil, ErrCartNotOpen
	}
	if cart.CustomerAccountID == nil {
		return nil, nil, ErrNoCustomer
	}

	// Validate first
	items, err := s.ValidateCart(ctx, cartID)
	if err != nil {
		return nil, nil, err
	}

	// Check at least one valid
	hasValid := false
	for _, item := range items {
		if item.ValidityStatus == "valid" {
			hasValid = true
			break
		}
	}
	if !hasValid {
		return nil, nil, ErrAllItemsInvalid
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	s.repo.SetCartStatus(ctx, tx, cartID, "converted")

	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "cart",
		EntityID:    cartID,
		Action:      "checkout",
		ActorUserID: &actorID,
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return items, cart, nil
}

func (s *Service) ListOpenCartsByCustomer(ctx context.Context, customerAccountID int, excludeCartID int) ([]Cart, error) {
	return s.repo.ListOpenCartsByCustomer(ctx, customerAccountID, excludeCartID)
}

func (s *Service) Repo() *Repository {
	return s.repo
}

func (s *Service) CountOpen(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM carts WHERE status = 'open'`).Scan(&count)
	return count, err
}

func (s *Service) SetCartStatus(ctx context.Context, cartID int, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE carts SET status=$1, updated_at=NOW() WHERE id=$2`, status, cartID)
	return err
}
