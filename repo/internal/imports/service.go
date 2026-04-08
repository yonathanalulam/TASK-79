package imports

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ImportJob struct {
	ID            int       `json:"id"`
	Filename      string    `json:"filename"`
	Status        string    `json:"status"`
	TotalRows     int       `json:"total_rows"`
	ValidRows     int       `json:"valid_rows"`
	InvalidRows   int       `json:"invalid_rows"`
	CommittedRows int       `json:"committed_rows"`
	UploadedBy    *int      `json:"uploaded_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type ImportRow struct {
	ID        int             `json:"id"`
	JobID     int             `json:"job_id"`
	RowNumber int             `json:"row_number"`
	RawData   json.RawMessage `json:"raw_data"`
	Status    string          `json:"status"`
	Errors    json.RawMessage `json:"errors"`
}

var requiredHeaders = []string{"model_code", "model_name", "brand", "series", "year"}

type Service struct {
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, auditSvc: auditSvc}
}

func (s *Service) ParseAndValidate(ctx context.Context, filename string, reader io.Reader, uploadedBy int) (*ImportJob, []ImportRow, error) {
	csvReader := csv.NewReader(reader)
	headers, err := csvReader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read headers: %w", err)
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[h] = i
	}

	for _, req := range requiredHeaders {
		if _, ok := headerMap[req]; !ok {
			return nil, nil, fmt.Errorf("missing required column: %s", req)
		}
	}

	// Create job
	var jobID int
	err = s.pool.QueryRow(ctx, `INSERT INTO csv_import_jobs (filename, uploaded_by) VALUES ($1, $2) RETURNING id`, filename, uploadedBy).Scan(&jobID)
	if err != nil {
		return nil, nil, err
	}

	var rows []ImportRow
	rowNum := 0
	validCount := 0
	invalidCount := 0

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rowNum++

		data := make(map[string]string)
		for name, idx := range headerMap {
			if idx < len(record) {
				data[name] = record[idx]
			}
		}

		rawData, _ := json.Marshal(data)
		rowErrors := s.validateRow(ctx, data, rowNum)

		status := "valid"
		var errJSON json.RawMessage
		if len(rowErrors) > 0 {
			status = "invalid"
			errJSON, _ = json.Marshal(rowErrors)
			invalidCount++
		} else {
			validCount++
		}

		var rowID int
		s.pool.QueryRow(ctx, `INSERT INTO csv_import_rows (job_id, row_number, raw_data, status, errors) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			jobID, rowNum, rawData, status, errJSON).Scan(&rowID)

		rows = append(rows, ImportRow{
			ID:        rowID,
			JobID:     jobID,
			RowNumber: rowNum,
			RawData:   rawData,
			Status:    status,
			Errors:    errJSON,
		})
	}

	// Update job counts
	s.pool.Exec(ctx, `UPDATE csv_import_jobs SET total_rows=$1, valid_rows=$2, invalid_rows=$3, status='validated' WHERE id=$4`,
		rowNum, validCount, invalidCount, jobID)

	job := &ImportJob{
		ID:          jobID,
		Filename:    filename,
		Status:      "validated",
		TotalRows:   rowNum,
		ValidRows:   validCount,
		InvalidRows: invalidCount,
		UploadedBy:  &uploadedBy,
	}

	return job, rows, nil
}

