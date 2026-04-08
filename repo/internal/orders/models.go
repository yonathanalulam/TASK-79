package orders

import "time"

type Order struct {
	ID                  int        `json:"id"`
	OrderNumber         string     `json:"order_number"`
	CustomerAccountID   *int       `json:"customer_account_id"`
	CustomerName        string     `json:"customer_name"`
	SourceCartID        *int       `json:"source_cart_id"`
	Status              string     `json:"status"`
	PromisedDate        string     `json:"promised_date,omitempty"`
	Location            string     `json:"location"`
	CreatedBy           *int       `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	CutoffAt            *time.Time `json:"cutoff_at,omitempty"`
	PaymentRecordedAt   *time.Time `json:"payment_recorded_at,omitempty"`
	SplitParentOrderID  *int       `json:"split_parent_order_id,omitempty"`
}

type OrderLine struct {
	ID                   int       `json:"id"`
	OrderID              int       `json:"order_id"`
	VehicleModelID       int       `json:"vehicle_model_id"`
	VehicleName          string    `json:"vehicle_name"`
	QuantityRequested    int       `json:"quantity_requested"`
	QuantityAllocated    int       `json:"quantity_allocated"`
	QuantityBackordered  int       `json:"quantity_backordered"`
	LineStatus           string    `json:"line_status"`
	StockSnapshot        *int      `json:"stock_snapshot"`
	PublicationSnapshot  string    `json:"publication_snapshot"`
	DiscontinuedSnapshot bool      `json:"discontinued_snapshot"`
	CreatedAt            time.Time `json:"created_at"`
}

type OrderNote struct {
	ID         int       `json:"id"`
	OrderID    int       `json:"order_id"`
	NoteType   string    `json:"note_type"`
	Content    string    `json:"content"`
	AuthorID   *int      `json:"author_id"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type OrderStateHistory struct {
	ID             int       `json:"id"`
	OrderID        int       `json:"order_id"`
	FromStatus     string    `json:"from_status"`
	ToStatus       string    `json:"to_status"`
	ActorID        *int      `json:"actor_id"`
	ActorName      string    `json:"actor_name"`
	ActorType      string    `json:"actor_type"`
	Reason         string    `json:"reason"`
	TransitionedAt time.Time `json:"transitioned_at"`
}

type CreateOrderParams struct {
	CustomerAccountID int
	SourceCartID      int
	PromisedDate      string
	Location          string
	CreatedBy         int
	Lines             []CreateOrderLineParams
}

type CreateOrderLineParams struct {
	VehicleModelID      int
	QuantityRequested   int
	StockSnapshot       int
	PublicationSnapshot string
	DiscontinuedSnapshot bool
}

type ListParams struct {
	Status         string
	Query          string
	Page           int
	PageSize       int
	ViewerUserID   int  // 0 = unset (should not happen for authenticated calls)
	GlobalReadScope bool // if true, returns all orders; if false, scoped to ViewerUserID
}
