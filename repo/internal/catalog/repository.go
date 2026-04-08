package catalog

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

func (r *Repository) ListBrands(ctx context.Context) ([]Brand, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM brands ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var brands []Brand
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		brands = append(brands, b)
	}
	return brands, nil
}

func (r *Repository) ListSeries(ctx context.Context, brandID int) ([]Series, error) {
	query := `SELECT id, brand_id, name FROM series`
	var args []interface{}
	if brandID > 0 {
		query += ` WHERE brand_id = $1`
		args = append(args, brandID)
	}
	query += ` ORDER BY name`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Series
	for rows.Next() {
		var s Series
		if err := rows.Scan(&s.ID, &s.BrandID, &s.Name); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func (r *Repository) ListModels(ctx context.Context, p ListParams) ([]VehicleModel, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 1

	if p.BrandID > 0 {
		where += " AND vm.brand_id = $" + strconv.Itoa(n)
		args = append(args, p.BrandID)
		n++
	}
	if p.SeriesID > 0 {
		where += " AND vm.series_id = $" + strconv.Itoa(n)
		args = append(args, p.SeriesID)
		n++
	}
	if p.Status != "" {
		where += " AND vm.publication_status = $" + strconv.Itoa(n)
		args = append(args, p.Status)
		n++
	}
	if p.Query != "" {
		where += " AND (vm.model_code ILIKE $" + strconv.Itoa(n) + " OR vm.model_name ILIKE $" + strconv.Itoa(n) + ")"
		args = append(args, "%"+p.Query+"%")
		n++
	}

	countQuery := "SELECT COUNT(*) FROM vehicle_models vm " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.PageSize

	query := fmt.Sprintf(`SELECT vm.id, vm.brand_id, b.name, vm.series_id, s.name,
		vm.model_code, vm.model_name, vm.year, COALESCE(vm.description,''), vm.publication_status,
		vm.stock_quantity, COALESCE(vm.expiry_date::text,''), vm.discontinued_at, vm.created_at, vm.updated_at
		FROM vehicle_models vm
		JOIN brands b ON b.id = vm.brand_id
		JOIN series s ON s.id = vm.series_id
		%s ORDER BY vm.updated_at DESC LIMIT $%d OFFSET $%d`, where, n, n+1)
	args = append(args, p.PageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var models []VehicleModel
	for rows.Next() {
		var m VehicleModel
		if err := rows.Scan(&m.ID, &m.BrandID, &m.BrandName, &m.SeriesID, &m.SeriesName,
			&m.ModelCode, &m.ModelName, &m.Year, &m.Description, &m.PublicationStatus,
			&m.StockQuantity, &m.ExpiryDate, &m.DiscontinuedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, 0, err
		}
		models = append(models, m)
	}
	return models, total, nil
}

func (r *Repository) GetModel(ctx context.Context, id int) (*VehicleModel, error) {
	var m VehicleModel
	err := r.pool.QueryRow(ctx, `SELECT vm.id, vm.brand_id, b.name, vm.series_id, s.name,
		vm.model_code, vm.model_name, vm.year, COALESCE(vm.description,''), vm.publication_status,
		vm.stock_quantity, COALESCE(vm.expiry_date::text,''), vm.discontinued_at, vm.created_at, vm.updated_at
		FROM vehicle_models vm
		JOIN brands b ON b.id = vm.brand_id
		JOIN series s ON s.id = vm.series_id
		WHERE vm.id = $1`, id,
	).Scan(&m.ID, &m.BrandID, &m.BrandName, &m.SeriesID, &m.SeriesName,
		&m.ModelCode, &m.ModelName, &m.Year, &m.Description, &m.PublicationStatus,
		&m.StockQuantity, &m.ExpiryDate, &m.DiscontinuedAt, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) CreateModel(ctx context.Context, tx pgx.Tx, p CreateModelParams) (int, error) {
	var id int
	expiryDate := interface{}(nil)
	if p.ExpiryDate != "" {
		expiryDate = p.ExpiryDate
	}
	err := tx.QueryRow(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, description, stock_quantity, expiry_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		p.BrandID, p.SeriesID, p.ModelCode, p.ModelName, p.Year, p.Description, p.StockQuantity, expiryDate,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateDraft(ctx context.Context, tx pgx.Tx, modelID int, p UpdateDraftParams) error {
	expiryDate := interface{}(nil)
	if p.ExpiryDate != "" {
		expiryDate = p.ExpiryDate
	}
	_, err := tx.Exec(ctx, `UPDATE vehicle_models SET model_name=$1, year=$2, description=$3, stock_quantity=$4, expiry_date=$5, updated_at=NOW()
		WHERE id=$6`, p.ModelName, p.Year, p.Description, p.StockQuantity, expiryDate, modelID)
	return err
}

func (r *Repository) SetPublicationStatus(ctx context.Context, tx pgx.Tx, modelID int, status string) error {
	_, err := tx.Exec(ctx, `UPDATE vehicle_models SET publication_status=$1, updated_at=NOW() WHERE id=$2`, status, modelID)
	return err
}

func (r *Repository) CreateVersion(ctx context.Context, tx pgx.Tx, v VehicleModelVersion) (int, error) {
	var id int
	err := tx.QueryRow(ctx, `INSERT INTO vehicle_model_versions (vehicle_model_id, version_number, model_name, year, description, stock_quantity, expiry_date, status, is_current_draft, is_current_pub, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		v.VehicleModelID, v.VersionNumber, v.ModelName, v.Year, v.Description, v.StockQuantity, nilIfEmpty(v.ExpiryDate), v.Status, v.IsCurrentDraft, v.IsCurrentPub, v.CreatedBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) GetNextVersionNumber(ctx context.Context, modelID int) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(version_number), 0) + 1 FROM vehicle_model_versions WHERE vehicle_model_id = $1`, modelID).Scan(&n)
	return n, err
}

func (r *Repository) ListVersions(ctx context.Context, modelID int) ([]VehicleModelVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, vehicle_model_id, version_number, model_name, year, COALESCE(description,''), stock_quantity, COALESCE(expiry_date::text,''), status, is_current_draft, is_current_pub, created_by, created_at
		FROM vehicle_model_versions WHERE vehicle_model_id = $1 ORDER BY version_number DESC`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []VehicleModelVersion
	for rows.Next() {
		var v VehicleModelVersion
		if err := rows.Scan(&v.ID, &v.VehicleModelID, &v.VersionNumber, &v.ModelName, &v.Year, &v.Description, &v.StockQuantity, &v.ExpiryDate, &v.Status, &v.IsCurrentDraft, &v.IsCurrentPub, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (r *Repository) ListMedia(ctx context.Context, modelID int) ([]Media, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, vehicle_model_id, kind, original_filename, stored_path, mime_type, size_bytes, sha256_fingerprint, uploaded_by, created_at
		FROM vehicle_media WHERE vehicle_model_id = $1 ORDER BY created_at DESC`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var media []Media
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.ID, &m.VehicleModelID, &m.Kind, &m.OriginalFilename, &m.StoredPath, &m.MimeType, &m.SizeBytes, &m.SHA256Fingerprint, &m.UploadedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		media = append(media, m)
	}
	return media, nil
}

func (r *Repository) CreateMedia(ctx context.Context, m Media) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `INSERT INTO vehicle_media (vehicle_model_id, kind, original_filename, stored_path, mime_type, size_bytes, sha256_fingerprint, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		m.VehicleModelID, m.Kind, m.OriginalFilename, m.StoredPath, m.MimeType, m.SizeBytes, m.SHA256Fingerprint, m.UploadedBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) GetModelByCode(ctx context.Context, code string) (*VehicleModel, error) {
	return r.getModelByField(ctx, "model_code", code)
}

func (r *Repository) getModelByField(ctx context.Context, field, value string) (*VehicleModel, error) {
	var m VehicleModel
	query := fmt.Sprintf(`SELECT vm.id, vm.brand_id, b.name, vm.series_id, s.name,
		vm.model_code, vm.model_name, vm.year, COALESCE(vm.description,''), vm.publication_status,
		vm.stock_quantity, COALESCE(vm.expiry_date::text,''), vm.discontinued_at, vm.created_at, vm.updated_at
		FROM vehicle_models vm
		JOIN brands b ON b.id = vm.brand_id
		JOIN series s ON s.id = vm.series_id
		WHERE vm.%s = $1`, field)
	err := r.pool.QueryRow(ctx, query, value).Scan(&m.ID, &m.BrandID, &m.BrandName, &m.SeriesID, &m.SeriesName,
		&m.ModelCode, &m.ModelName, &m.Year, &m.Description, &m.PublicationStatus,
		&m.StockQuantity, &m.ExpiryDate, &m.DiscontinuedAt, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
