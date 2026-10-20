package dashboards

import (
	"github.com/desnoe/homecode/dashboards/internal/builder"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

// BackupsDashboard generates the domus-backups dashboard definition
func BackupsDashboard(cfg builder.Config) *resource.ManifestBuilder {
	dashUID := cfg.DashboardUID("domus-backups")
	dashTitle := cfg.DashboardTitle("Backup & Service Health")

	dash := dashboard.NewDashboardBuilder(dashTitle).
		Tags([]string{"domus", "backup", "rclone", "snapshot", "systemd"}).
		TimeSettings(
			dashboard.NewTimeSettingsBuilder().
				From("now-24h").
				To("now").
				AutoRefresh("1m"),
		).
		QueryVariable(builder.HostVariable())

	// 0. Header Synthesis (4 Key Indicators)
	dash.Panel("header-failed-services", builder.HeaderStatPanel(
		"Failed Services",
		"short",
		builder.PromInstantTarget(
			`count(node_systemd_unit_state{host="$host", state="failed", name=~"rclone-sync-.*|btrfs-snapshot-.*|btrfs-scrub-.*|smbd.*|alloy.*"} == 1) or vector(0)`,
			"Failed",
		),
		builder.PercentThresholds(1, 1),
	))

	dash.Panel("header-active-daemons", builder.HeaderStatPanel(
		"Active Daemons",
		"short",
		builder.PromInstantTarget(
			`count(node_systemd_unit_state{host="$host", state="active", name=~"smbd.*|alloy.*"} == 1) or vector(0)`,
			"Daemons",
		),
		nil,
	))

	dash.Panel("header-backup-execution", builder.HeaderStatPanelWithMappings(
		"Active Backup Execution",
		"short",
		builder.PromInstantTarget(
			`max(node_systemd_unit_state{host="$host", name=~"rclone-sync-.*|btrfs-snapshot-.*|btrfs-scrub-.*", state="active"}) or vector(0)`,
			"Backup",
		),
		builder.BackupExecutionValueMappings(),
		nil,
	))

	dash.Panel("header-journal-errors", builder.HeaderStatPanel(
		"Audit Log Errors (24h)",
		"short",
		builder.LokiInstantMetricTarget(
			`sum(count_over_time({host="$host", unit=~"rclone-sync.*|btrfs-snapshot.*|btrfs-scrub.*|smbd.*|alloy.*"} |~ "(?i)error" [24h])) or vector(0)`,
			"Errors",
		),
		builder.PercentThresholds(1, 5),
	))

	// 1. Monitored Services Status & Health
	dash.Panel("backup-services-table", builder.TablePanel(
		"Monitored Services Inventory",
		builder.PromTableTarget(
			`node_systemd_unit_state{host="$host", name=~"rclone-sync-.*|btrfs-snapshot-.*|btrfs-scrub-.*|smbd.*|alloy.*"} == 1`,
		),
	))

	dash.Panel("backup-services-failed", builder.MultiStatPanel(
		"Active Service Failures",
		"short",
		builder.PromInstantTarget(
			`node_systemd_unit_state{host="$host", state="failed", name=~"rclone-sync-.*|btrfs-snapshot-.*|btrfs-scrub-.*|smbd.*|alloy.*"} == 1`,
			"{{name}}",
		),
		builder.BinaryThresholds("green", "red", 1),
	))

	// 2. 24-Hour Execution History
	dash.Panel("backup-service-timeline", builder.StateTimelinePanel(
		"24-Hour Service Execution Timeline",
		builder.PromTarget(
			`node_systemd_unit_state{host="$host", name=~"rclone-sync-.*|btrfs-snapshot-.*|btrfs-scrub-.*|smbd.*|alloy.*", state="active"}`,
			"{{name}}",
		),
	))

	// 3. Loki Logs
	dash.Panel("backup-logs", builder.LogsPanel(
		"Backup & Scrub Journald Logs (rclone / snapshot / scrub / alloy / smbd)",
		builder.LokiTarget(
			`{host="$host", unit=~"rclone-sync.*|btrfs-snapshot.*|btrfs-scrub.*|smbd.*|alloy.*"}`,
			500,
		),
	))

	// Layout Structure: 4 Rows (Compact Header Row + 3 Detail Rows)
	dash.RowsLayout(
		dashboard.Rows().
			Row(builder.CompactHeaderRow(
				"Backup Key Metrics & Health Overview",
				"header-failed-services",
				"header-active-daemons",
				"header-backup-execution",
				"header-journal-errors",
			)).
			Row(dashboard.Row("Monitored Services Status & Health").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("backup-services-table").
					WithItem("backup-services-failed"),
			)).
			Row(dashboard.Row("Execution Timeline (24h)").AutoGridLayout(
				dashboard.AutoGrid().
					MaxColumnCount(1).
					WithItem("backup-service-timeline"),
			)).
			Row(builder.LogsRow("Audit Logs (Loki)", "backup-logs")),
	)

	return dashboard.Manifest(dashUID, dash).
		Metadata(
			resource.Named(dashUID).
				Annotations(map[string]string{
					"grafana.app/folder": cfg.FolderUID,
				}),
		)
}
