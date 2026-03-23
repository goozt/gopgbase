// Package timescale provides TimescaleDB-specific operations including
// hypertable management, continuous aggregates, compression policies,
// retention policies, and hyperfunctions.
package timescale

import (
	"context"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// TimescaleLibrary provides TimescaleDB-specific operations.
type TimescaleLibrary struct {
	client *gopgbase.Client
}

// NewTimescaleLibrary creates a new TimescaleLibrary backed by the given Client.
func NewTimescaleLibrary(client *gopgbase.Client) (*TimescaleLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/timescale: client must not be nil")
	}
	return &TimescaleLibrary{client: client}, nil
}

// CreateHypertable converts a regular table into a TimescaleDB hypertable.
//
// Parameters:
//   - table: name of the existing table
//   - timeCol: the time-based column for partitioning
//   - partitionCol: optional space-partitioning column (empty to skip)
//   - ifNotExists: if true, does not error if hypertable already exists
func (l *TimescaleLibrary) CreateHypertable(ctx context.Context, table, timeCol, partitionCol string, ifNotExists bool) error {
	var query string
	if partitionCol != "" {
		query = fmt.Sprintf(
			"SELECT create_hypertable(%s, %s, %s, if_not_exists => %t)",
			quote(table), quote(timeCol), quote(partitionCol), ifNotExists,
		)
	} else {
		query = fmt.Sprintf(
			"SELECT create_hypertable(%s, %s, if_not_exists => %t)",
			quote(table), quote(timeCol), ifNotExists,
		)
	}

	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: create hypertable: %w", err)
	}
	return nil
}

// ConvertToHypertable converts an existing table to a hypertable,
// migrating existing data.
func (l *TimescaleLibrary) ConvertToHypertable(ctx context.Context, table, timeCol string) error {
	query := fmt.Sprintf(
		"SELECT create_hypertable(%s, %s, migrate_data => true, if_not_exists => true)",
		quote(table), quote(timeCol),
	)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: convert to hypertable: %w", err)
	}
	return nil
}

// DropHypertable drops a hypertable and all its chunks.
func (l *TimescaleLibrary) DropHypertable(ctx context.Context, table string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quoteIdentifier(table))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: drop hypertable: %w", err)
	}
	return nil
}

// TimeBucketResult represents a row from a time_bucket aggregation.
type TimeBucketResult struct {
	Bucket string  `json:"bucket"`
	Value  float64 `json:"value"`
}

