package builder

import (
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
)

const (
	// PrometheusDatasource identifies the primary Prometheus instance on Grafana Cloud
	PrometheusDatasource = "grafanacloud-prom"
	// LokiDatasource identifies the primary Loki instance on Grafana Cloud
	LokiDatasource = "grafanacloud-logs"
)

// PromDS returns a Prometheus datasource reference builder
func PromDS() *dashboard.Dashboardv2DataQueryKindDatasourceBuilder {
	return dashboard.NewDashboardv2DataQueryKindDatasourceBuilder().Name(PrometheusDatasource)
}

// LokiDS returns a Loki datasource reference builder
func LokiDS() *dashboard.Dashboardv2DataQueryKindDatasourceBuilder {
	return dashboard.NewDashboardv2DataQueryKindDatasourceBuilder().Name(LokiDatasource)
}
