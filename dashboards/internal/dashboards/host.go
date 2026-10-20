package dashboards

import (
	"github.com/desnoe/homecode/dashboards/internal/builder"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

// HostDashboard generates the domus-host dashboard definition
func HostDashboard(cfg builder.Config) *resource.ManifestBuilder {
	dashUID := cfg.DashboardUID("domus-host")
	dashTitle := cfg.DashboardTitle("Host Overview")

	dash := dashboard.NewDashboardBuilder(dashTitle).
		Tags([]string{"domus", "host", "linux"}).
		TimeSettings(
			dashboard.NewTimeSettingsBuilder().
				From("now-1h").
				To("now").
				AutoRefresh("30s"),
		).
		QueryVariable(builder.HostVariable())

	// 0. Header Synthesis (4 Key Indicators)
	dash.Panel("header-uptime", builder.HeaderStatPanel(
		"Uptime",
		"s",
		builder.PromInstantTarget(`time() - node_boot_time_seconds{host="$host"}`, "Uptime"),
		nil,
	))

	dash.Panel("header-cpu", builder.HeaderStatPanel(
		"CPU Busy",
		"percent",
		builder.PromInstantTarget(`100 - (avg(rate(node_cpu_seconds_total{host="$host", mode="idle"}[5m])) * 100)`, "CPU"),
		builder.PercentThresholds(75, 90),
	))

	dash.Panel("header-mem", builder.HeaderStatPanel(
		"Memory Used",
		"percent",
		builder.PromInstantTarget(`(1 - (node_memory_MemAvailable_bytes{host="$host"} / node_memory_MemTotal_bytes{host="$host"})) * 100`, "Memory"),
		builder.PercentThresholds(80, 95),
	))

	dash.Panel("header-load", builder.HeaderStatPanel(
		"Load (1m)",
		"short",
		builder.PromInstantTarget(`node_load1{host="$host"}`, "Load 1m"),
		nil,
	))

	// 1. CPU & Load
	dash.Panel("cpu-gauge", builder.GaugePanel(
		"CPU Utilization",
		"percent",
		0, 100,
		builder.PromInstantTarget(
			`100 - (avg(rate(node_cpu_seconds_total{host="$host", mode="idle"}[5m])) * 100)`,
			"CPU Busy",
		),
		builder.PercentThresholds(75, 90),
	))

	dash.Panel("cpu-modes", builder.StackedTimeseriesPanel(
		"CPU Usage by Mode",
		"percent",
		builder.PromTarget(
			`avg by (mode) (rate(node_cpu_seconds_total{host="$host", mode!="idle"}[5m])) * 100`,
			"{{mode}}",
		),
	))

	dash.Panel("load-avg", builder.TimeseriesPanel(
		"System Load Average",
		"short",
		builder.PromTarget(`node_load1{host="$host"}`, "1m load"),
		builder.PromTarget(`node_load5{host="$host"}`, "5m load"),
		builder.PromTarget(`node_load15{host="$host"}`, "15m load"),
	))

	dash.Panel("cpu-context-switches", builder.TimeseriesPanel(
		"Context Switches & Interrupts",
		"ops",
		builder.PromTarget(`rate(node_context_switches_total{host="$host"}[5m])`, "Context Switches/s"),
		builder.PromTarget(`rate(node_intr_total{host="$host"}[5m])`, "Interrupts/s"),
	))

	dash.Panel("procs-state", builder.TimeseriesPanel(
		"System Processes (Running vs Blocked on I/O)",
		"short",
		builder.PromTarget(`node_procs_running{host="$host"} or vector(0)`, "Running (Run Queue)"),
		builder.PromTarget(`node_procs_blocked{host="$host"} or vector(0)`, "Blocked (Uninterruptible I/O)"),
	))

	// 2. Memory & Swap
	dash.Panel("mem-gauge", builder.GaugePanel(
		"Memory Utilization",
		"percent",
		0, 100,
		builder.PromInstantTarget(
			`(1 - (node_memory_MemAvailable_bytes{host="$host"} / node_memory_MemTotal_bytes{host="$host"})) * 100`,
			"Memory Used",
		),
		builder.PercentThresholds(80, 95),
	))

	dash.Panel("mem-breakdown", builder.StackedTimeseriesPanel(
		"Memory Distribution",
		"bytes",
		builder.PromTarget(`node_memory_MemTotal_bytes{host="$host"} - node_memory_MemAvailable_bytes{host="$host"}`, "Used"),
		builder.PromTarget(`node_memory_Buffers_bytes{host="$host"}`, "Buffers"),
		builder.PromTarget(`node_memory_Cached_bytes{host="$host"}`, "Cached"),
		builder.PromTarget(`node_memory_MemFree_bytes{host="$host"}`, "Free"),
	))

	dash.Panel("mem-dirty", builder.TimeseriesPanel(
		"Dirty & Writeback Memory",
		"bytes",
		builder.PromTarget(`node_memory_Dirty_bytes{host="$host"}`, "Dirty"),
		builder.PromTarget(`node_memory_Writeback_bytes{host="$host"}`, "Writeback"),
	))

	dash.Panel("swap-usage", builder.TimeseriesPanel(
		"Swap Usage",
		"bytes",
		builder.PromTarget(`node_memory_SwapTotal_bytes{host="$host"} - node_memory_SwapFree_bytes{host="$host"}`, "Swap Used"),
		builder.PromTarget(`node_memory_SwapTotal_bytes{host="$host"}`, "Swap Total"),
	))

	dash.Panel("swap-thrashing", builder.TimeseriesPanel(
		"Swap Paging Activity",
		"ops",
		builder.PromTarget(`rate(node_vmstat_pswpin{host="$host"}[5m])`, "Pages In / s"),
		builder.PromTarget(`rate(node_vmstat_pswpout{host="$host"}[5m])`, "Pages Out / s"),
	))

	dash.Panel("oom-kills", builder.StatPanel(
		"OOM Kills (1h)",
		"short",
		builder.PromInstantTarget(`sum(increase(node_vmstat_oom_kill{host="$host"}[1h]))`, "OOM Kills"),
		builder.PercentThresholds(1, 5),
	))

	// 3. Thermals & Health
	dash.Panel("thermals", builder.TimeseriesPanel(
		"Thermal Zones Temperature",
		"celsius",
		builder.PromTarget(`node_thermal_zone_temp{host="$host"}`, "{{zone}} (type: {{type}})"),
	))

	dash.Panel("file-descriptors", builder.TimeseriesPanel(
		"File Descriptors Allocated",
		"short",
		builder.PromTarget(`node_filefd_allocated{host="$host"}`, "Allocated FDs"),
		builder.PromTarget(`node_filefd_maximum{host="$host"}`, "Max FDs"),
	))

	dash.Panel("uptime", builder.StatPanel(
		"System Uptime",
		"s",
		builder.PromInstantTarget(`time() - node_boot_time_seconds{host="$host"}`, "Uptime"),
		nil,
	))

	dash.Panel("timex-offset", builder.TimeseriesPanel(
		"NTP Time Synchronization Offset & Max Error",
		"s",
		builder.PromTarget(`node_timex_offset_seconds{host="$host"}`, "Offset"),
		builder.PromTarget(`node_timex_maxerror_seconds{host="$host"}`, "Max Estimated Error"),
	))

	dash.Panel("timex-sync", builder.StatPanelWithMappings(
		"NTP Kernel Sync State",
		"short",
		builder.PromInstantTarget(`node_timex_sync_status{host="$host"}`, "Sync Status"),
		builder.TimexSyncStatusValueMappings(),
		builder.BinaryThresholds("red", "green", 1),
	))

	dash.Panel("os-kernel-info", builder.TextInfoStatPanel(
		"Operating System & Kernel Version",
		builder.PromInstantTarget(`node_uname_info{host="$host"}`, "{{sysname}} {{release}} ({{machine}})"),
	))

	// 4. Network & Sockets
	dash.Panel("net-bandwidth", builder.TimeseriesPanelWithOverrides(
		"Network Bandwidth (Rx / Tx)",
		"Bps",
		map[string]string{
			".*Rx.*": "semi-dark-green",
			".*Tx.*": "semi-dark-blue",
		},
		builder.PromTarget(`rate(node_network_receive_bytes_total{host="$host", device!="lo"}[5m])`, "{{device}} Rx"),
		builder.PromTarget(`rate(node_network_transmit_bytes_total{host="$host", device!="lo"}[5m])`, "{{device}} Tx"),
	))

	dash.Panel("net-packets", builder.TimeseriesPanelWithOverrides(
		"Network Packet Rates",
		"pps",
		map[string]string{
			".*Rx.*": "semi-dark-green",
			".*Tx.*": "semi-dark-blue",
		},
		builder.PromTarget(`rate(node_network_receive_packets_total{host="$host", device!="lo"}[5m])`, "{{device}} Rx Packets"),
		builder.PromTarget(`rate(node_network_transmit_packets_total{host="$host", device!="lo"}[5m])`, "{{device}} Tx Packets"),
	))

	dash.Panel("net-errors", builder.TimeseriesPanelWithOverrides(
		"Network Drops & Errors",
		"pps",
		map[string]string{
			".*": "red",
		},
		builder.PromTarget(`rate(node_network_receive_drop_total{host="$host"}[5m])`, "{{device}} Rx Drop"),
		builder.PromTarget(`rate(node_network_receive_errs_total{host="$host"}[5m])`, "{{device}} Rx Err"),
		builder.PromTarget(`rate(node_network_transmit_drop_total{host="$host"}[5m])`, "{{device}} Tx Drop"),
		builder.PromTarget(`rate(node_network_transmit_errs_total{host="$host"}[5m])`, "{{device}} Tx Err"),
	))

	dash.Panel("tcp-sockets", builder.TimeseriesPanel(
		"TCP Socket Allocations",
		"short",
		builder.PromTarget(`node_sockstat_TCP_inuse{host="$host"}`, "In Use"),
		builder.PromTarget(`node_sockstat_TCP_alloc{host="$host"}`, "Allocated"),
		builder.PromTarget(`node_sockstat_TCP_tw{host="$host"}`, "Time Wait"),
	))

	dash.Panel("net-retrans", builder.TimeseriesPanelWithOverrides(
		"TCP Retransmission Rate",
		"pps",
		map[string]string{
			".*": "semi-dark-orange",
		},
		builder.PromTarget(`rate(node_netstat_Tcp_RetransSegs{host="$host"}[5m])`, "Retrans Segs/s"),
	))

	// Layout Structure: 5 Rows (Compact Header Row + 4 Detail Rows)
	dash.RowsLayout(
		dashboard.Rows().
			Row(builder.CompactHeaderRow(
				"Host Key Metrics & Health Overview",
				"header-uptime",
				"header-cpu",
				"header-mem",
				"header-load",
			)).
			Row(dashboard.Row("CPU & System Load").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("cpu-gauge").
					WithItem("cpu-modes").
					WithItem("load-avg").
					WithItem("cpu-context-switches").
					WithItem("procs-state"),
			)).
			Row(dashboard.Row("Memory & Swap").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("mem-gauge").
					WithItem("mem-breakdown").
					WithItem("mem-dirty").
					WithItem("swap-usage").
					WithItem("swap-thrashing").
					WithItem("oom-kills"),
			)).
			Row(dashboard.Row("Thermals & OS Health").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("thermals").
					WithItem("file-descriptors").
					WithItem("uptime").
					WithItem("os-kernel-info").
					WithItem("timex-offset").
					WithItem("timex-sync"),
			)).
			Row(dashboard.Row("Network & Sockets").AutoGridLayout(
				dashboard.AutoGrid().
					WithItem("net-bandwidth").
					WithItem("net-packets").
					WithItem("net-errors").
					WithItem("tcp-sockets").
					WithItem("net-retrans"),
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
