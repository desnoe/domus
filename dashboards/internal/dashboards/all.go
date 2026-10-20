package dashboards

import (
	"github.com/desnoe/homecode/dashboards/internal/builder"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

// All returns all 4 standard domus dashboard manifest builders
func All(cfg builder.Config) []*resource.ManifestBuilder {
	return []*resource.ManifestBuilder{
		HostDashboard(cfg),
		StorageDashboard(cfg),
		BackupsDashboard(cfg),
		AlloyDashboard(cfg),
	}
}
