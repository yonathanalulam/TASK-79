package metrics_test

import (
	"context"
	"testing"

	"fleetcommerce/internal/metrics"
	"fleetcommerce/internal/testutil"
)

func TestCreateMetricPersistsIsDerived(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-derived@test.com")

	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name:        "Derived Revenue",
		Description: "Sum of sub-metrics",
		IsDerived:   true,
		TimeGrain:   "daily",
		OwnerID:     userID,
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	versions, err := svc.ListVersions(ctx, id)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if !versions[0].IsDerived {
		t.Error("expected is_derived=true on persisted version")
	}
}

func TestCreateMetricPersistsDimensionsFiltersDepencies(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-dims@test.com")

	// Create a base metric for dependency
	baseID, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Base Metric", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("create base metric: %v", err)
	}

	// Create derived metric with dimensions, filters, dependencies, and window
	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name:              "Complex Metric",
		Description:       "A complex derived metric",
		SQLExpression:     "SUM(revenue) OVER (ORDER BY date ROWS 7 PRECEDING)",
		SemanticFormula:   "revenue.sum",
		TimeGrain:         "weekly",
		IsDerived:         true,
		WindowCalculation: "7d_rolling_sum",
		OwnerID:           userID,
		DependsOnMetrics:  []int{baseID},
		Filters: []metrics.MetricFilterDef{
			{Name: "region", Expression: "region = 'US'"},
			{Name: "channel", Expression: "channel IN ('web','mobile')"},
		},
		Dimensions: []metrics.DimensionDef{
			{Name: "region", Description: "Geographic region"},
			{Name: "product_line", Description: "Product category"},
		},
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	// Verify dimensions
	dims, err := svc.ListDimensions(ctx, id)
	if err != nil {
		t.Fatalf("ListDimensions: %v", err)
	}
	if len(dims) != 2 {
		t.Errorf("expected 2 dimensions, got %d", len(dims))
	}

	// Verify filters
	filters, err := svc.ListFilters(ctx, id)
	if err != nil {
		t.Fatalf("ListFilters: %v", err)
	}
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}

	// Verify dependencies
	deps, err := svc.ListDependencies(ctx, id)
	if err != nil {
		t.Fatalf("ListDependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
	if len(deps) > 0 && deps[0].DependsOnMetric != baseID {
		t.Errorf("expected dependency on metric %d, got %d", baseID, deps[0].DependsOnMetric)
	}

	// Verify version has window_calculation
	versions, err := svc.ListVersions(ctx, id)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least 1 version")
	}
	if versions[0].WindowCalculation != "7d_rolling_sum" {
		t.Errorf("expected window_calculation='7d_rolling_sum', got %q", versions[0].WindowCalculation)
	}

	// Verify lineage
	lineage, err := svc.GetLineage(ctx, id)
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(lineage) == 0 {
		t.Error("expected at least 1 lineage edge")
	}
}

func TestUpdateMetricCreatesNewVersion(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-update@test.com")

	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Update Test", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	// Update with new version
	err = svc.UpdateMetric(ctx, metrics.UpdateParams{
		MetricID:          id,
		Description:       "Updated description",
		SQLExpression:     "COUNT(DISTINCT user_id)",
		TimeGrain:         "monthly",
		IsDerived:         true,
		WindowCalculation: "30d_rolling",
		ActorID:           userID,
		Filters:           []metrics.MetricFilterDef{{Name: "active", Expression: "is_active=true"}},
		Dimensions:        []metrics.DimensionDef{{Name: "country", Description: "Country"}},
	})
	if err != nil {
		t.Fatalf("UpdateMetric: %v", err)
	}

	versions, err := svc.ListVersions(ctx, id)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Latest version should be version 2
	if versions[0].VersionNumber != 2 {
		t.Errorf("expected latest version number 2, got %d", versions[0].VersionNumber)
	}
	if !versions[0].IsDerived {
		t.Error("expected is_derived=true on version 2")
	}

	// Updated metric description
	metric, err := svc.GetMetric(ctx, id)
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if metric.Description != "Updated description" {
		t.Errorf("expected updated description, got %q", metric.Description)
	}
}

