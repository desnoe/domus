package builder

import (
	"fmt"

	"github.com/grafana/grafana-foundation-sdk/go/bargauge"
	"github.com/grafana/grafana-foundation-sdk/go/common"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/gauge"
	"github.com/grafana/grafana-foundation-sdk/go/logs"
	"github.com/grafana/grafana-foundation-sdk/go/loki"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/statetimeline"
	"github.com/grafana/grafana-foundation-sdk/go/table"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
)

var queryCounter = 0

// NextQueryID produces unique refIds across dashboard panel queries
func NextQueryID() string {
	queryCounter++
	return fmt.Sprintf("q%d", queryCounter)
}

// PromTarget creates a standard Prometheus time-series query target
func PromTarget(expr, legend string) *dashboard.TargetBuilder {
	return dashboard.NewTargetBuilder().
		RefId(NextQueryID()).
		Query(
			prometheus.NewQueryV2Builder().
				Datasource(PromDS()).
				Expr(expr).
				LegendFormat(legend),
		)
}

// PromInstantTarget creates an instant query target for Stat and Gauge panels
func PromInstantTarget(expr, legend string) *dashboard.TargetBuilder {
	return dashboard.NewTargetBuilder().
		RefId(NextQueryID()).
		Query(
			prometheus.NewQueryV2Builder().
				Datasource(PromDS()).
				Expr(expr).
				LegendFormat(legend).
				Instant(true),
		)
}

// PromTableTarget creates an instant query target formatted for table visualization
func PromTableTarget(expr string) *dashboard.TargetBuilder {
	return dashboard.NewTargetBuilder().
		RefId(NextQueryID()).
		Query(
			prometheus.NewQueryV2Builder().
				Datasource(PromDS()).
				Expr(expr).
				Format(prometheus.PromQueryFormatTable).
				Instant(true),
		)
}

// LokiTarget creates a Loki log stream query target
func LokiTarget(expr string, maxLines int64) *dashboard.TargetBuilder {
	return dashboard.NewTargetBuilder().
		RefId(NextQueryID()).
		Query(
			loki.NewQueryV2Builder().
				Datasource(LokiDS()).
				Expr(expr).
				MaxLines(maxLines),
		)
}

// LokiInstantMetricTarget creates a Loki instant metric query target
func LokiInstantMetricTarget(expr, legend string) *dashboard.TargetBuilder {
	return dashboard.NewTargetBuilder().
		RefId(NextQueryID()).
		Query(
			loki.NewQueryV2Builder().
				Datasource(LokiDS()).
				Expr(expr).
				LegendFormat(legend).
				Instant(true),
		)
}

// PercentThresholds creates standard green/yellow/red threshold steps
func PercentThresholds(warn, crit float64) *dashboard.ThresholdsConfigBuilder {
	return dashboard.NewThresholdsConfigBuilder().
		Mode(dashboard.ThresholdsModeAbsolute).
		Steps([]dashboard.Threshold{
			{Color: "green", Value: nil},
			{Color: "yellow", Value: &warn},
			{Color: "red", Value: &crit},
		})
}

// BinaryThresholds creates a 2-step threshold (e.g. 0 -> red, 1 -> green)
func BinaryThresholds(colorLow, colorHigh string, splitVal float64) *dashboard.ThresholdsConfigBuilder {
	return dashboard.NewThresholdsConfigBuilder().
		Mode(dashboard.ThresholdsModeAbsolute).
		Steps([]dashboard.Threshold{
			{Color: colorLow, Value: nil},
			{Color: colorHigh, Value: &splitVal},
		})
}

// CountThresholds creates threshold steps for error counters (0 -> green, >=warn -> orange, >=crit -> red)
func CountThresholds(warn, crit float64) *dashboard.ThresholdsConfigBuilder {
	return dashboard.NewThresholdsConfigBuilder().
		Mode(dashboard.ThresholdsModeAbsolute).
		Steps([]dashboard.Threshold{
			{Color: "green", Value: nil},
			{Color: "semi-dark-orange", Value: &warn},
			{Color: "red", Value: &crit},
		})
}

