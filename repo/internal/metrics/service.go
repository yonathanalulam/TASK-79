package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrImpactAnalysisRequired = errors.New("impact analysis must be run and approved before activation")
	ErrMissingDependencies    = errors.New("metric has unresolved dependencies")
)

type MetricDefinition struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	OwnerID         *int      `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	LatestVersion   int       `json:"latest_version"`
	DependencyCount int       `json:"dependency_count"`
	CreatedAt       time.Time `json:"created_at"`
}

type MetricVersion struct {
	ID                int       `json:"id"`
	MetricID          int       `json:"metric_id"`
	VersionNumber     int       `json:"version_number"`
	SQLExpression     string    `json:"sql_expression"`
	SemanticFormula   string    `json:"semantic_formula"`
	TimeGrain         string    `json:"time_grain"`
	Description       string    `json:"description"`
	IsDerived         bool      `json:"is_derived"`
	WindowCalculation string    `json:"window_calculation"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

type Dimension struct {
	ID          int    `json:"id"`
	MetricID    int    `json:"metric_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type LineageEdge struct {
	ID         int    `json:"id"`
	SourceType string `json:"source_type"`
	SourceID   int    `json:"source_id"`
	SourceName string `json:"source_name"`
	TargetType string `json:"target_type"`
	TargetID   int    `json:"target_id"`
	TargetName string `json:"target_name"`
}

type ImpactAnalysis struct {
	DependentMetrics int    `json:"dependent_metrics"`
	DependentCharts  int    `json:"dependent_charts"`
	MissingDeps      int    `json:"missing_deps"`
	Status           string `json:"status"`
}

type CreateParams struct {
	Name              string
	Description       string
	SQLExpression     string
	SemanticFormula   string
	TimeGrain         string
	IsDerived         bool
	WindowCalculation string
	OwnerID           int
	DependsOnMetrics  []int             // metric IDs this metric depends on
	Filters           []MetricFilterDef // filter definitions
	Dimensions        []DimensionDef    // dimension definitions
}

type MetricFilter struct {
	ID         int    `json:"id"`
	MetricID   int    `json:"metric_id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

type MetricDependency struct {
	ID              int    `json:"id"`
	MetricID        int    `json:"metric_id"`
	DependsOnMetric int    `json:"depends_on_metric"`
	DependsOnName   string `json:"depends_on_name"`
}

type MetricFilterDef struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

type DimensionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateParams struct {
	MetricID          int
	Description       string
	SQLExpression     string
	SemanticFormula   string
	TimeGrain         string
	IsDerived         bool
	WindowCalculation string
	ActorID           int
	DependsOnMetrics  []int
	Filters           []MetricFilterDef
	Dimensions        []DimensionDef
}

type Service struct {
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, auditSvc: auditSvc}
}