// TimeBucketAgg performs a time_bucket aggregation query.
//
// Parameters:
//   - table: hypertable name
//   - bucket: time bucket interval (e.g., "1 hour", "5 minutes")
//   - aggFunc: aggregation function (e.g., "AVG", "SUM", "MAX")
//   - valueCol: column to aggregate
//   - where: optional WHERE clause
//   - args: placeholder arguments for the WHERE clause
func (l *TimescaleLibrary) TimeBucketAgg(ctx context.Context, table, bucket, aggFunc, valueCol, where string, args ...any) ([]TimeBucketResult, error) {
	query := fmt.Sprintf(
		"SELECT time_bucket($1, time)::text AS bucket, %s(%s) AS value FROM %s",
		aggFunc, quoteIdentifier(valueCol), quoteIdentifier(table),
	)

	// Shift args to account for the $1 bucket parameter.
	allArgs := make([]any, 0, len(args)+1)
	allArgs = append(allArgs, bucket)

	if where != "" {
		query += " WHERE " + where
		allArgs = append(allArgs, args...)
	}
	query += " GROUP BY bucket ORDER BY bucket"

	rows, err := l.client.DataStore().QueryContext(ctx, query, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/timescale: time bucket agg: %w", err)
	}
	defer rows.Close()

	var results []TimeBucketResult
	for rows.Next() {
		var r TimeBucketResult
		if err := rows.Scan(&r.Bucket, &r.Value); err != nil {
			return nil, fmt.Errorf("gopgbase/timescale: time bucket agg scan: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// ContinuousAggView creates a continuous aggregate materialized view.
//
// Parameters:
//   - matView: name of the materialized view to create
//   - sourceTable: source hypertable
//   - bucketSize: time bucket size (e.g., "1 hour")
//   - aggs: aggregation expressions (e.g., "AVG(temperature) AS avg_temp")
func (l *TimescaleLibrary) ContinuousAggView(ctx context.Context, matView, sourceTable, bucketSize string, aggs []string) error {
	query := fmt.Sprintf(
		`CREATE MATERIALIZED VIEW IF NOT EXISTS %s
		WITH (timescaledb.continuous) AS
		SELECT time_bucket('%s', time) AS bucket, %s
		FROM %s
		GROUP BY bucket`,
		quoteIdentifier(matView),
		bucketSize,
		strings.Join(aggs, ", "),
		quoteIdentifier(sourceTable),
	)

	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: continuous agg view: %w", err)
	}
	return nil
}

// AddCompressionPolicy adds a compression policy to a hypertable.
// afterInterval specifies how old data must be before compression
// (e.g., "7 days").
func (l *TimescaleLibrary) AddCompressionPolicy(ctx context.Context, table, afterInterval string) error {
	// First enable compression on the hypertable.
	enableQuery := fmt.Sprintf(
		"ALTER TABLE %s SET (timescaledb.compress)",
		quoteIdentifier(table),
	)
	if _, err := l.client.DataStore().ExecContext(ctx, enableQuery); err != nil {
		return fmt.Errorf("gopgbase/timescale: enable compression: %w", err)
	}

	// Then add the policy.
	policyQuery := fmt.Sprintf(
		"SELECT add_compression_policy(%s, INTERVAL '%s')",
		quote(table), afterInterval,
	)
	if _, err := l.client.DataStore().ExecContext(ctx, policyQuery); err != nil {
		return fmt.Errorf("gopgbase/timescale: add compression policy: %w", err)
	}

	return nil
}

// AddRetentionPolicy adds a data retention policy that automatically
// drops chunks older than the specified interval.
func (l *TimescaleLibrary) AddRetentionPolicy(ctx context.Context, table, dropAfter string) error {
	query := fmt.Sprintf(
		"SELECT add_retention_policy(%s, INTERVAL '%s')",
		quote(table), dropAfter,
	)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: add retention policy: %w", err)
	}
	return nil
}

// RefreshContinuousAgg manually refreshes a continuous aggregate.
func (l *TimescaleLibrary) RefreshContinuousAgg(ctx context.Context, matview string) error {
	query := fmt.Sprintf("CALL refresh_continuous_aggregate(%s, NULL, NULL)", quote(matview))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/timescale: refresh continuous agg: %w", err)
	}
	return nil
}

// LTTBResult represents a point in an LTTB (Largest Triangle Three Buckets) downsampled result.
type LTTBResult struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

// LTTB performs Largest Triangle Three Buckets downsampling using
// TimescaleDB's lttb hyperfunction.
func (l *TimescaleLibrary) LTTB(ctx context.Context, table, timeCol, valueCol string, bucketSize string, maxPoints int) ([]LTTBResult, error) {
	query := fmt.Sprintf(
		`SELECT time::text, value FROM unnest(
			(SELECT lttb(%s, %s, $1)
			 FROM %s
			 WHERE %s >= NOW() - INTERVAL '%s')
		)`,
		quoteIdentifier(timeCol), quoteIdentifier(valueCol),
		quoteIdentifier(table),
		quoteIdentifier(timeCol), bucketSize,
	)

	rows, err := l.client.DataStore().QueryContext(ctx, query, maxPoints)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/timescale: lttb: %w", err)
	}
	defer rows.Close()

	var results []LTTBResult
	for rows.Next() {
		var r LTTBResult
		if err := rows.Scan(&r.Time, &r.Value); err != nil {
			return nil, fmt.Errorf("gopgbase/timescale: lttb scan: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// quote wraps a string in single quotes for SQL string literals.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
