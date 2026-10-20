package builder_test

import (
	"testing"

	"github.com/desnoe/homecode/dashboards/internal/builder"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"main", "main"},
		{"feat/my-feature", "feat-my-a1a30"},
		{"feat_test.123", "feat-test-123"},
		{"short", "short"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := builder.SanitizeSlug(tc.input)
			if got != tc.expected {
				t.Errorf("SanitizeSlug(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestConfigDerivations(t *testing.T) {
	prodCfg := builder.NewConfig(builder.EnvProd, "", "")
	if prodCfg.FolderUID != "domus" {
		t.Errorf("expected folder domus, got %s", prodCfg.FolderUID)
	}
	if prodCfg.DashboardUID("domus-host") != "domus-host" {
		t.Errorf("expected uid domus-host, got %s", prodCfg.DashboardUID("domus-host"))
	}
	if prodCfg.DashboardTitle("Host Overview") != "domus / Host Overview" {
		t.Errorf("unexpected title: %s", prodCfg.DashboardTitle("Host Overview"))
	}

	previewCfg := builder.NewConfig(builder.EnvPreview, "my-feature-branch", "")
	if len(previewCfg.DashboardUID("domus-storage")) > 40 {
		t.Errorf("UID exceeds 40 characters: %s", previewCfg.DashboardUID("domus-storage"))
	}

	prCfg := builder.NewConfig(builder.EnvPR, "42", "")
	if prCfg.FolderUID != "domus-pr-42" {
		t.Errorf("expected folder domus-pr-42, got %s", prCfg.FolderUID)
	}
}

func TestDatasourceReferences(t *testing.T) {
	prom, err := builder.PromDS().Build()
	if err != nil {
		t.Fatalf("failed to build PromDS: %v", err)
	}
	if prom.Name == nil || *prom.Name != builder.PrometheusDatasource {
		t.Errorf("expected datasource %s, got %+v", builder.PrometheusDatasource, prom.Name)
	}

	loki, err := builder.LokiDS().Build()
	if err != nil {
		t.Fatalf("failed to build LokiDS: %v", err)
	}
	if loki.Name == nil || *loki.Name != builder.LokiDatasource {
		t.Errorf("expected datasource %s, got %+v", builder.LokiDatasource, loki.Name)
	}
}

func TestHostVariable(t *testing.T) {
	v, err := builder.HostVariable().Build()
	if err != nil {
		t.Fatalf("failed to build HostVariable: %v", err)
	}

	if v.Spec.Name != "host" {
		t.Errorf("expected variable name 'host', got %q", v.Spec.Name)
	}
	if v.Spec.Multi {
		t.Errorf("multi-selection should be false")
	}
	if v.Spec.IncludeAll {
		t.Errorf("include-all should be false")
	}
	if v.Spec.Current.Value.String == nil || *v.Spec.Current.Value.String != "kodlix.delarche.fr" {
		t.Errorf("expected current value kodlix.delarche.fr, got %+v", v.Spec.Current.Value)
	}

	querySpec, ok := v.Spec.Query.Spec.(map[string]any)
	if !ok {
		t.Fatalf("expected Query.Spec to be map[string]any, got %T", v.Spec.Query.Spec)
	}
	if querySpec["query"] != `label_values(up{job="integrations/node_exporter"}, host)` {
		t.Errorf("unexpected query in spec: %v", querySpec["query"])
	}
	if querySpec["qryType"] != 1 {
		t.Errorf("expected qryType 1, got %v", querySpec["qryType"])
	}
}

func TestPanelBuilders(t *testing.T) {
	target := builder.PromInstantTarget(`up`, "Up")
	header := builder.HeaderStatPanel("Status", "short", target, nil)
	if _, err := header.Build(); err != nil {
		t.Fatalf("failed to build HeaderStatPanel: %v", err)
	}

	ts := builder.TimeseriesPanel("Network", "bps", builder.PromTarget(`rate(net[5m])`, "Net"))
	if _, err := ts.Build(); err != nil {
		t.Fatalf("failed to build TimeseriesPanel: %v", err)
	}

	stacked := builder.StackedTimeseriesPanel("CPU", "percent", builder.PromTarget(`rate(cpu[5m])`, "CPU"))
	if _, err := stacked.Build(); err != nil {
		t.Fatalf("failed to build StackedTimeseriesPanel: %v", err)
	}

	gauge := builder.GaugePanel("Gauge", "percent", 0, 100, target, nil)
	if _, err := gauge.Build(); err != nil {
		t.Fatalf("failed to build GaugePanel: %v", err)
	}

	bar := builder.BarGaugePanel("Bar", "percent", 0, 100, target, nil)
	if _, err := bar.Build(); err != nil {
		t.Fatalf("failed to build BarGaugePanel: %v", err)
	}

	logs := builder.LogsPanel("Logs", builder.LokiTarget(`{host="host"}`, 100))
	if _, err := logs.Build(); err != nil {
		t.Fatalf("failed to build LogsPanel: %v", err)
	}
}