// ListMetrics returns metrics filtered by metric-level permissions.
// Metrics with no rows in metric_permissions are visible to all with role-level access.
// Metrics with metric_permissions rows are only visible to permitted users.
func (s *Service) ListMetrics(ctx context.Context, userID int) ([]MetricDefinition, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.id, m.name, COALESCE(m.description,''), m.status, m.owner_id, COALESCE(u.full_name,''),
		COALESCE((SELECT MAX(version_number) FROM metric_definition_versions WHERE metric_id=m.id), 0),
		(SELECT COUNT(*) FROM metric_dependencies WHERE metric_id=m.id),
		m.created_at
		FROM metric_definitions m LEFT JOIN users u ON u.id = m.owner_id
		WHERE NOT EXISTS (SELECT 1 FROM metric_permissions WHERE metric_id = m.id)
		   OR EXISTS (SELECT 1 FROM metric_permissions mp WHERE mp.metric_id = m.id AND mp.can_view = TRUE
		              AND (mp.user_id = $1 OR mp.role_id IN (SELECT role_id FROM user_roles WHERE user_id = $1)))
		ORDER BY m.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []MetricDefinition
	for rows.Next() {
		var m MetricDefinition
		if err := rows.Scan(&m.ID, &m.Name, &m.Description, &m.Status, &m.OwnerID, &m.OwnerName, &m.LatestVersion, &m.DependencyCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (s *Service) GetMetric(ctx context.Context, id int) (*MetricDefinition, error) {
	var m MetricDefinition
	err := s.pool.QueryRow(ctx, `SELECT m.id, m.name, COALESCE(m.description,''), m.status, m.owner_id, COALESCE(u.full_name,''),
		COALESCE((SELECT MAX(version_number) FROM metric_definition_versions WHERE metric_id=m.id), 0),
		(SELECT COUNT(*) FROM metric_dependencies WHERE metric_id=m.id),
		m.created_at
		FROM metric_definitions m LEFT JOIN users u ON u.id = m.owner_id
		WHERE m.id=$1`, id).Scan(&m.ID, &m.Name, &m.Description, &m.Status, &m.OwnerID, &m.OwnerName, &m.LatestVersion, &m.DependencyCount, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Service) CreateMetric(ctx context.Context, p CreateParams) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, `INSERT INTO metric_definitions (name, description, owner_id) VALUES ($1,$2,$3) RETURNING id`,
		p.Name, p.Description, p.OwnerID).Scan(&id)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO metric_definition_versions (metric_id, version_number, sql_expression, semantic_formula, time_grain, is_derived, window_calculation, status)
		VALUES ($1, 1, $2, $3, $4, $5, $6, 'draft')`, id, p.SQLExpression, p.SemanticFormula, p.TimeGrain, p.IsDerived, p.WindowCalculation); err != nil {
		return 0, fmt.Errorf("create version: %w", err)
	}

	// Persist dependencies
	for _, depID := range p.DependsOnMetrics {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_dependencies (metric_id, depends_on_metric) VALUES ($1, $2)`, id, depID); err != nil {
			return 0, fmt.Errorf("create dependency: %w", err)
		}
		// Also record lineage edge
		if _, err := tx.Exec(ctx, `INSERT INTO metric_lineage_edges (source_type, source_id, target_type, target_id) VALUES ('metric', $1, 'metric', $2)`, depID, id); err != nil {
			return 0, fmt.Errorf("create lineage edge: %w", err)
		}
	}

	// Persist filters
	for _, f := range p.Filters {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_filters (metric_id, name, expression) VALUES ($1, $2, $3)`, id, f.Name, f.Expression); err != nil {
			return 0, fmt.Errorf("create filter: %w", err)
		}
	}

	// Persist dimensions
	for _, d := range p.Dimensions {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_dimensions (metric_id, name, description) VALUES ($1, $2, $3)`, id, d.Name, d.Description); err != nil {
			return 0, fmt.Errorf("create dimension: %w", err)
		}
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric", EntityID: id, Action: "created", ActorUserID: &p.OwnerID,
	}); err != nil {
		return 0, fmt.Errorf("write audit: %w", err)
	}

	return id, tx.Commit(ctx)
}