func TestImpactAnalysisWithDependencies(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-impact@test.com")

	// Create base metric (stays draft — not active)
	baseID, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Inactive Base", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("create base: %v", err)
	}

	// Create dependent metric
	derivedID, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Derived", OwnerID: userID, IsDerived: true,
		DependsOnMetrics: []int{baseID}, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("create derived: %v", err)
	}

	// Impact analysis should report missing deps (base is draft, not active)
	analysis, err := svc.RunImpactAnalysis(ctx, derivedID)
	if err != nil {
		t.Fatalf("RunImpactAnalysis: %v", err)
	}
	if analysis.MissingDeps != 1 {
		t.Errorf("expected 1 missing dep, got %d", analysis.MissingDeps)
	}
	if analysis.Status != "rejected" {
		t.Errorf("expected rejected status, got %q", analysis.Status)
	}

	// Activation should fail due to missing deps
	err = svc.ActivateMetric(ctx, derivedID, userID)
	if err == nil {
		t.Fatal("expected activation to fail")
	}
}

func TestActivateMetricWithApprovedAnalysis(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-activate@test.com")

	// Create standalone metric (no deps)
	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Standalone Metric", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	// Activation without impact analysis should fail
	err = svc.ActivateMetric(ctx, id, userID)
	if err != metrics.ErrImpactAnalysisRequired {
		t.Fatalf("expected ErrImpactAnalysisRequired, got %v", err)
	}

	// Run impact analysis (should approve since no deps)
	analysis, err := svc.RunImpactAnalysis(ctx, id)
	if err != nil {
		t.Fatalf("RunImpactAnalysis: %v", err)
	}
	if analysis.Status != "approved" {
		t.Fatalf("expected approved, got %q", analysis.Status)
	}

	// Now activation should succeed
	err = svc.ActivateMetric(ctx, id, userID)
	if err != nil {
		t.Fatalf("ActivateMetric: %v", err)
	}

	// Verify status
	metric, err := svc.GetMetric(ctx, id)
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if metric.Status != "active" {
		t.Errorf("expected active status, got %q", metric.Status)
	}
}

func TestInvalidFilterExpressionValidation(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-filter@test.com")

	// Filter with empty name should fail (DB constraint — name NOT NULL, VARCHAR(100))
	_, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Filter Test", OwnerID: userID, TimeGrain: "daily",
		Filters: []metrics.MetricFilterDef{{Name: "", Expression: "x=1"}},
	})
	// Empty filter name should cause a DB constraint failure
	if err == nil {
		t.Error("expected error for filter with empty name")
	}
}

