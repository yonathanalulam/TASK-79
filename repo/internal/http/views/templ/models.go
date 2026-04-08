package templ

// PageData holds common data for all pages.
type PageData struct {
	Title       string
	ActiveNav   string
	UserName    string
	RoleName    string
	CSRFToken   string
	Permissions map[string]bool
	UnreadCount int
	FlashType   string
	FlashMsg    string
}

type DashboardSummary struct {
	OpenCarts           int
	ActiveOrders        int
	UnreadNotifications int
	ActiveAlerts        int
}

type CatalogModel struct {
	ID                int
	ModelCode         string
	ModelName         string
	BrandName         string
	SeriesName        string
	Year              int
	StockQuantity     int
	PublicationStatus string
}

type CartView struct {
	ID           int
	CustomerName string
	ItemCount    int
	Status       string
	CreatedAt    string
}

type CartItemView struct {
	ID                int
	VehicleName       string
	Quantity          int
	UnitPriceSnapshot string
	ValidityStatus    string
	ValidationMessage string
}

type OrderView struct {
	ID           int
	OrderNumber  string
	CustomerName string
	Status       string
	CreatedAt    string
	PromisedDate string
}

type OrderLineView struct {
	VehicleName       string
	QuantityRequested int
	QuantityAllocated int
	QuantityBackord   int
	LineStatus        string
}

type OrderNoteView struct {
	NoteType   string
	Content    string
	AuthorName string
	CreatedAt  string
}

type TimelineEntry struct {
	FromStatus string
	ToStatus   string
	ActorName  string
	ActorType  string
	Reason     string
	Time       string
}

type NotificationView struct {
	ID        int
	Type      string
	Title     string
	Body      string
	IsRead    bool
	CreatedAt string
}

type AlertView struct {
	ID            int
	Title         string
	Severity      string
	EntityType    string
	EntityID      int
	Status        string
	ClaimedByName string
}

type AnnouncementView struct {
	ID        int
	Title     string
	Body      string
	Priority  string
	IsRead    bool
	CreatedAt string
}

type PreferenceView struct {
	Channel   string
	EventType string
	Enabled   bool
}

type ExportQueueView struct {
	ID          int
	Channel     string
	Recipient   string
	Status      string
	Attempts    int
	MaxAttempts int
	CreatedAt   string
}

type Pagination struct {
	Page       int
	TotalPages int
}

type BrandOption struct {
	ID   int
	Name string
}

type SeriesOption struct {
	ID   int
	Name string
}
