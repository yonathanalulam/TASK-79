package cart

import (
	"errors"
	"time"
)

var ErrItemNotInCart = errors.New("item does not belong to this cart")

type CustomerAccount struct {
	ID                int    `json:"id"`
	AccountCode       string `json:"account_code"`
	AccountName       string `json:"account_name"`
	ContactPhoneMask  string `json:"contact_phone_masked"`
	Location          string `json:"location"`
}

type Cart struct {
	ID                int       `json:"id"`
	CustomerAccountID *int      `json:"customer_account_id"`
	CustomerName      string    `json:"customer_name"`
	Status            string    `json:"status"`
	ItemCount         int       `json:"item_count"`
	CreatedBy         *int      `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CartItem struct {
	ID                int       `json:"id"`
	CartID            int       `json:"cart_id"`
	VehicleModelID    int       `json:"vehicle_model_id"`
	VehicleName       string    `json:"vehicle_name"`
	Quantity          int       `json:"quantity"`
	UnitPriceSnapshot *float64  `json:"unit_price_snapshot"`
	ValidityStatus    string    `json:"validity_status"`
	ValidationMessage string    `json:"validation_message"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AddItemParams struct {
	VehicleModelID int
	Quantity       int
	UnitPrice      float64
}

type MergeResult struct {
	ItemsMerged int
	ItemsAdded  int
}