// TimeseriesPanel creates a styled time-series visualization panel with standard palette
func TimeseriesPanel(title, unit string, targets ...*dashboard.TargetBuilder) *dashboard.PanelBuilder {
	return TimeseriesPanelWithOverrides(title, unit, nil, targets...)
}

// TimeseriesPanelWithOverrides creates a styled time-series panel with regex color overrides
func TimeseriesPanelWithOverrides(title, unit string, colorOverrides map[string]string, targets ...*dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder()
	for _, t := range targets {
		qg.Target(t)
	}

	viz := timeseries.NewVisualizationV2Builder().
		Unit(unit).
		LineInterpolation(common.LineInterpolationSmooth).
		LineWidth(1.5).
		FillOpacity(8).
		ColorScheme(
			dashboard.NewFieldColorBuilder().
				Mode(dashboard.FieldColorModeIdPaletteClassic),
		).
		Legend(
			common.NewVizLegendOptionsBuilder().
				DisplayMode(common.LegendDisplayModeTable).
				Placement(common.LegendPlacementBottom).
				Calcs([]string{"lastNotNull", "max"}),
		)

	for pattern, color := range colorOverrides {
		viz.OverrideByRegexp(pattern, []dashboard.DynamicConfigValue{
			{
				Id: "color",
				Value: map[string]any{
					"mode":       "fixed",
					"fixedColor": color,
				},
			},
		})
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// StackedTimeseriesPanel creates a stacked time-series visualization panel
func StackedTimeseriesPanel(title, unit string, targets ...*dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder()
	for _, t := range targets {
		qg.Target(t)
	}

	viz := timeseries.NewVisualizationV2Builder().
		Unit(unit).
		LineInterpolation(common.LineInterpolationLinear).
		LineWidth(1.0).
		FillOpacity(30).
		Stacking(common.NewStackingConfigBuilder().Mode(common.StackingModeNormal)).
		ColorScheme(
			dashboard.NewFieldColorBuilder().
				Mode(dashboard.FieldColorModeIdPaletteClassic),
		).
		Legend(
			common.NewVizLegendOptionsBuilder().
				DisplayMode(common.LegendDisplayModeTable).
				Placement(common.LegendPlacementBottom).
				Calcs([]string{"lastNotNull", "max"}),
		)

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// HeaderStatPanel creates a key synthesis metric card for the top header row
func HeaderStatPanel(title, unit string, target *dashboard.TargetBuilder, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		Unit(unit).
		GraphMode(common.BigValueGraphModeNone).
		ColorMode(common.BigValueColorModeValue).
		NoValue("0")

	if thresholds != nil {
		viz.Thresholds(thresholds)
	} else {
		viz.Thresholds(
			dashboard.NewThresholdsConfigBuilder().
				Mode(dashboard.ThresholdsModeAbsolute).
				Steps([]dashboard.Threshold{
					{Color: "green", Value: nil},
				}),
		)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// StatPanel creates a single-value stat panel with sparkline
func StatPanel(title, unit string, target *dashboard.TargetBuilder, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		Unit(unit).
		GraphMode(common.BigValueGraphModeArea).
		ColorMode(common.BigValueColorModeValue).
		NoValue("0")

	if thresholds != nil {
		viz.Thresholds(thresholds)
	} else {
		viz.Thresholds(
			dashboard.NewThresholdsConfigBuilder().
				Mode(dashboard.ThresholdsModeAbsolute).
				Steps([]dashboard.Threshold{
					{Color: "green", Value: nil},
				}),
		)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// GaugePanel creates a circular gauge panel
func GaugePanel(title, unit string, min, max float64, target *dashboard.TargetBuilder, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := gauge.NewVisualizationV2Builder().
		Unit(unit).
		Min(min).
		Max(max)

	if thresholds != nil {
		viz.Thresholds(thresholds)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// BarGaugePanel creates a horizontal or vertical bar gauge panel displaying all series
func BarGaugePanel(title, unit string, min, max float64, target *dashboard.TargetBuilder, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := bargauge.NewVisualizationV2Builder().
		Unit(unit).
		Min(min).
		Max(max).
		Orientation(common.VizOrientationHorizontal).
		DisplayMode(common.BarGaugeDisplayModeGradient).
		ReduceOptions(
			common.NewReduceDataOptionsBuilder().
				Calcs([]string{"lastNotNull"}).
				Fields(""),
		)

	if thresholds != nil {
		viz.Thresholds(thresholds)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// LogsPanel creates a journal/loki log viewer panel
func LogsPanel(title string, target *dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := logs.NewVisualizationV2Builder().
		ShowTime(true).
		ShowLabels(false).
		WrapLogMessage(true).
		EnableLogDetails(true).
		SortOrder(common.LogsSortOrderDescending)

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

func stringPtr(s string) *string {
	return &s
}

// BackupExecutionValueMappings creates value mappings for backup execution states (0 -> Idle, 1 -> Syncing)
func BackupExecutionValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("Idle (Standby)"),
						Color: stringPtr("semi-dark-blue"),
					},
					"1": {
						Text:  stringPtr("Syncing / Snapshotting"),
						Color: stringPtr("green"),
					},
				},
			},
		},
	}
}

// ServiceStatusValueMappings creates value mappings for daemon/scrape up/down states (1 -> UP, 0 -> DOWN)
func ServiceStatusValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("DOWN"),
						Color: stringPtr("red"),
					},
					"1": {
						Text:  stringPtr("UP"),
						Color: stringPtr("green"),
					},
				},
			},
		},
	}
}

