package dashboards

import (
	"github.com/desnoe/homecode/dashboards/internal/builder"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

// AlloyDashboard generates the domus-alloy-health dashboard definition
func AlloyDashboard(cfg builder.Config) *resource.ManifestBuilder {
	dashUID := cfg.DashboardUID("domus-alloy-health")
	dashTitle := cfg.DashboardTitle("Alloy Telemetry Health")

	dash := dashboard.NewDashboardBuilder(dashTitle).
		Tags([]string{"domus", "alloy", "telemetry", "prometheus", "loki"}).
		TimeSettings(
			dashboard.NewTimeSettingsBuilder().
				From("now-1h").
				To("now").
				AutoRefresh("30s"),
		).
		QueryVariable(builder.HostVariable())

	// 0. Header Synthesis (4 Key Indicators)
	dash.Panel("header-alloy-up", builder.HeaderStatPanelWithMappings(
		"Alloy Agent Status",
		"short",
		builder.PromInstantTarget(`up{job="integrations/alloy", host="$host"}`, "Status"),
		builder.ServiceStatusValueMappings(),
		builder.BinaryThresholds("red", "green", 1),
	))

	dash.Panel("header-alloy-mem", builder.HeaderStatPanel(
		"Alloy Memory (RSS)",
		"bytes",
		builder.PromInstantTarget(`process_resident_memory_bytes{job="integrations/alloy", host="$host"}`, "Memory"),
		builder.PercentThresholds(268435456, 536870912),
	))

	dash.Panel("header-alloy-sent-rate", builder.HeaderStatPanel(
		"Samples Sent Rate",
		"ops",
		builder.PromInstantTarget(`(rate(prometheus_remote_storage_samples_total{job="integrations/alloy", host="$host"}[5m]) or rate(prometheus_remote_write_sent_samples_total{job="integrations/alloy", host="$host"}[5m]))`, "Sent/s"),
		nil,
	))

	dash.Panel("header-alloy-drops", builder.HeaderStatPanel(
		"Dropped Samples / Logs",
		"ops",
		builder.PromInstantTarget(
			`(sum(rate(prometheus_remote_storage_samples_failed_total{job="integrations/alloy", host="$host"}[5m])) or vector(0)) + (sum(rate(loki_write_dropped_entries_total{job="integrations/alloy", host="$host"}[5m])) or vector(0))`,
			"Drops",
		),
		builder.BinaryThresholds("green", "red", 0.001),
	))

	// 1. Process Resources
	dash.Panel("alloy-cpu", builder.TimeseriesPanel(
		"Alloy Process CPU Usage",
		"percent",
		builder.PromTarget(`rate(process_cpu_seconds_total{job="integrations/alloy", host="$host"}[5m]) * 100`, "CPU"),
	))

	dash.Panel("alloy-mem", builder.TimeseriesPanel(
		"Alloy Resident Memory (RSS)",
		"bytes",
		builder.PromTarget(`process_resident_memory_bytes{job="integrations/alloy", host="$host"}`, "Resident Memory"),
	))

	dash.Panel("alloy-goroutines", builder.TimeseriesPanel(
		"Active Go Goroutines",
		"short",
		builder.PromTarget(`go_goroutines{job="integrations/alloy", host="$host"}`, "Goroutines"),
	))

	dash.Panel("alloy-up", builder.StatPanelWithMappings(
		"Alloy Scrape Target Status",
		"short",
		builder.PromInstantTarget(`up{job="integrations/alloy", host="$host"}`, "{{instance}}"),
		builder.ServiceStatusValueMappings(),
		builder.BinaryThresholds("red", "green", 1),
	))

	// 2. Prometheus Pipeline
	dash.Panel("alloy-prom-samples", builder.TimeseriesPanel(
		"Remote-Write Samples Sent Rate",
		"ops",
		builder.PromTarget(`rate(prometheus_remote_storage_samples_total{job="integrations/alloy", host="$host"}[5m]) or rate(prometheus_remote_write_sent_samples_total{job="integrations/alloy", host="$host"}[5m])`, "Samples/sec"),
	))

	dash.Panel("alloy-prom-bytes", builder.TimeseriesPanel(
		"Remote-Write Data Throughput",
		"binBps",
		builder.PromTarget(`rate(prometheus_remote_storage_bytes_total{job="integrations/alloy", host="$host"}[5m])`, "Throughput"),
	))

	dash.Panel("alloy-prom-shards", builder.TimeseriesPanel(
		"Active Remote-Write Shards",
		"short",
		builder.PromTarget(`prometheus_remote_storage_shards{job="integrations/alloy", host="$host"}`, "Shards"),
	))

	dash.Panel("alloy-prom-dropped", builder.TimeseriesPanelWithOverrides(
		"Remote-Write Samples Dropped",
		"ops",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`rate(prometheus_remote_storage_samples_failed_total{job="integrations/alloy", host="$host"}[5m]) or vector(0)`, "Dropped/sec"),
	))

	dash.Panel("alloy-prom-retries", builder.TimeseriesPanelWithOverrides(
		"Remote-Write Retries",
		"ops",
		map[string]string{
			".*": "semi-dark-orange",
		},
		builder.PromTarget(`rate(prometheus_remote_storage_samples_retried_total{job="integrations/alloy", host="$host"}[5m]) or vector(0)`, "Retries/sec"),
	))

	// 3. Loki Pipeline
	dash.Panel("alloy-loki-entries", builder.TimeseriesPanel(
		"Loki Log Entries Shipped Rate",
		"ops",
		builder.PromTarget(`rate(loki_write_sent_entries_total{job="integrations/alloy", host="$host"}[5m])`, "Entries/sec"),
	))

	dash.Panel("alloy-loki-retries", builder.TimeseriesPanelWithOverrides(
		"Loki Batch Retries",
		"ops",
		map[string]string{
			".*": "semi-dark-orange",
		},
		builder.PromTarget(`rate(loki_write_batch_retries_total{job="integrations/alloy", host="$host"}[5m]) or vector(0)`, "Retries/sec"),
	))

	dash.Panel("alloy-loki-dropped", builder.TimeseriesPanelWithOverrides(
		"Loki Log Entries Dropped",
		"ops",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`rate(loki_write_dropped_entries_total{job="integrations/alloy", host="$host"}[5m])`, "Dropped/sec"),
	))

	// Layout Structure: 4 Rows (Compact Header Row + 3 Detail Rows)
	dash.RowsLayout(
		dashboard.Rows().
			Row(builder.CompactHeaderRow(
				"Alloy Key Metrics & Health Overview",
				"header-alloy-up",
				"header-alloy-mem",
				"header-alloy-sent-rate",
				"header-alloy-drops",
			)).
			Row(dashboard.Row("Alloy Process Resources").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("alloy-up").
					WithItem("alloy-cpu").
					WithItem("alloy-mem").
					WithItem("alloy-goroutines"),
			)).
			Row(dashboard.Row("Prometheus Remote-Write Pipeline").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("alloy-prom-samples").
					WithItem("alloy-prom-bytes").
					WithItem("alloy-prom-shards").
					WithItem("alloy-prom-dropped").
					WithItem("alloy-prom-retries"),
			)).
			Row(dashboard.Row("Loki Log Push Pipeline").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("alloy-loki-entries").
					WithItem("alloy-loki-retries").
					WithItem("alloy-loki-dropped"),
			)),
	)

	return dashboard.Manifest(dashUID, dash).
		Metadata(
			resource.Named(dashUID).
				Annotations(map[string]string{
					"grafana.app/folder": cfg.FolderUID,
				}),
		)
}