// TestAddAndRemoveDimension verifies individual dimension management.
func TestAddAndRemoveDimension(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-add-dim@test.com")

	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Dim Mgmt Test", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	// Add dimension
	dimID, err := svc.AddDimension(ctx, id, metrics.DimensionDef{Name: "region", Description: "Geographic region"}, userID)
	if err != nil {
		t.Fatalf("AddDimension: %v", err)
	}
	if dimID == 0 {
		t.Error("expected non-zero dimension ID")
	}

	// Verify it's listed
	dims, _ := svc.ListDimensions(ctx, id)
	if len(dims) != 1 {
		t.Fatalf("expected 1 dimension, got %d", len(dims))
	}
	if dims[0].Name != "region" {
		t.Errorf("expected dimension name 'region', got %q", dims[0].Name)
	}

	// Add another
	dimID2, _ := svc.AddDimension(ctx, id, metrics.DimensionDef{Name: "channel", Description: "Sales channel"}, userID)

	dims, _ = svc.ListDimensions(ctx, id)
	if len(dims) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(dims))
	}

	// Remove the first
	if err := svc.RemoveDimension(ctx, id, dimID, userID); err != nil {
		t.Fatalf("RemoveDimension: %v", err)
	}

	dims, _ = svc.ListDimensions(ctx, id)
	if len(dims) != 1 {
		t.Fatalf("expected 1 dimension after removal, got %d", len(dims))
	}
	if dims[0].ID != dimID2 {
		t.Error("wrong dimension remained after removal")
	}

	// Verify audit for add/remove
	var auditCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE entity_type='metric_dimension'`).Scan(&auditCount)
	if auditCount < 2 {
		t.Errorf("expected at least 2 audit entries for dimension management, got %d", auditCount)
	}
}

// TestAddAndRemoveFilter verifies individual filter management.
func TestAddAndRemoveFilter(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-add-filter@test.com")

	id, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Filter Mgmt Test", OwnerID: userID, TimeGrain: "daily",
	})
	if err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}

	// Add filter
	filterID, err := svc.AddFilter(ctx, id, metrics.MetricFilterDef{Name: "active_only", Expression: "is_active=true"}, userID)
	if err != nil {
		t.Fatalf("AddFilter: %v", err)
	}

	filters, _ := svc.ListFilters(ctx, id)
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	if filters[0].Expression != "is_active=true" {
		t.Errorf("filter expression mismatch: %q", filters[0].Expression)
	}

	// Remove filter
	if err := svc.RemoveFilter(ctx, id, filterID, userID); err != nil {
		t.Fatalf("RemoveFilter: %v", err)
	}
	filters, _ = svc.ListFilters(ctx, id)
	if len(filters) != 0 {
		t.Errorf("expected 0 filters after removal, got %d", len(filters))
	}
}

// TestAddAndRemoveDependency verifies individual dependency management and lineage coherence.
func TestAddAndRemoveDependency(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-add-dep@test.com")

	baseID, _ := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Dep Base", OwnerID: userID, TimeGrain: "daily",
	})
	derivedID, _ := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Dep Derived", OwnerID: userID, IsDerived: true, TimeGrain: "daily",
	})

	// Add dependency
	depID, err := svc.AddDependency(ctx, derivedID, baseID, userID)
	if err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	deps, _ := svc.ListDependencies(ctx, derivedID)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].DependsOnMetric != baseID {
		t.Errorf("expected dependency on %d, got %d", baseID, deps[0].DependsOnMetric)
	}

	// Verify lineage edge was created
	lineage, _ := svc.GetLineage(ctx, derivedID)
	if len(lineage) == 0 {
		t.Error("expected lineage edge after adding dependency")
	}

	// Remove dependency
	if err := svc.RemoveDependency(ctx, derivedID, depID, userID); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}

	deps, _ = svc.ListDependencies(ctx, derivedID)
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies after removal, got %d", len(deps))
	}

	// Lineage edge should be cleaned up
	lineage, _ = svc.GetLineage(ctx, derivedID)
	if len(lineage) != 0 {
		t.Errorf("expected 0 lineage edges after dependency removal, got %d", len(lineage))
	}
}

// TestSelfDependencyRejected verifies a metric cannot depend on itself.
func TestSelfDependencyRejected(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-self-dep@test.com")

	id, _ := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Self Dep Test", OwnerID: userID, TimeGrain: "daily",
	})

	_, err := svc.AddDependency(ctx, id, id, userID)
	if err == nil {
		t.Fatal("expected error for self-referencing dependency")
	}
}

// TestRemoveDimensionFromWrongMetricFails verifies defense-in-depth:
// you cannot remove a dimension that belongs to a different metric.
func TestRemoveDimensionFromWrongMetricFails(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-wrong-dim@test.com")

	m1, _ := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Metric A", OwnerID: userID, TimeGrain: "daily",
	})
	m2, _ := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Metric B", OwnerID: userID, TimeGrain: "daily",
	})

	dimID, _ := svc.AddDimension(ctx, m1, metrics.DimensionDef{Name: "owned_by_m1"}, userID)

	// Try to remove m1's dimension using m2's ID — should fail
	err := svc.RemoveDimension(ctx, m2, dimID, userID)
	if err == nil {
		t.Fatal("expected error removing dimension from wrong metric")
	}
}

func TestDuplicateDimensionNameFails(t *testing.T) {
	pool := testutil.MustDB(t)
	auditSvc := testutil.MustAudit(t, pool)
	svc := metrics.NewService(pool, auditSvc)
	ctx := context.Background()
	userID := testutil.SeedUser(t, pool, "Test User", "metric-dupedim@test.com")

	_, err := svc.CreateMetric(ctx, metrics.CreateParams{
		Name: "Dupe Dim Test", OwnerID: userID, TimeGrain: "daily",
		Dimensions: []metrics.DimensionDef{
			{Name: "region", Description: "Region A"},
			{Name: "region", Description: "Region B"},
		},
	})
	if err == nil {
		t.Error("expected error for duplicate dimension names")
	}
}
