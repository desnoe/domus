package dashboards

import (
	"github.com/desnoe/homecode/dashboards/internal/builder"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

// StorageDashboard generates the domus-storage dashboard definition
func StorageDashboard(cfg builder.Config) *resource.ManifestBuilder {
	dashUID := cfg.DashboardUID("domus-storage")
	dashTitle := cfg.DashboardTitle("Storage & Hardware Health")

	dash := dashboard.NewDashboardBuilder(dashTitle).
		Tags([]string{"domus", "storage", "btrfs", "smart", "luks"}).
		TimeSettings(
			dashboard.NewTimeSettingsBuilder().
				From("now-1h").
				To("now").
				AutoRefresh("30s"),
		).
		QueryVariable(builder.HostVariable())

	// 0. Header Synthesis (4 Key Indicators)
	dash.Panel("header-smart-health", builder.HeaderStatPanelWithMappings(
		"S.M.A.R.T. Drive Health",
		"short",
		builder.PromInstantTarget(`min(smartctl_device_smart_status{host="$host"})`, "Health"),
		builder.SmartHeaderValueMappings(),
		builder.BinaryThresholds("red", "green", 1),
	))

	dash.Panel("header-critical-sectors", builder.HeaderStatPanel(
		"Critical Sectors",
		"short",
		builder.PromInstantTarget(
			`sum(smartctl_device_attribute{host="$host", attribute_name=~"(?i)reallocated_sector_ct|current_pending_sector|offline_uncorrectable", attribute_value_type="raw"}) or vector(0)`,
			"Sectors",
		),
		builder.PercentThresholds(1, 10),
	))

	dash.Panel("header-storage-capacity", builder.HeaderStatPanel(
		"Storage Capacity Used",
		"percent",
		builder.PromInstantTarget(
			`(1 - (sum(node_filesystem_avail_bytes{host="$host", mountpoint!~"/boot.*"}) / sum(node_filesystem_size_bytes{host="$host", mountpoint!~"/boot.*"}))) * 100`,
			"Capacity",
		),
		builder.PercentThresholds(80, 90),
	))

	dash.Panel("header-disk-io-rate", builder.HeaderStatPanel(
		"Total Disk I/O Rate",
		"Bps",
		builder.PromInstantTarget(
			`sum(rate(node_disk_read_bytes_total{host="$host"}[5m])) + sum(rate(node_disk_written_bytes_total{host="$host"}[5m]))`,
			"I/O Rate",
		),
		nil,
	))

	// 1. Physical Disks Throughput & IOPS (sd* / nvme*)
	dash.Panel("disk-phys-read", builder.TimeseriesPanel(
		"Physical Disks Read Throughput",
		"Bps",
		builder.PromTarget(`rate(node_disk_read_bytes_total{host="$host", device=~"sd.*|nvme.*"}[5m])`, "{{device}} Read"),
	))

	dash.Panel("disk-phys-write", builder.TimeseriesPanel(
		"Physical Disks Write Throughput",
		"Bps",
		builder.PromTarget(`rate(node_disk_written_bytes_total{host="$host", device=~"sd.*|nvme.*"}[5m])`, "{{device}} Write"),
	))

	dash.Panel("disk-phys-iops-read", builder.TimeseriesPanel(
		"Physical Disks Read IOPS",
		"iops",
		builder.PromTarget(`rate(node_disk_reads_completed_total{host="$host", device=~"sd.*|nvme.*"}[5m])`, "{{device}} Reads"),
	))

	dash.Panel("disk-phys-iops-write", builder.TimeseriesPanel(
		"Physical Disks Write IOPS",
		"iops",
		builder.PromTarget(`rate(node_disk_writes_completed_total{host="$host", device=~"sd.*|nvme.*"}[5m])`, "{{device}} Writes"),
	))

	dash.Panel("disk-phys-latency", builder.TimeseriesPanel(
		"Physical Disks Average I/O Latency",
		"s",
		builder.PromTarget(`rate(node_disk_read_time_seconds_total{host="$host", device=~"sd.*|nvme.*"}[5m]) / clamp_min(rate(node_disk_reads_completed_total{host="$host", device=~"sd.*|nvme.*"}[5m]), 0.001)`, "{{device}} Read Latency"),
		builder.PromTarget(`rate(node_disk_write_time_seconds_total{host="$host", device=~"sd.*|nvme.*"}[5m]) / clamp_min(rate(node_disk_writes_completed_total{host="$host", device=~"sd.*|nvme.*"}[5m]), 0.001)`, "{{device}} Write Latency"),
	))

	dash.Panel("disk-phys-util", builder.TimeseriesPanel(
		"Physical Disks I/O Utilization",
		"percent",
		builder.PromTarget(`rate(node_disk_io_time_seconds_total{host="$host", device=~"sd.*|nvme.*"}[5m]) * 100`, "{{device}}"),
	))

	// 2. LUKS dm-crypt Mappers Throughput & IOPS (dm-*)
	dash.Panel("disk-luks-read", builder.TimeseriesPanel(
		"LUKS Mappers Read Throughput",
		"Bps",
		builder.PromTarget(`rate(node_disk_read_bytes_total{host="$host", device=~"dm-.*"}[5m])`, "{{device}} Read"),
	))

	dash.Panel("disk-luks-write", builder.TimeseriesPanel(
		"LUKS Mappers Write Throughput",
		"Bps",
		builder.PromTarget(`rate(node_disk_written_bytes_total{host="$host", device=~"dm-.*"}[5m])`, "{{device}} Write"),
	))

	dash.Panel("disk-luks-iops-read", builder.TimeseriesPanel(
		"LUKS Mappers Read IOPS",
		"iops",
		builder.PromTarget(`rate(node_disk_reads_completed_total{host="$host", device=~"dm-.*"}[5m])`, "{{device}} Reads"),
	))

	dash.Panel("disk-luks-iops-write", builder.TimeseriesPanel(
		"LUKS Mappers Write IOPS",
		"iops",
		builder.PromTarget(`rate(node_disk_writes_completed_total{host="$host", device=~"dm-.*"}[5m])`, "{{device}} Writes"),
	))

	dash.Panel("disk-luks-latency", builder.TimeseriesPanel(
		"LUKS Mappers Average I/O Latency",
		"s",
		builder.PromTarget(`rate(node_disk_read_time_seconds_total{host="$host", device=~"dm-.*"}[5m]) / clamp_min(rate(node_disk_reads_completed_total{host="$host", device=~"dm-.*"}[5m]), 0.001)`, "{{device}} Read Latency"),
		builder.PromTarget(`rate(node_disk_write_time_seconds_total{host="$host", device=~"dm-.*"}[5m]) / clamp_min(rate(node_disk_writes_completed_total{host="$host", device=~"dm-.*"}[5m]), 0.001)`, "{{device}} Write Latency"),
	))

	// 3. Btrfs Filesystems & Subvolumes
	dash.Panel("fs-usage-bar", builder.BarGaugePanel(
		"Filesystem & Subvolume Capacity Used (%)",
		"percent",
		0, 100,
		builder.PromInstantTarget(
			`(1 - (node_filesystem_avail_bytes{host="$host", mountpoint!~"/boot.*"} / node_filesystem_size_bytes{host="$host", mountpoint!~"/boot.*"})) * 100`,
			"{{mountpoint}} ({{fstype}})",
		),
		builder.PercentThresholds(80, 90),
	))

	dash.Panel("fs-space-used", builder.TimeseriesPanel(
		"Filesystem Space Used",
		"bytes",
		builder.PromTarget(`node_filesystem_size_bytes{host="$host", mountpoint!~"/boot.*"} - node_filesystem_avail_bytes{host="$host", mountpoint!~"/boot.*"}`, "{{mountpoint}} Used"),
	))

	dash.Panel("fs-space-avail", builder.TimeseriesPanel(
		"Filesystem Space Free / Available",
		"bytes",
		builder.PromTarget(`node_filesystem_avail_bytes{host="$host", mountpoint!~"/boot.*"}`, "{{mountpoint}} Avail"),
	))

	dash.Panel("fs-health-stat", builder.MultiStatPanelWithMappings(
		"Filesystem Health & Read-Only Status",
		"short",
		builder.PromInstantTarget(`node_filesystem_readonly{host="$host", mountpoint!~"/boot.*"}`, "{{mountpoint}}"),
		builder.FilesystemStatusValueMappings(),
		builder.BinaryThresholds("green", "red", 1),
	))

	dash.Panel("fs-device-error", builder.MultiStatPanel(
		"Filesystem Underlying Block Device Errors",
		"short",
		builder.PromInstantTarget(`node_filesystem_device_error{host="$host", mountpoint!~"/boot.*"}`, "{{mountpoint}}"),
		builder.BinaryThresholds("green", "red", 1),
	))

	dash.Panel("fs-inode-usage", builder.BarGaugePanel(
		"Filesystem Inode Table Utilization (%)",
		"percent",
		0, 100,
		builder.PromInstantTarget(
			`(1 - (node_filesystem_files_free{host="$host", mountpoint!~"/boot.*"} / node_filesystem_files{host="$host", mountpoint!~"/boot.*"})) * 100`,
			"{{mountpoint}} ({{fstype}})",
		),
		builder.PercentThresholds(80, 90),
	))

	dash.Panel("btrfs-device-usage", builder.TimeseriesPanel(
		"Btrfs Pool Member Disks Physical Size",
		"bytes",
		builder.PromTarget(`node_btrfs_device_size_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} - Device: {{device}}"),
	))

	dash.Panel("btrfs-device-unused", builder.TimeseriesPanel(
		"Btrfs Pool Member Disks Unallocated Physical Space",
		"bytes",
		builder.PromTarget(`node_btrfs_device_unused_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} - Device: {{device}} Unallocated"),
	))

	dash.Panel("btrfs-device-errors", builder.TimeseriesPanel(
		"Btrfs Pool Member Disk Errors",
		"short",
		builder.PromTarget(`node_btrfs_device_errors_total{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} - Device: {{device}} [{{type}}]"),
	))

	dash.Panel("btrfs-chunks", builder.TimeseriesPanel(
		"Btrfs Block Group Chunk Allocation",
		"bytes",
		builder.PromTarget(`node_btrfs_used_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} ({{mode}}) - {{block_group_type}} Used"),
		builder.PromTarget(`node_btrfs_size_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} ({{mode}}) - {{block_group_type}} Allocated"),
		builder.PromTarget(`node_btrfs_reserved_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} ({{mode}}) - {{block_group_type}} Reserved"),
	))

	dash.Panel("btrfs-global-rsv", builder.TimeseriesPanel(
		"Btrfs Global Reserve Size",
		"bytes",
		builder.PromTarget(`node_btrfs_global_rsv_size_bytes{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} Global Reserve"),
	))

	dash.Panel("btrfs-alloc-ratio", builder.TimeseriesPanel(
		"Btrfs Redundancy Allocation Factor",
		"short",
		builder.PromTarget(`node_btrfs_allocation_ratio{host="$host"} * on(uuid) group_left(label) node_btrfs_info{host="$host"}`, "Pool: {{label}} ({{mode}}) - {{block_group_type}}"),
	))

	// 4. S.M.A.R.T. Drive Health & Telemetry
	dash.Panel("smart-status", builder.MultiStatPanelWithMappings(
		"S.M.A.R.T. Drive Health Status",
		"short",
		builder.PromInstantTarget(`smartctl_device_smart_status{host="$host"}`, "{{device}}"),
		builder.SmartStatusValueMappings(),
		builder.BinaryThresholds("red", "green", 1),
	))

	dash.Panel("smart-exit-status", builder.MultiStatPanelWithMappings(
		"S.M.A.R.T. Command Execution Status",
		"short",
		builder.PromInstantTarget(`smartctl_device_smartctl_exit_status{host="$host"}`, "{{device}}"),
		builder.SmartExitStatusValueMappings(),
		builder.BinaryThresholds("green", "red", 1),
	))

	dash.Panel("smart-error-logs", builder.MultiStatPanel(
		"S.M.A.R.T. ATA Error Log Entries",
		"short",
		builder.PromInstantTarget(
			`max by (device) (smartctl_device_error_log_count{host="$host"} or (0 * smartctl_device_capacity_bytes{host="$host"}))`,
			"{{device}}",
		),
		builder.CountThresholds(1, 5),
	))

	dash.Panel("smart-rotation-rate", builder.MultiStatPanelWithMappings(
		"Drive Media Type & Speed",
		"short",
		builder.PromInstantTarget(
			`max by (device) (smartctl_device_rotation_rate{host="$host"} or (0 * smartctl_device_capacity_bytes{host="$host"}))`,
			"{{device}}",
		),
		builder.DriveRotationRateValueMappings(),
		nil,
	))

	dash.Panel("smart-temp", builder.TimeseriesPanel(
		"Drive Operating Temperature",
		"celsius",
		builder.PromTarget(`smartctl_device_temperature{host="$host"}`, "{{device}}"),
	))

	dash.Panel("smart-capacity", builder.MultiStatPanel(
		"Drive Hardware Capacity",
		"bytes",
		builder.PromInstantTarget(`smartctl_device_capacity_bytes{host="$host"}`, "{{device}}"),
		nil,
	))

	dash.Panel("smart-power-on", builder.MultiStatPanel(
		"Drive Power-On Hours",
		"h",
		builder.PromInstantTarget(`smartctl_device_power_on_seconds{host="$host"} / 3600`, "{{device}}"),
		nil,
	))

	dash.Panel("smart-power-cycles", builder.MultiStatPanel(
		"Drive Power Cycles Count",
		"short",
		builder.PromInstantTarget(`smartctl_device_power_cycle_count{host="$host"}`, "{{device}}"),
		nil,
	))

	dash.Panel("smart-sectors-reallocated", builder.TimeseriesPanelWithOverrides(
		"Reallocated Sectors (ID 5)",
		"short",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`smartctl_device_attribute{host="$host", attribute_name=~"(?i)reallocated_sector_ct", attribute_value_type="raw"}`, "{{device}} Reallocated"),
	))

	dash.Panel("smart-sectors-pending", builder.TimeseriesPanelWithOverrides(
		"Current Pending Sectors (ID 197)",
		"short",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`smartctl_device_attribute{host="$host", attribute_name=~"(?i)current_pending_sector", attribute_value_type="raw"}`, "{{device}} Pending"),
	))

	dash.Panel("smart-sectors-uncorrectable", builder.TimeseriesPanelWithOverrides(
		"Offline Uncorrectable Sectors (ID 198)",
		"short",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`smartctl_device_attribute{host="$host", attribute_name=~"(?i)offline_uncorrectable|uncorrectable_error_cnt", attribute_value_type="raw"}`, "{{device}} Uncorrectable"),
	))

	dash.Panel("smart-sectors-crc", builder.TimeseriesPanelWithOverrides(
		"Interface CRC Error Count (ID 199 / UDMA)",
		"short",
		map[string]string{
			".*": "semi-dark-orange",
		},
		builder.PromTarget(`smartctl_device_attribute{host="$host", attribute_name=~"(?i).*crc.*", attribute_value_type="raw"}`, "{{device}} CRC Errors"),
	))

	dash.Panel("smart-logs", builder.LogsPanel(
		"S.M.A.R.T. Exporter Hardware Logs (Loki)",
		builder.LokiTarget(
			`{host="$host", unit="smartctl_exporter.service"}`,
			500,
		),
	))

	// Layout Structure: 5 Rows (Compact Header Row + 4 Detail Rows)
	dash.RowsLayout(
		dashboard.Rows().
			Row(builder.CompactHeaderRow(
				"Storage Key Metrics & Health Overview",
				"header-smart-health",
				"header-critical-sectors",
				"header-storage-capacity",
				"header-disk-io-rate",
			)).
			Row(dashboard.Row("Physical Disks Throughput & IOPS (sd* / nvme*)").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("disk-phys-read").
					WithItem("disk-phys-write").
					WithItem("disk-phys-iops-read").
					WithItem("disk-phys-iops-write").
					WithItem("disk-phys-latency").
					WithItem("disk-phys-util"),
			)).
			Row(dashboard.Row("LUKS dm-crypt Mappers Throughput & IOPS (dm-*)").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("disk-luks-read").
					WithItem("disk-luks-write").
					WithItem("disk-luks-iops-read").
					WithItem("disk-luks-iops-write").
					WithItem("disk-luks-latency"),
			)).
			Row(dashboard.Row("Btrfs Filesystems & Subvolumes").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("fs-usage-bar").
					WithItem("fs-space-used").
					WithItem("fs-space-avail").
					WithItem("fs-health-stat").
					WithItem("fs-device-error").
					WithItem("fs-inode-usage").
					WithItem("btrfs-device-usage").
					WithItem("btrfs-device-unused").
					WithItem("btrfs-device-errors").
					WithItem("btrfs-chunks").
					WithItem("btrfs-global-rsv").
					WithItem("btrfs-alloc-ratio"),
			)).
			Row(dashboard.Row("S.M.A.R.T. Drive Health & Telemetry").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("smart-status").
					WithItem("smart-exit-status").
					WithItem("smart-error-logs").
					WithItem("smart-rotation-rate").
					WithItem("smart-temp").
					WithItem("smart-capacity").
					WithItem("smart-power-on").
					WithItem("smart-power-cycles").
					WithItem("smart-sectors-reallocated").
					WithItem("smart-sectors-pending").
					WithItem("smart-sectors-uncorrectable").
					WithItem("smart-sectors-crc"),
			)).
			Row(builder.LogsRow(
				"Hardware & Storage Audit Logs (Loki)",
				"smart-logs",
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
