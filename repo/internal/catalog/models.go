package catalog

import "time"

type Brand struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Series struct {
	ID      int    `json:"id"`
	BrandID int    `json:"brand_id"`
	Name    string `json:"name"`
}

type VehicleModel struct {
	ID                int        `json:"id"`
	BrandID           int        `json:"brand_id"`
	BrandName         string     `json:"brand_name"`
	SeriesID          int        `json:"series_id"`
	SeriesName        string     `json:"series_name"`
	ModelCode         string     `json:"model_code"`
	ModelName         string     `json:"model_name"`
	Year              int        `json:"year"`
	Description       string     `json:"description"`
	PublicationStatus string     `json:"publication_status"`
	StockQuantity     int        `json:"stock_quantity"`
	ExpiryDate        string     `json:"expiry_date,omitempty"`
	DiscontinuedAt    *time.Time `json:"discontinued_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type VehicleModelVersion struct {
	ID             int       `json:"id"`
	VehicleModelID int       `json:"vehicle_model_id"`
	VersionNumber  int       `json:"version_number"`
	ModelName      string    `json:"model_name"`
	Year           int       `json:"year"`
	Description    string    `json:"description"`
	StockQuantity  int       `json:"stock_quantity"`
	ExpiryDate     string    `json:"expiry_date"`
	Status         string    `json:"status"`
	IsCurrentDraft bool      `json:"is_current_draft"`
	IsCurrentPub   bool      `json:"is_current_pub"`
	CreatedBy      *int      `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Media struct {
	ID                int       `json:"id"`
	VehicleModelID    int       `json:"vehicle_model_id"`
	Kind              string    `json:"kind"`
	OriginalFilename  string    `json:"original_filename"`
	StoredPath        string    `json:"stored_path"`
	MimeType          string    `json:"mime_type"`
	SizeBytes         int64     `json:"size_bytes"`
	SHA256Fingerprint string    `json:"sha256_fingerprint"`
	UploadedBy        *int      `json:"uploaded_by"`
	CreatedAt         time.Time `json:"created_at"`
}

type ListParams struct {
	BrandID  int
	SeriesID int
	Status   string
	Query    string
	Page     int
	PageSize int
}

type CreateModelParams struct {
	BrandID       int
	SeriesID      int
	ModelCode     string
	ModelName     string
	Year          int
	Description   string
	StockQuantity int
	ExpiryDate    string
}

type UpdateDraftParams struct {
	ModelName     string
	Year          int
	Description   string
	StockQuantity int
	ExpiryDate    string
}
