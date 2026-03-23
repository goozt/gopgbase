package common

import (
	"context"
	"encoding/json"
	"fmt"

	gopgbase "github.com/goozt/gopgbase"
)

// ExplainResult holds the parsed output of EXPLAIN ANALYZE.
type ExplainResult struct {
	Query     string  `json:"query"`
	RawPlan   string  `json:"raw_plan"`
	PlanJSON  []any   `json:"plan_json,omitempty"`
	TotalCost float64 `json:"total_cost,omitempty"`
	TotalTime float64 `json:"total_time_ms,omitempty"`
}

// SlowQuery represents a query that exceeded a latency threshold.
type SlowQuery struct {
	Query       string  `json:"query"`
	Calls       int64   `json:"calls"`
	MeanTimeMS  float64 `json:"mean_time_ms"`
	TotalTimeMS float64 `json:"total_time_ms"`
}

// ExplainAnalyze runs EXPLAIN (ANALYZE, FORMAT JSON) on the given query
// and returns the parsed execution plan.
func ExplainAnalyze(ctx context.Context, client *gopgbase.Client, query string, args ...any) (*ExplainResult, error) {
	explainQuery := "EXPLAIN (ANALYZE, FORMAT JSON) " + query
	rows, err := client.DataStore().QueryContext(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/common: explain analyze: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var planText string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("gopgbase/common: explain analyze scan: %w", err)
		}
		planText += line
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gopgbase/common: explain analyze rows: %w", err)
	}

	result := &ExplainResult{
		Query:   query,
		RawPlan: planText,
	}

	var planJSON []any
	if err := json.Unmarshal([]byte(planText), &planJSON); err == nil {
		result.PlanJSON = planJSON
	}

	return result, nil
}

// SlowQueryLog queries pg_stat_statements for queries exceeding the
// given threshold in milliseconds.
func SlowQueryLog(ctx context.Context, client *gopgbase.Client, thresholdMS int) ([]SlowQuery, error) {
	query := `
		SELECT query, calls, mean_exec_time, total_exec_time
		FROM pg_stat_statements
		WHERE mean_exec_time > $1
		ORDER BY mean_exec_time DESC
		LIMIT 50
	`

	rows, err := client.DataStore().QueryContext(ctx, query, float64(thresholdMS))
	if err != nil {
		return nil, fmt.Errorf("gopgbase/common: slow query log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SlowQuery
	for rows.Next() {
		var sq SlowQuery
		if err := rows.Scan(&sq.Query, &sq.Calls, &sq.MeanTimeMS, &sq.TotalTimeMS); err != nil {
			return nil, fmt.Errorf("gopgbase/common: slow query log scan: %w", err)
		}
		results = append(results, sq)
	}

	return results, rows.Err()
}
