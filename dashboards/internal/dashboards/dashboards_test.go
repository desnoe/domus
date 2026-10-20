package dashboards_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/desnoe/homecode/dashboards/internal/builder"
	"github.com/desnoe/homecode/dashboards/internal/dashboards"
)

func TestAllDashboardsBuild(t *testing.T) {
	configs := []builder.Config{
		builder.NewConfig(builder.EnvProd, "", ""),
		builder.NewConfig(builder.EnvPreview, "test-branch", ""),
		builder.NewConfig(builder.EnvPR, "42", ""),
	}

	for _, cfg := range configs {
		t.Run(string(cfg.Env), func(t *testing.T) {
			manifestBuilders := dashboards.All(cfg)
			if len(manifestBuilders) != 4 {
				t.Fatalf("expected 4 dashboards, got %d", len(manifestBuilders))
			}

			for _, mb := range manifestBuilders {
				manifest, err := mb.Build()
				if err != nil {
					t.Fatalf("failed to build dashboard manifest: %v", err)
				}

				if manifest.Metadata.Name == "" {
					t.Errorf("manifest metadata name is empty")
				}

				folder := manifest.Metadata.Annotations["grafana.app/folder"]
				if folder != cfg.FolderUID {
					t.Errorf("expected folder %q, got %q", cfg.FolderUID, folder)
				}

				data, err := json.Marshal(manifest)
				if err != nil {
					t.Fatalf("failed to marshal manifest to JSON: %v", err)
				}
				if len(data) == 0 {
					t.Fatalf("marshaled manifest is empty")
				}
				if !strings.Contains(string(data), "header-") {
					t.Errorf("manifest %s does not contain synthesis header panels", manifest.Metadata.Name)
				}
			}
		})
	}
}