// SmartStatusValueMappings creates value mappings for S.M.A.R.T. device health (1 -> PASSED, 0 -> FAILED)
func SmartStatusValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("FAILED"),
						Color: stringPtr("red"),
					},
					"1": {
						Text:  stringPtr("PASSED"),
						Color: stringPtr("green"),
					},
				},
			},
		},
	}
}

// SmartHeaderValueMappings creates value mappings for header synthesis S.M.A.R.T. status (1 -> ALL PASSED, 0 -> DRIVE FAILED)
func SmartHeaderValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("DRIVE FAILED"),
						Color: stringPtr("red"),
					},
					"1": {
						Text:  stringPtr("ALL PASSED"),
						Color: stringPtr("green"),
					},
				},
			},
		},
	}
}

// FilesystemStatusValueMappings creates value mappings for filesystem read-only / error states
func FilesystemStatusValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("HEALTHY"),
						Color: stringPtr("green"),
					},
					"1": {
						Text:  stringPtr("READ-ONLY / FAULT"),
						Color: stringPtr("red"),
					},
				},
			},
		},
	}
}

// SmartExitStatusValueMappings creates value mappings for smartctl exit status codes (0 -> SUCCESS, >0 -> ATA ERROR)
func SmartExitStatusValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("SUCCESS (0)"),
						Color: stringPtr("green"),
					},
					"1": {
						Text:  stringPtr("CLI ERROR (1)"),
						Color: stringPtr("red"),
					},
					"2": {
						Text:  stringPtr("DEV OPEN FAILED (2)"),
						Color: stringPtr("red"),
					},
					"4": {
						Text:  stringPtr("ATA / CHECKSUM WARN (4)"),
						Color: stringPtr("semi-dark-orange"),
					},
					"8": {
						Text:  stringPtr("DISK FAILING (8)"),
						Color: stringPtr("red"),
					},
				},
			},
		},
	}
}

// TimexSyncStatusValueMappings creates value mappings for NTP kernel sync status (1 -> SYNCHRONIZED, 0 -> DESYNCHRONIZED)
func TimexSyncStatusValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("DESYNCHRONIZED / DRIFT"),
						Color: stringPtr("red"),
					},
					"1": {
						Text:  stringPtr("SYNCHRONIZED"),
						Color: stringPtr("green"),
					},
				},
			},
		},
	}
}