// UpdateMetric creates a new version of the metric and replaces its dimensions, filters, and dependencies.
func (s *Service) UpdateMetric(ctx context.Context, p UpdateParams) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get current latest version number
	var latestVersion int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_number),0) FROM metric_definition_versions WHERE metric_id=$1`, p.MetricID).Scan(&latestVersion); err != nil {
		return fmt.Errorf("get latest version: %w", err)
	}
	newVersion := latestVersion + 1

	// Update description on metric_definitions
	if _, err := tx.Exec(ctx, `UPDATE metric_definitions SET description=$1, updated_at=NOW() WHERE id=$2`, p.Description, p.MetricID); err != nil {
		return fmt.Errorf("update metric: %w", err)
	}

	// Create new version
	if _, err := tx.Exec(ctx, `INSERT INTO metric_definition_versions (metric_id, version_number, sql_expression, semantic_formula, time_grain, is_derived, window_calculation, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft')`, p.MetricID, newVersion, p.SQLExpression, p.SemanticFormula, p.TimeGrain, p.IsDerived, p.WindowCalculation); err != nil {
		return fmt.Errorf("create version: %w", err)
	}

	// Replace dependencies
	if _, err := tx.Exec(ctx, `DELETE FROM metric_dependencies WHERE metric_id=$1`, p.MetricID); err != nil {
		return fmt.Errorf("clear dependencies: %w", err)
	}
	// Clear old lineage edges where this metric is the target
	if _, err := tx.Exec(ctx, `DELETE FROM metric_lineage_edges WHERE target_type='metric' AND target_id=$1 AND source_type='metric'`, p.MetricID); err != nil {
		return fmt.Errorf("clear lineage: %w", err)
	}
	for _, depID := range p.DependsOnMetrics {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_dependencies (metric_id, depends_on_metric) VALUES ($1, $2)`, p.MetricID, depID); err != nil {
			return fmt.Errorf("create dependency: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO metric_lineage_edges (source_type, source_id, target_type, target_id) VALUES ('metric', $1, 'metric', $2)`, depID, p.MetricID); err != nil {
			return fmt.Errorf("create lineage edge: %w", err)
		}
	}

	// Replace filters
	if _, err := tx.Exec(ctx, `DELETE FROM metric_filters WHERE metric_id=$1`, p.MetricID); err != nil {
		return fmt.Errorf("clear filters: %w", err)
	}
	for _, f := range p.Filters {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_filters (metric_id, name, expression) VALUES ($1, $2, $3)`, p.MetricID, f.Name, f.Expression); err != nil {
			return fmt.Errorf("create filter: %w", err)
		}
	}

	// Replace dimensions
	if _, err := tx.Exec(ctx, `DELETE FROM metric_dimensions WHERE metric_id=$1`, p.MetricID); err != nil {
		return fmt.Errorf("clear dimensions: %w", err)
	}
	for _, d := range p.Dimensions {
		if _, err := tx.Exec(ctx, `INSERT INTO metric_dimensions (metric_id, name, description) VALUES ($1, $2, $3)`, p.MetricID, d.Name, d.Description); err != nil {
			return fmt.Errorf("create dimension: %w", err)
		}
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric", EntityID: p.MetricID, Action: "updated", ActorUserID: &p.ActorID,
		Metadata: map[string]interface{}{"new_version": newVersion},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) ListFilters(ctx context.Context, metricID int) ([]MetricFilter, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, metric_id, name, expression FROM metric_filters WHERE metric_id=$1`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []MetricFilter
	for rows.Next() {
		var f MetricFilter
		if err := rows.Scan(&f.ID, &f.MetricID, &f.Name, &f.Expression); err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

func (s *Service) ListDependencies(ctx context.Context, metricID int) ([]MetricDependency, error) {
	rows, err := s.pool.Query(ctx, `SELECT md.id, md.metric_id, md.depends_on_metric, COALESCE(m.name,'')
		FROM metric_dependencies md LEFT JOIN metric_definitions m ON m.id = md.depends_on_metric
		WHERE md.metric_id=$1`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []MetricDependency
	for rows.Next() {
		var d MetricDependency
		if err := rows.Scan(&d.ID, &d.MetricID, &d.DependsOnMetric, &d.DependsOnName); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// AddDimension adds a single dimension to a metric.
func (s *Service) AddDimension(ctx context.Context, metricID int, d DimensionDef, actorID int) (int, error) {
	if d.Name == "" {
		return 0, errors.New("dimension name is required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	if err := tx.QueryRow(ctx, `INSERT INTO metric_dimensions (metric_id, name, description) VALUES ($1,$2,$3) RETURNING id`,
		metricID, d.Name, d.Description).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert dimension: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_dimension", EntityID: id, Action: "added", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID, "name": d.Name},
	}); err != nil {
		return 0, fmt.Errorf("write audit: %w", err)
	}
	return id, tx.Commit(ctx)
}

// RemoveDimension removes a dimension from a metric.
func (s *Service) RemoveDimension(ctx context.Context, metricID, dimensionID, actorID int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM metric_dimensions WHERE id=$1 AND metric_id=$2`, dimensionID, metricID)
	if err != nil {
		return fmt.Errorf("delete dimension: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("dimension not found or does not belong to this metric")
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_dimension", EntityID: dimensionID, Action: "removed", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}
	return tx.Commit(ctx)
}

// AddFilter adds a single filter to a metric.
func (s *Service) AddFilter(ctx context.Context, metricID int, f MetricFilterDef, actorID int) (int, error) {
	if f.Name == "" {
		return 0, errors.New("filter name is required")
	}
	if f.Expression == "" {
		return 0, errors.New("filter expression is required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	if err := tx.QueryRow(ctx, `INSERT INTO metric_filters (metric_id, name, expression) VALUES ($1,$2,$3) RETURNING id`,
		metricID, f.Name, f.Expression).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert filter: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_filter", EntityID: id, Action: "added", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID, "name": f.Name},
	}); err != nil {
		return 0, fmt.Errorf("write audit: %w", err)
	}
	return id, tx.Commit(ctx)
}

// RemoveFilter removes a filter from a metric.
func (s *Service) RemoveFilter(ctx context.Context, metricID, filterID, actorID int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM metric_filters WHERE id=$1 AND metric_id=$2`, filterID, metricID)
	if err != nil {
		return fmt.Errorf("delete filter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("filter not found or does not belong to this metric")
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_filter", EntityID: filterID, Action: "removed", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}
	return tx.Commit(ctx)
}

// AddDependency adds a dependency relationship between metrics and records lineage.
func (s *Service) AddDependency(ctx context.Context, metricID, dependsOnMetricID, actorID int) (int, error) {
	if metricID == dependsOnMetricID {
		return 0, errors.New("metric cannot depend on itself")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int
	if err := tx.QueryRow(ctx, `INSERT INTO metric_dependencies (metric_id, depends_on_metric) VALUES ($1,$2) RETURNING id`,
		metricID, dependsOnMetricID).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert dependency: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO metric_lineage_edges (source_type, source_id, target_type, target_id) VALUES ('metric', $1, 'metric', $2)`,
		dependsOnMetricID, metricID); err != nil {
		return 0, fmt.Errorf("insert lineage edge: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_dependency", EntityID: id, Action: "added", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID, "depends_on": dependsOnMetricID},
	}); err != nil {
		return 0, fmt.Errorf("write audit: %w", err)
	}
	return id, tx.Commit(ctx)
}

// RemoveDependency removes a dependency relationship and its lineage edge.
func (s *Service) RemoveDependency(ctx context.Context, metricID, dependencyID, actorID int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get the depends_on_metric before deleting
	var dependsOn int
	if err := tx.QueryRow(ctx, `SELECT depends_on_metric FROM metric_dependencies WHERE id=$1 AND metric_id=$2`,
		dependencyID, metricID).Scan(&dependsOn); err != nil {
		return errors.New("dependency not found or does not belong to this metric")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM metric_dependencies WHERE id=$1`, dependencyID); err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM metric_lineage_edges WHERE source_type='metric' AND source_id=$1 AND target_type='metric' AND target_id=$2`,
		dependsOn, metricID); err != nil {
		return fmt.Errorf("delete lineage edge: %w", err)
	}
	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType: "metric_dependency", EntityID: dependencyID, Action: "removed", ActorUserID: &actorID,
		Metadata: map[string]interface{}{"metric_id": metricID, "depends_on": dependsOn},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Service) ListVersions(ctx context.Context, metricID int) ([]MetricVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, metric_id, version_number, COALESCE(sql_expression,''), COALESCE(semantic_formula,''), COALESCE(time_grain,''), COALESCE(description,''), is_derived, COALESCE(window_calculation,''), status, created_at
		FROM metric_definition_versions WHERE metric_id=$1 ORDER BY version_number DESC`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []MetricVersion
	for rows.Next() {
		var v MetricVersion
		if err := rows.Scan(&v.ID, &v.MetricID, &v.VersionNumber, &v.SQLExpression, &v.SemanticFormula, &v.TimeGrain, &v.Description, &v.IsDerived, &v.WindowCalculation, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (s *Service) ListDimensions(ctx context.Context, metricID int) ([]Dimension, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, metric_id, name, COALESCE(description,'') FROM metric_dimensions WHERE metric_id=$1`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dims []Dimension
	for rows.Next() {
		var d Dimension
		if err := rows.Scan(&d.ID, &d.MetricID, &d.Name, &d.Description); err != nil {
			return nil, err
		}
		dims = append(dims, d)
	}
	return dims, nil
}

func (s *Service) GetLineage(ctx context.Context, metricID int) ([]LineageEdge, error) {
	rows, err := s.pool.Query(ctx, `SELECT le.id, le.source_type, le.source_id,
		CASE WHEN le.source_type='metric' THEN COALESCE((SELECT name FROM metric_definitions WHERE id=le.source_id),'')
		     ELSE COALESCE((SELECT name FROM charts WHERE id=le.source_id),'') END,
		le.target_type, le.target_id,
		CASE WHEN le.target_type='metric' THEN COALESCE((SELECT name FROM metric_definitions WHERE id=le.target_id),'')
		     ELSE COALESCE((SELECT name FROM charts WHERE id=le.target_id),'') END
		FROM metric_lineage_edges le
		WHERE (le.source_type='metric' AND le.source_id=$1) OR (le.target_type='metric' AND le.target_id=$1)`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []LineageEdge
	for rows.Next() {
		var e LineageEdge
		if err := rows.Scan(&e.ID, &e.SourceType, &e.SourceID, &e.SourceName, &e.TargetType, &e.TargetID, &e.TargetName); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, nil
}

func (s *Service) RunImpactAnalysis(ctx context.Context, metricID int) (*ImpactAnalysis, error) {
	// Count dependent metrics
	var depMetrics int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM metric_dependencies WHERE depends_on_metric=$1`, metricID).Scan(&depMetrics); err != nil {
		return nil, fmt.Errorf("count dependent metrics: %w", err)
	}

	// Count dependent charts
	var depCharts int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM chart_metric_dependencies WHERE metric_id=$1`, metricID).Scan(&depCharts); err != nil {
		return nil, fmt.Errorf("count dependent charts: %w", err)
	}

	// Check for missing dependencies this metric needs
	var missingDeps int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM metric_dependencies md
		WHERE md.metric_id=$1 AND NOT EXISTS (SELECT 1 FROM metric_definitions WHERE id=md.depends_on_metric AND status='active')`,
		metricID).Scan(&missingDeps); err != nil {
		return nil, fmt.Errorf("count missing deps: %w", err)
	}

	status := "approved"
	if missingDeps > 0 {
		status = "rejected"
	}

	analysis := &ImpactAnalysis{
		DependentMetrics: depMetrics,
		DependentCharts:  depCharts,
		MissingDeps:      missingDeps,
		Status:           status,
	}

	// Save review record
	var latestVersionID int
	if err := s.pool.QueryRow(ctx, `SELECT id FROM metric_definition_versions WHERE metric_id=$1 ORDER BY version_number DESC LIMIT 1`, metricID).Scan(&latestVersionID); err != nil {
		return nil, fmt.Errorf("get latest version: %w", err)
	}

	summary, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal summary: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO metric_activation_reviews (metric_id, version_id, impact_summary, status) VALUES ($1,$2,$3,$4)`,
		metricID, latestVersionID, summary, status); err != nil {
		return nil, fmt.Errorf("save review: %w", err)
	}

	return analysis, nil
}

// CheckMetricPermission verifies that a user has access to a specific metric.
// Returns nil if access is allowed. Checks metric_permissions table first;
// if no rows exist for this metric, access falls through to role-level perms.
func (s *Service) CheckMetricPermission(ctx context.Context, metricID, userID int, needActivate bool) error {
	// Check if metric-level permissions exist for this metric
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM metric_permissions WHERE metric_id=$1`, metricID).Scan(&count); err != nil {
		return fmt.Errorf("check metric permissions: %w", err)
	}
	if count == 0 {
		return nil // no metric-level restrictions, fall through to role permission
	}

	// Check user-level permission
	query := `SELECT COUNT(*) FROM metric_permissions WHERE metric_id=$1 AND user_id=$2 AND can_view=TRUE`
	if needActivate {
		query = `SELECT COUNT(*) FROM metric_permissions WHERE metric_id=$1 AND user_id=$2 AND can_activate=TRUE`
	}
	var allowed int
	if err := s.pool.QueryRow(ctx, query, metricID, userID).Scan(&allowed); err != nil {
		return fmt.Errorf("check user permission: %w", err)
	}
	if allowed > 0 {
		return nil
	}

	// Check role-level permission
	roleQuery := `SELECT COUNT(*) FROM metric_permissions mp
		JOIN user_roles ur ON ur.role_id = mp.role_id
		WHERE mp.metric_id=$1 AND ur.user_id=$2 AND mp.can_view=TRUE`
	if needActivate {
		roleQuery = `SELECT COUNT(*) FROM metric_permissions mp
			JOIN user_roles ur ON ur.role_id = mp.role_id
			WHERE mp.metric_id=$1 AND ur.user_id=$2 AND mp.can_activate=TRUE`
	}
	if err := s.pool.QueryRow(ctx, roleQuery, metricID, userID).Scan(&allowed); err != nil {
		return fmt.Errorf("check role permission: %w", err)
	}
	if allowed > 0 {
		return nil
	}

	return errors.New("insufficient metric-level permissions")
}

func (s *Service) ActivateMetric(ctx context.Context, metricID int, actorID int) error {
	// Enforce metric-level permission
	if err := s.CheckMetricPermission(ctx, metricID, actorID, true); err != nil {
		return err
	}

	// Get current state for audit before
	metric, err := s.GetMetric(ctx, metricID)
	if err != nil {
		return err
	}

	// Check last impact analysis was approved
	var reviewStatus string
	err = s.pool.QueryRow(ctx, `SELECT status FROM metric_activation_reviews WHERE metric_id=$1 ORDER BY created_at DESC LIMIT 1`, metricID).Scan(&reviewStatus)
	if err != nil || reviewStatus != "approved" {
		return ErrImpactAnalysisRequired
	}

	// Check dependency chain: all depended-on metrics must be active
	var missingDeps int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM metric_dependencies md
		WHERE md.metric_id=$1 AND NOT EXISTS (SELECT 1 FROM metric_definitions WHERE id=md.depends_on_metric AND status='active')`,
		metricID).Scan(&missingDeps); err != nil {
		return fmt.Errorf("check dependencies: %w", err)
	}
	if missingDeps > 0 {
		return ErrMissingDependencies
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE metric_definitions SET status='active', updated_at=NOW() WHERE id=$1`, metricID); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if err := s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "metric",
		EntityID:    metricID,
		Action:      "activated",
		ActorUserID: &actorID,
		Before:      map[string]string{"status": metric.Status},
		After:       map[string]string{"status": "active"},
	}); err != nil {
		return fmt.Errorf("write audit: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) GetLatestImpactAnalysis(ctx context.Context, metricID int) (*ImpactAnalysis, error) {
	var summary json.RawMessage
	var status string
	err := s.pool.QueryRow(ctx, `SELECT impact_summary, status FROM metric_activation_reviews WHERE metric_id=$1 ORDER BY created_at DESC LIMIT 1`, metricID).Scan(&summary, &status)
	if err != nil {
		return nil, err
	}

	var analysis ImpactAnalysis
	if err := json.Unmarshal(summary, &analysis); err != nil {
		return nil, fmt.Errorf("unmarshal impact summary: %w", err)
	}
	analysis.Status = status
	return &analysis, nil
}
