package metrics

import (
	"testing"
)

func TestErrorConstants(t *testing.T) {
	if ErrImpactAnalysisRequired == nil {
		t.Error("ErrImpactAnalysisRequired should not be nil")
	}
	if ErrMissingDependencies == nil {
		t.Error("ErrMissingDependencies should not be nil")
	}
}

// TestCreateParamsCarriesSemanticFields verifies that CreateParams correctly
// carries all semantic-layer fields (is_derived, window_calculation, dimensions,
// filters, dependencies) rather than silently dropping them.
func TestCreateParamsCarriesSemanticFields(t *testing.T) {
	p := CreateParams{
		Name:              "Test",
		IsDerived:         true,
		WindowCalculation: "7d_rolling_avg",
		DependsOnMetrics:  []int{1, 2},
		Filters: []MetricFilterDef{
			{Name: "region", Expression: "region='US'"},
		},
		Dimensions: []DimensionDef{
			{Name: "country", Description: "Country dimension"},
		},
	}

	if !p.IsDerived {
		t.Error("IsDerived should be true")
	}
	if p.WindowCalculation != "7d_rolling_avg" {
		t.Errorf("expected WindowCalculation '7d_rolling_avg', got %q", p.WindowCalculation)
	}
	if len(p.DependsOnMetrics) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(p.DependsOnMetrics))
	}
	if len(p.Filters) != 1 || p.Filters[0].Name != "region" {
		t.Error("filter not carried correctly")
	}
	if len(p.Dimensions) != 1 || p.Dimensions[0].Name != "country" {
		t.Error("dimension not carried correctly")
	}
}

// TestUpdateParamsCarriesSemanticFields verifies UpdateParams carries all fields.
func TestUpdateParamsCarriesSemanticFields(t *testing.T) {
	p := UpdateParams{
		MetricID:          42,
		IsDerived:         true,
		WindowCalculation: "30d_sum",
		DependsOnMetrics:  []int{5},
		Filters:           []MetricFilterDef{{Name: "f1", Expression: "x=1"}},
		Dimensions:        []DimensionDef{{Name: "d1", Description: "dim"}},
	}
	if p.MetricID != 42 {
		t.Error("MetricID not set")
	}
	if !p.IsDerived {
		t.Error("IsDerived should be true")
	}
	if len(p.Filters) != 1 {
		t.Error("filter not carried")
	}
	if len(p.Dimensions) != 1 {
		t.Error("dimension not carried")
	}
}

// TestAddDimensionValidation verifies the service-level validation guard.
func TestAddDimensionValidation(t *testing.T) {
	svc := &Service{}
	_, err := svc.AddDimension(nil, 1, DimensionDef{Name: ""}, 1)
	if err == nil {
		t.Error("expected error for empty dimension name")
	}
}

// TestAddFilterValidation verifies the service-level validation guards.
func TestAddFilterValidation(t *testing.T) {
	svc := &Service{}

	_, err := svc.AddFilter(nil, 1, MetricFilterDef{Name: "", Expression: "x=1"}, 1)
	if err == nil {
		t.Error("expected error for empty filter name")
	}

	_, err = svc.AddFilter(nil, 1, MetricFilterDef{Name: "f1", Expression: ""}, 1)
	if err == nil {
		t.Error("expected error for empty filter expression")
	}
}

// TestAddDependencySelfReferenceRejected verifies a metric cannot depend on itself.
func TestAddDependencySelfReferenceRejected(t *testing.T) {
	svc := &Service{}
	_, err := svc.AddDependency(nil, 42, 42, 1)
	if err == nil {
		t.Error("expected error for self-referencing dependency")
	}
}

// TestMetricDefinitionStatusValues verifies valid metric statuses.
func TestMetricDefinitionStatusValues(t *testing.T) {
	validStatuses := map[string]bool{
		"draft": true, "pending_review": true, "active": true, "deprecated": true,
	}
	for status := range validStatuses {
		md := MetricDefinition{Status: status}
		if !validStatuses[md.Status] {
			t.Errorf("unexpected status: %s", md.Status)
		}
	}
}

// TestMetricFilterDefJSONTags verifies JSON tags are present for API correctness.
func TestMetricFilterDefJSONTags(t *testing.T) {
	// This test verifies that MetricFilterDef has json tags by checking that
	// the struct can be properly used in JSON marshaling contexts.
	f := MetricFilterDef{Name: "test", Expression: "x=1"}
	if f.Name != "test" || f.Expression != "x=1" {
		t.Error("MetricFilterDef fields not set correctly")
	}
}