// DriveRotationRateValueMappings creates value mappings for disk rotational speed (0 -> SSD, >0 -> RPM)
func DriveRotationRateValueMappings() []dashboard.ValueMapping {
	return []dashboard.ValueMapping{
		{
			ValueMap: &dashboard.ValueMap{
				Type: dashboard.MappingTypeValue,
				Options: map[string]dashboard.ValueMappingResult{
					"0": {
						Text:  stringPtr("SSD (Solid State)"),
						Color: stringPtr("semi-dark-blue"),
					},
					"5400": {
						Text:  stringPtr("5400 RPM (HDD)"),
						Color: stringPtr("purple"),
					},
					"5900": {
						Text:  stringPtr("5900 RPM (HDD)"),
						Color: stringPtr("purple"),
					},
					"7200": {
						Text:  stringPtr("7200 RPM (HDD)"),
						Color: stringPtr("purple"),
					},
					"10000": {
						Text:  stringPtr("10000 RPM (HDD)"),
						Color: stringPtr("purple"),
					},
					"15000": {
						Text:  stringPtr("15000 RPM (HDD)"),
						Color: stringPtr("purple"),
					},
				},
			},
		},
	}
}

// CompactHeaderRow creates a standard compact 4-metric synthesis row with short height
func CompactHeaderRow(title string, items ...string) *dashboard.RowBuilder {
	grid := dashboard.AutoGrid().
		MaxColumnCount(4).
		RowHeightMode(dashboard.AutoGridLayoutSpecRowHeightModeShort)
	for _, item := range items {
		grid.WithItem(item)
	}
	return dashboard.Row(title).AutoGridLayout(grid)
}

// LogsRow creates a dedicated full-width row with tall height for log viewers
func LogsRow(title, item string) *dashboard.RowBuilder {
	grid := dashboard.AutoGrid().
		MaxColumnCount(1).
		RowHeightMode(dashboard.AutoGridLayoutSpecRowHeightModeTall).
		WithItem(item)
	return dashboard.Row(title).AutoGridLayout(grid)
}

