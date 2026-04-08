package orders

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListOrders(ctx context.Context, p ListParams) ([]Order, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 1

	// Scope filtering: non-global users only see orders they created
	if !p.GlobalReadScope && p.ViewerUserID > 0 {
		where += " AND o.created_by = $" + strconv.Itoa(n)
		args = append(args, p.ViewerUserID)
		n++
	}

	if p.Status != "" {
		where += " AND o.status = $" + strconv.Itoa(n)
		args = append(args, p.Status)
		n++
	}
	if p.Query != "" {
		where += " AND o.order_number ILIKE $" + strconv.Itoa(n)
		args = append(args, "%"+p.Query+"%")
		n++
	}

	var total int
	countQ := "SELECT COUNT(*) FROM orders o " + where
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.PageSize

	query := fmt.Sprintf(`SELECT o.id, o.order_number, o.customer_account_id, COALESCE(ca.account_name,''),
		o.source_cart_id, o.status, COALESCE(o.promised_date::text,''), COALESCE(o.location,''),
		o.created_by, o.created_at, o.updated_at, o.cutoff_at, o.payment_recorded_at, o.split_parent_order_id
		FROM orders o LEFT JOIN customer_accounts ca ON ca.id = o.customer_account_id
		%s ORDER BY o.created_at DESC LIMIT $%d OFFSET $%d`, where, n, n+1)
	args = append(args, p.PageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.CustomerAccountID, &o.CustomerName,
			&o.SourceCartID, &o.Status, &o.PromisedDate, &o.Location,
			&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt, &o.CutoffAt, &o.PaymentRecordedAt, &o.SplitParentOrderID); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, nil
}

