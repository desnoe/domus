package builder

import (
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
)

type prometheusVariableQueryBuilder struct {
	query string
}

func (b *prometheusVariableQueryBuilder) Build() (dashboard.DataQueryKind, error) {
	ds, err := PromDS().Build()
	if err != nil {
		return dashboard.DataQueryKind{}, err
	}
	return dashboard.DataQueryKind{
		Kind:       "DataQuery",
		Group:      "prometheus",
		Version:    "v0",
		Datasource: &ds,
		Spec: map[string]any{
			"query":   b.query,
			"qryType": 1,
		},
	}, nil
}

// HostVariable builds the standard dynamic $host dropdown variable
func HostVariable() *dashboard.QueryVariableBuilder {
	query := &prometheusVariableQueryBuilder{
		query: `label_values(up{job="integrations/node_exporter"}, host)`,
	}

	defaultHost := "kodlix.delarche.fr"
	selected := true
	return dashboard.NewQueryVariableBuilder("host").
		Label("Host").
		Query(query).
		Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
		IncludeAll(false).
		Multi(false).
		Current(dashboard.VariableOption{
			Selected: &selected,
			Text:     dashboard.StringOrArrayOfString{String: &defaultHost},
			Value:    dashboard.StringOrArrayOfString{String: &defaultHost},
		}).
		Sort(dashboard.VariableSortAlphabeticalAsc)
}