// StatPanelWithMappings creates a single-value stat panel with custom value mappings
func StatPanelWithMappings(title, unit string, target *dashboard.TargetBuilder, mappings []dashboard.ValueMapping, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		Unit(unit).
		GraphMode(common.BigValueGraphModeArea).
		ColorMode(common.BigValueColorModeValue).
		NoValue("0").
		Mappings(mappings)

	if thresholds != nil {
		viz.Thresholds(thresholds)
	} else {
		viz.Thresholds(
			dashboard.NewThresholdsConfigBuilder().
				Mode(dashboard.ThresholdsModeAbsolute).
				Steps([]dashboard.Threshold{
					{Color: "green", Value: nil},
				}),
		)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// HeaderStatPanelWithMappings creates a key synthesis metric card with custom value mappings
func HeaderStatPanelWithMappings(title, unit string, target *dashboard.TargetBuilder, mappings []dashboard.ValueMapping, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		Unit(unit).
		GraphMode(common.BigValueGraphModeNone).
		ColorMode(common.BigValueColorModeValue).
		NoValue("0").
		Mappings(mappings)

	if thresholds != nil {
		viz.Thresholds(thresholds)
	} else {
		viz.Thresholds(
			dashboard.NewThresholdsConfigBuilder().
				Mode(dashboard.ThresholdsModeAbsolute).
				Steps([]dashboard.Threshold{
					{Color: "green", Value: nil},
				}),
		)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// MultiStatPanel creates a multi-item stat grid displaying all series side-by-side
func MultiStatPanel(title, unit string, target *dashboard.TargetBuilder, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	return MultiStatPanelWithMappings(title, unit, target, nil, thresholds)
}

// MultiStatPanelWithMappings creates a multi-item stat grid displaying all series side-by-side with value mappings
func MultiStatPanelWithMappings(title, unit string, target *dashboard.TargetBuilder, mappings []dashboard.ValueMapping, thresholds *dashboard.ThresholdsConfigBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		Unit(unit).
		GraphMode(common.BigValueGraphModeNone).
		ColorMode(common.BigValueColorModeValue).
		TextMode(common.BigValueTextModeValueAndName).
		Orientation(common.VizOrientationHorizontal).
		NoValue("0").
		ReduceOptions(
			common.NewReduceDataOptionsBuilder().
				Calcs([]string{"lastNotNull"}).
				Fields(""),
		)

	if mappings != nil {
		viz.Mappings(mappings)
	}

	if thresholds != nil {
		viz.Thresholds(thresholds)
	} else {
		viz.Thresholds(
			dashboard.NewThresholdsConfigBuilder().
				Mode(dashboard.ThresholdsModeAbsolute).
				Steps([]dashboard.Threshold{
					{Color: "green", Value: nil},
				}),
		)
	}

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// TextInfoStatPanel creates a single-stat panel displaying metadata label text
func TextInfoStatPanel(title string, target *dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder().Target(target)

	viz := stat.NewVisualizationV2Builder().
		GraphMode(common.BigValueGraphModeNone).
		ColorMode(common.BigValueColorModeNone).
		TextMode(common.BigValueTextModeName).
		NoValue("-").
		ReduceOptions(
			common.NewReduceDataOptionsBuilder().
				Calcs([]string{"lastNotNull"}).
				Fields(""),
		)

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// StateTimelinePanel creates a state timeline visualization panel for service execution history
func StateTimelinePanel(title string, targets ...*dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder()
	for _, t := range targets {
		qg.Target(t)
	}

	viz := statetimeline.NewVisualizationV2Builder().
		RowHeight(0.85).
		ShowValue(common.VisibilityModeAuto).
		MergeValues(true).
		Mappings([]dashboard.ValueMapping{
			{
				ValueMap: &dashboard.ValueMap{
					Type: dashboard.MappingTypeValue,
					Options: map[string]dashboard.ValueMappingResult{
						"0": {
							Text:  stringPtr("Standby / Inactive"),
							Color: stringPtr("semi-dark-blue"),
						},
						"1": {
							Text:  stringPtr("Running / Active"),
							Color: stringPtr("green"),
						},
					},
				},
			},
		}).
		Legend(
			common.NewVizLegendOptionsBuilder().
				DisplayMode(common.LegendDisplayModeList).
				Placement(common.LegendPlacementBottom),
		)

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}

// TablePanel creates a styled table visualization panel for inventory and service lists
func TablePanel(title string, targets ...*dashboard.TargetBuilder) *dashboard.PanelBuilder {
	qg := dashboard.NewQueryGroupBuilder()
	for _, t := range targets {
		qg.Target(t)
	}

	viz := table.NewVisualizationV2Builder().
		ShowHeader(true).
		Filterable(true).
		Align(common.FieldTextAlignmentAuto).
		CellHeight(common.TableCellHeightSm)

	// Hide redundant PromQL instant query columns to display a clean inventory
	for _, col := range []string{"Time", "__name__", "host", "instance", "job", "Value"} {
		viz.OverrideByName(col, []dashboard.DynamicConfigValue{
			{
				Id:    "custom.hidden",
				Value: true,
			},
		})
	}

	// Format remaining inventory columns with clear titles and status badges
	viz.OverrideByName("name", []dashboard.DynamicConfigValue{
		{
			Id:    "displayName",
			Value: "Service Unit",
		},
	})

	viz.OverrideByName("type", []dashboard.DynamicConfigValue{
		{
			Id:    "displayName",
			Value: "Startup Type",
		},
	})

	viz.OverrideByName("state", []dashboard.DynamicConfigValue{
		{
			Id:    "displayName",
			Value: "Current State",
		},
		{
			Id: "custom.cellOptions",
			Value: map[string]any{
				"type": "color-background",
				"mode": "basic",
			},
		},
		{
			Id: "mappings",
			Value: []map[string]any{
				{
					"type": "value",
					"options": map[string]any{
						"active": map[string]any{
							"text":  "ACTIVE",
							"color": "green",
						},
						"inactive": map[string]any{
							"text":  "STANDBY",
							"color": "semi-dark-blue",
						},
						"failed": map[string]any{
							"text":  "FAILED",
							"color": "red",
						},
					},
				},
			},
		},
	})

	return dashboard.NewPanelBuilder().
		Title(title).
		Visualization(viz).
		Data(qg)
}
