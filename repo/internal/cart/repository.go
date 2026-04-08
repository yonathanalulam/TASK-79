package cart

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListCustomerAccounts(ctx context.Context) ([]CustomerAccount, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, account_code, account_name, COALESCE(contact_phone_masked,''), COALESCE(location,'') FROM customer_accounts ORDER BY account_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []CustomerAccount
	for rows.Next() {
		var a CustomerAccount
		if err := rows.Scan(&a.ID, &a.AccountCode, &a.AccountName, &a.ContactPhoneMask, &a.Location); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *Repository) ListCarts(ctx context.Context, userID int, isAdmin bool) ([]Cart, error) {
	query := `SELECT c.id, c.customer_account_id, COALESCE(ca.account_name,'Unassigned'), c.status,
		(SELECT COUNT(*) FROM cart_items WHERE cart_id = c.id), c.created_by, c.created_at, c.updated_at
		FROM carts c LEFT JOIN customer_accounts ca ON ca.id = c.customer_account_id`
	var args []interface{}
	if !isAdmin {
		query += ` WHERE c.created_by = $1`
		args = append(args, userID)
	}
	query += ` ORDER BY c.updated_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carts []Cart
	for rows.Next() {
		var c Cart
		if err := rows.Scan(&c.ID, &c.CustomerAccountID, &c.CustomerName, &c.Status, &c.ItemCount, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		carts = append(carts, c)
	}
	return carts, nil
}

func (r *Repository) GetCart(ctx context.Context, id int) (*Cart, error) {
	var c Cart
	err := r.pool.QueryRow(ctx, `SELECT c.id, c.customer_account_id, COALESCE(ca.account_name,'Unassigned'), c.status,
		(SELECT COUNT(*) FROM cart_items WHERE cart_id = c.id), c.created_by, c.created_at, c.updated_at
		FROM carts c LEFT JOIN customer_accounts ca ON ca.id = c.customer_account_id
		WHERE c.id = $1`, id).Scan(&c.ID, &c.CustomerAccountID, &c.CustomerName, &c.Status, &c.ItemCount, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) CreateCart(ctx context.Context, customerAccountID *int, createdBy int) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `INSERT INTO carts (customer_account_id, created_by, updated_by) VALUES ($1, $2, $2) RETURNING id`,
		customerAccountID, createdBy).Scan(&id)
	return id, err
}

func (r *Repository) ListCartItems(ctx context.Context, cartID int) ([]CartItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT ci.id, ci.cart_id, ci.vehicle_model_id, vm.model_name, ci.quantity, ci.unit_price_snapshot, ci.validity_status, COALESCE(ci.validation_message,''), ci.created_at, ci.updated_at
		FROM cart_items ci JOIN vehicle_models vm ON vm.id = ci.vehicle_model_id
		WHERE ci.cart_id = $1 ORDER BY ci.created_at`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.CartID, &item.VehicleModelID, &item.VehicleName, &item.Quantity, &item.UnitPriceSnapshot, &item.ValidityStatus, &item.ValidationMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) AddItem(ctx context.Context, cartID int, p AddItemParams) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `INSERT INTO cart_items (cart_id, vehicle_model_id, quantity, unit_price_snapshot)
		VALUES ($1, $2, $3, $4) RETURNING id`, cartID, p.VehicleModelID, p.Quantity, p.UnitPrice).Scan(&id)
	return id, err
}

// UpdateItem updates an item scoped by both cart_id and item_id.
// Returns error if no matching row (item doesn't belong to cart).
func (r *Repository) UpdateItem(ctx context.Context, cartID, itemID int, quantity int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE cart_items SET quantity=$1, updated_at=NOW() WHERE id=$2 AND cart_id=$3`, quantity, itemID, cartID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotInCart
	}
	return nil
}

// DeleteItem deletes an item scoped by both cart_id and item_id.
// Returns error if no matching row (item doesn't belong to cart).
func (r *Repository) DeleteItem(ctx context.Context, cartID, itemID int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`, itemID, cartID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotInCart
	}
	return nil
}

func (r *Repository) UpdateItemValidity(ctx context.Context, tx pgx.Tx, itemID int, status, message string) error {
	_, err := tx.Exec(ctx, `UPDATE cart_items SET validity_status=$1, validation_message=$2, updated_at=NOW() WHERE id=$3`, status, message, itemID)
	return err
}

func (r *Repository) SetCartStatus(ctx context.Context, tx pgx.Tx, cartID int, status string) error {
	_, err := tx.Exec(ctx, `UPDATE carts SET status=$1, updated_at=NOW() WHERE id=$2`, status, cartID)
	return err
}

func (r *Repository) ListOpenCartsByCustomer(ctx context.Context, customerAccountID int, excludeCartID int) ([]Cart, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.id, c.customer_account_id, COALESCE(ca.account_name,''), c.status,
		(SELECT COUNT(*) FROM cart_items WHERE cart_id = c.id), c.created_by, c.created_at, c.updated_at
		FROM carts c LEFT JOIN customer_accounts ca ON ca.id = c.customer_account_id
		WHERE c.customer_account_id = $1 AND c.status = 'open' AND c.id != $2`, customerAccountID, excludeCartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carts []Cart
	for rows.Next() {
		var c Cart
		if err := rows.Scan(&c.ID, &c.CustomerAccountID, &c.CustomerName, &c.Status, &c.ItemCount, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		carts = append(carts, c)
	}
	return carts, nil
}

func (r *Repository) CreateCartEvent(ctx context.Context, tx pgx.Tx, cartID int, eventType string, details interface{}, actorID int) error {
	_, err := tx.Exec(ctx, `INSERT INTO cart_events (cart_id, event_type, details, actor_id) VALUES ($1, $2, $3, $4)`,
		cartID, eventType, details, actorID)
	return err
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}
