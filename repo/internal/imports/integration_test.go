package imports_test

import (
	"context"
	"strings"
	"testing"

	"fleetcommerce/internal/imports"
	"fleetcommerce/internal/testutil"
)

func TestParseAndValidateCreatesJobAndRows(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := imports.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Import User", "import@test.com")

	csv := "model_code,model_name,brand,series,year,stock_quantity\nMC001,Model A,BrandA,SerA,2024,10\nMC002,Model B,BrandB,SerB,2024,5"
	reader := strings.NewReader(csv)

	job, rows, err := svc.ParseAndValidate(ctx, "test.csv", reader, userID)
	if err != nil {
		t.Fatalf("ParseAndValidate: %v", err)
	}
	if job.Status != "validated" {
		t.Errorf("expected validated, got %q", job.Status)
	}
	if job.TotalRows != 2 {
		t.Errorf("expected 2 total rows, got %d", job.TotalRows)
	}
	if job.ValidRows != 2 {
		t.Errorf("expected 2 valid rows, got %d", job.ValidRows)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

func TestCommitJobAtomicity(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := imports.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Import User", "import-commit@test.com")

	csv := "model_code,model_name,brand,series,year\nMC100,Commit Model,BrandC,SerC,2024"
	reader := strings.NewReader(csv)

	job, _, err := svc.ParseAndValidate(ctx, "commit.csv", reader, userID)
	if err != nil {
		t.Fatalf("ParseAndValidate: %v", err)
	}

	// Commit should succeed
	if err := svc.CommitJob(ctx, job.ID, userID); err != nil {
		t.Fatalf("CommitJob: %v", err)
	}

	// Verify job status is committed
	committedJob, err := svc.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if committedJob.Status != "committed" {
		t.Errorf("expected committed status, got %q", committedJob.Status)
	}
	if committedJob.CommittedRows != 1 {
		t.Errorf("expected 1 committed row, got %d", committedJob.CommittedRows)
	}

	// Verify vehicle was actually created
	var vehicleCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_models WHERE model_code='MC100'`).Scan(&vehicleCount)
	if vehicleCount != 1 {
		t.Errorf("expected 1 vehicle with model_code MC100, got %d", vehicleCount)
	}
}

func TestCommitJobRejectsNonValidatedJob(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := imports.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Import User", "import-reject@test.com")

	// Create a job directly in 'committed' status
	var jobID int
	pool.QueryRow(ctx, `INSERT INTO csv_import_jobs (filename, status, uploaded_by) VALUES ('already.csv','committed',$1) RETURNING id`, userID).Scan(&jobID)

	err := svc.CommitJob(ctx, jobID, userID)
	if err == nil {
		t.Fatal("expected error committing non-validated job")
	}
}

func TestParseAndValidateRejectsInvalidRows(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := imports.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Import User", "import-invalid@test.com")

	// Missing required fields
	csv := "model_code,model_name,brand,series,year\n,,,,\nMC200,Valid,BrandD,SerD,2024"
	reader := strings.NewReader(csv)

	job, _, err := svc.ParseAndValidate(ctx, "mixed.csv", reader, userID)
	if err != nil {
		t.Fatalf("ParseAndValidate: %v", err)
	}
	if job.InvalidRows != 1 {
		t.Errorf("expected 1 invalid row, got %d", job.InvalidRows)
	}
	if job.ValidRows != 1 {
		t.Errorf("expected 1 valid row, got %d", job.ValidRows)
	}
}

func TestMissingHeadersRejected(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := imports.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Import User", "import-headers@test.com")

	csv := "model_code,model_name\nMC300,Name"
	reader := strings.NewReader(csv)

	_, _, err := svc.ParseAndValidate(ctx, "bad_headers.csv", reader, userID)
	if err == nil {
		t.Fatal("expected error for missing required headers")
	}
}