func (r *Repository) GetOrder(ctx context.Context, id int) (*Order, error) {
	var o Order
	err := r.pool.QueryRow(ctx, `SELECT o.id, o.order_number, o.customer_account_id, COALESCE(ca.account_name,''),
		o.source_cart_id, o.status, COALESCE(o.promised_date::text,''), COALESCE(o.location,''),
		o.created_by, o.created_at, o.updated_at, o.cutoff_at, o.payment_recorded_at, o.split_parent_order_id
		FROM orders o LEFT JOIN customer_accounts ca ON ca.id = o.customer_account_id
		WHERE o.id = $1`, id,
	).Scan(&o.ID, &o.OrderNumber, &o.CustomerAccountID, &o.CustomerName,
		&o.SourceCartID, &o.Status, &o.PromisedDate, &o.Location,
		&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt, &o.CutoffAt, &o.PaymentRecordedAt, &o.SplitParentOrderID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *Repository) CreateOrder(ctx context.Context, tx pgx.Tx, p CreateOrderParams) (int, string, error) {
	var id int
	var orderNumber string
	promDate := interface{}(nil)
	if p.PromisedDate != "" {
		promDate = p.PromisedDate
	}
	err := tx.QueryRow(ctx, `INSERT INTO orders (order_number, customer_account_id, source_cart_id, promised_date, location, created_by)
		VALUES ('ORD-' || LPAD(nextval('orders_id_seq')::text, 6, '0'), $1, $2, $3, $4, $5) RETURNING id, order_number`,
		p.CustomerAccountID, p.SourceCartID, promDate, p.Location, p.CreatedBy,
	).Scan(&id, &orderNumber)
	return id, orderNumber, err
}

func (r *Repository) CreateOrderLine(ctx context.Context, tx pgx.Tx, orderID int, p CreateOrderLineParams) error {
	_, err := tx.Exec(ctx, `INSERT INTO order_lines (order_id, vehicle_model_id, quantity_requested, stock_snapshot, publication_snapshot, discontinued_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		orderID, p.VehicleModelID, p.QuantityRequested, p.StockSnapshot, p.PublicationSnapshot, p.DiscontinuedSnapshot)
	return err
}

func (r *Repository) CreateOrderLineWithAllocation(ctx context.Context, tx pgx.Tx, orderID int, p CreateOrderLineParams, allocated, backordered int, lineStatus string) error {
	_, err := tx.Exec(ctx, `INSERT INTO order_lines (order_id, vehicle_model_id, quantity_requested, quantity_allocated, quantity_backordered, line_status, stock_snapshot, publication_snapshot, discontinued_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		orderID, p.VehicleModelID, p.QuantityRequested, allocated, backordered, lineStatus, p.StockSnapshot, p.PublicationSnapshot, p.DiscontinuedSnapshot)
	return err
}

func (r *Repository) ListOrderLines(ctx context.Context, orderID int) ([]OrderLine, error) {
	rows, err := r.pool.Query(ctx, `SELECT ol.id, ol.order_id, ol.vehicle_model_id, vm.model_name,
		ol.quantity_requested, ol.quantity_allocated, ol.quantity_backordered, ol.line_status,
		ol.stock_snapshot, COALESCE(ol.publication_snapshot,''), ol.discontinued_snapshot, ol.created_at
		FROM order_lines ol JOIN vehicle_models vm ON vm.id = ol.vehicle_model_id
		WHERE ol.order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []OrderLine
	for rows.Next() {
		var l OrderLine
		if err := rows.Scan(&l.ID, &l.OrderID, &l.VehicleModelID, &l.VehicleName,
			&l.QuantityRequested, &l.QuantityAllocated, &l.QuantityBackordered, &l.LineStatus,
			&l.StockSnapshot, &l.PublicationSnapshot, &l.DiscontinuedSnapshot, &l.CreatedAt); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, nil
}

func (r *Repository) SetOrderStatus(ctx context.Context, tx pgx.Tx, orderID int, status string) error {
	_, err := tx.Exec(ctx, `UPDATE orders SET status=$1, updated_at=NOW() WHERE id=$2`, status, orderID)
	return err
}

func (r *Repository) SetPaymentRecorded(ctx context.Context, tx pgx.Tx, orderID int) error {
	_, err := tx.Exec(ctx, `UPDATE orders SET status='payment_recorded', payment_recorded_at=NOW(), updated_at=NOW() WHERE id=$1`, orderID)
	return err
}

func (r *Repository) SetCutoff(ctx context.Context, tx pgx.Tx, orderID int) error {
	_, err := tx.Exec(ctx, `UPDATE orders SET status='cutoff', cutoff_at=NOW(), updated_at=NOW() WHERE id=$1`, orderID)
	return err
}

func (r *Repository) CreateStateHistory(ctx context.Context, tx pgx.Tx, orderID int, from, to string, actorID *int, actorType, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO order_state_history (order_id, from_status, to_status, actor_id, actor_type, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`, orderID, from, to, actorID, actorType, reason)
	return err
}

func (r *Repository) CreateNote(ctx context.Context, orderID int, noteType, content string, authorID int) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `INSERT INTO order_notes (order_id, note_type, content, author_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		orderID, noteType, content, authorID).Scan(&id)
	return id, err
}

func (r *Repository) ListNotes(ctx context.Context, orderID int) ([]OrderNote, error) {
	rows, err := r.pool.Query(ctx, `SELECT n.id, n.order_id, n.note_type, n.content, n.author_id, COALESCE(u.full_name,'System'), n.created_at
		FROM order_notes n LEFT JOIN users u ON u.id = n.author_id
		WHERE n.order_id = $1 ORDER BY n.created_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []OrderNote
	for rows.Next() {
		var n OrderNote
		if err := rows.Scan(&n.ID, &n.OrderID, &n.NoteType, &n.Content, &n.AuthorID, &n.AuthorName, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (r *Repository) ListHistory(ctx context.Context, orderID int) ([]OrderStateHistory, error) {
	rows, err := r.pool.Query(ctx, `SELECT h.id, h.order_id, COALESCE(h.from_status,''), h.to_status, h.actor_id,
		COALESCE(u.full_name, 'System'), h.actor_type, COALESCE(h.reason,''), h.transitioned_at
		FROM order_state_history h LEFT JOIN users u ON u.id = h.actor_id
		WHERE h.order_id = $1 ORDER BY h.transitioned_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []OrderStateHistory
	for rows.Next() {
		var h OrderStateHistory
		if err := rows.Scan(&h.ID, &h.OrderID, &h.FromStatus, &h.ToStatus, &h.ActorID, &h.ActorName, &h.ActorType, &h.Reason, &h.TransitionedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *Repository) FindCreatedOrdersForCutoff(ctx context.Context, cutoffMinutes int) ([]Order, error) {
	rows, err := r.pool.Query(ctx, `SELECT o.id, o.order_number, o.status, o.created_at
		FROM orders o
		WHERE o.status = 'created' AND o.created_at < NOW() - make_interval(mins => $1)`, cutoffMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (r *Repository) CreatePaymentRecord(ctx context.Context, tx pgx.Tx, orderID int, recordedBy int) error {
	_, err := tx.Exec(ctx, `INSERT INTO payment_records (order_id, recorded_by) VALUES ($1, $2)`, orderID, recordedBy)
	return err
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *Repository) CountActive(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status NOT IN ('completed', 'cancelled')`).Scan(&count)
	return count, err
}