func (s *Service) validateRow(ctx context.Context, data map[string]string, rowNum int) map[string]string {
	errs := make(map[string]string)

	if data["model_code"] == "" {
		errs["model_code"] = "Model code is required"
	}
	if data["model_name"] == "" {
		errs["model_name"] = "Model name is required"
	}
	if data["brand"] == "" {
		errs["brand"] = "Brand is required"
	}
	if data["series"] == "" {
		errs["series"] = "Series is required"
	}

	if yr := data["year"]; yr != "" {
		y, err := strconv.Atoi(yr)
		if err != nil || y < 1900 || y > 2100 {
			errs["year"] = "Year must be between 1900 and 2100"
		}
	} else {
		errs["year"] = "Year is required"
	}

	if sq := data["stock_quantity"]; sq != "" {
		if n, err := strconv.Atoi(sq); err != nil || n < 0 {
			errs["stock_quantity"] = "Stock quantity must be a non-negative integer"
		}
	}

	if ps := data["publication_status"]; ps != "" {
		validStatuses := map[string]bool{"draft": true, "published": true, "unpublished": true}
		if !validStatuses[ps] {
			errs["publication_status"] = "Invalid publication status"
		}
	}

	if ed := data["expiry_date"]; ed != "" {
		if _, err := time.Parse("2006-01-02", ed); err != nil {
			errs["expiry_date"] = "Invalid date format (expected YYYY-MM-DD)"
		}
	}

	// Check for duplicate model code
	if data["model_code"] != "" {
		var exists int
		s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_models WHERE model_code=$1`, data["model_code"]).Scan(&exists)
		if exists > 0 {
			errs["model_code"] = "Duplicate model code already exists in database"
		}
	}

	return errs
}

func (s *Service) GetJob(ctx context.Context, jobID int) (*ImportJob, error) {
	var job ImportJob
	err := s.pool.QueryRow(ctx, `SELECT id, filename, status, total_rows, valid_rows, invalid_rows, committed_rows, uploaded_by, created_at FROM csv_import_jobs WHERE id=$1`, jobID).
		Scan(&job.ID, &job.Filename, &job.Status, &job.TotalRows, &job.ValidRows, &job.InvalidRows, &job.CommittedRows, &job.UploadedBy, &job.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Service) GetJobRows(ctx context.Context, jobID int) ([]ImportRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, job_id, row_number, raw_data, status, errors FROM csv_import_rows WHERE job_id=$1 ORDER BY row_number`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ImportRow
	for rows.Next() {
		var r ImportRow
		if err := rows.Scan(&r.ID, &r.JobID, &r.RowNumber, &r.RawData, &r.Status, &r.Errors); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *Service) CommitJob(ctx context.Context, jobID int, actorID int) error {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "validated" {
		return errors.New("job is not in validated state")
	}

	// Load valid rows before starting transaction
	validRows, err := s.pool.Query(ctx, `SELECT id, raw_data FROM csv_import_rows WHERE job_id=$1 AND status='valid'`, jobID)
	if err != nil {
		return err
	}
	type rowData struct {
		ID   int
		Data map[string]string
	}
	var rows []rowData
	for validRows.Next() {
		var id int
		var raw json.RawMessage
		if err := validRows.Scan(&id, &raw); err != nil {
			validRows.Close()
			return fmt.Errorf("scan row: %w", err)
		}
		var d map[string]string
		json.Unmarshal(raw, &d)
		rows = append(rows, rowData{ID: id, Data: d})
	}
	validRows.Close()

	// All-or-nothing transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	committed := 0
	for _, row := range rows {
		data := row.Data

		// Find or create brand
		var brandID int
		err := tx.QueryRow(ctx, `SELECT id FROM brands WHERE name=$1`, data["brand"]).Scan(&brandID)
		if err != nil {
			if err := tx.QueryRow(ctx, `INSERT INTO brands (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, data["brand"]).Scan(&brandID); err != nil {
				return fmt.Errorf("row %d: create brand: %w", row.ID, err)
			}
		}

		// Find or create series
		var seriesID int
		err = tx.QueryRow(ctx, `SELECT id FROM series WHERE brand_id=$1 AND name=$2`, brandID, data["series"]).Scan(&seriesID)
		if err != nil {
			if err := tx.QueryRow(ctx, `INSERT INTO series (brand_id, name) VALUES ($1, $2) ON CONFLICT (brand_id, name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, brandID, data["series"]).Scan(&seriesID); err != nil {
				return fmt.Errorf("row %d: create series: %w", row.ID, err)
			}
		}

		year, _ := strconv.Atoi(data["year"])
		stock := 0
		if sv, err := strconv.Atoi(data["stock_quantity"]); err == nil {
			stock = sv
		}
		pubStatus := "draft"
		if ps := data["publication_status"]; ps != "" {
			pubStatus = ps
		}

		if _, err := tx.Exec(ctx, `INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, description, publication_status, stock_quantity, expiry_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			brandID, seriesID, data["model_code"], data["model_name"], year, data["description"], pubStatus, stock, nilIfEmpty(data["expiry_date"])); err != nil {
			return fmt.Errorf("row %d: insert vehicle: %w", row.ID, err)
		}

		if _, err := tx.Exec(ctx, `UPDATE csv_import_rows SET status='committed' WHERE id=$1`, row.ID); err != nil {
			return fmt.Errorf("row %d: update status: %w", row.ID, err)
		}
		committed++
	}

	if _, err := tx.Exec(ctx, `UPDATE csv_import_jobs SET status='committed', committed_rows=$1, completed_at=NOW() WHERE id=$2`, committed, jobID); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		// On commit failure, mark job as failed outside the rolled-back tx
		s.pool.Exec(ctx, `UPDATE csv_import_jobs SET status='failed', completed_at=NOW() WHERE id=$1`, jobID)
		return fmt.Errorf("commit: %w", err)
	}

	s.auditSvc.Log(ctx, audit.LogParams{
		EntityType: "csv_import", EntityID: jobID, Action: "committed", ActorUserID: &actorID,
		Metadata: map[string]int{"committed_rows": committed},
	})
	return nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
