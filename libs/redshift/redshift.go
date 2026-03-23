// Package redshift provides Amazon Redshift-specific operations including
// vacuum/analyze, materialized views, WLM queue management, concurrency
// scaling, and Spectrum external tables.
package redshift

import (
	"context"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// RedshiftLibrary provides Redshift-specific operations.
type RedshiftLibrary struct {
	client *gopgbase.Client
}

// NewRedshiftLibrary creates a new RedshiftLibrary backed by the given Client.
func NewRedshiftLibrary(client *gopgbase.Client) (*RedshiftLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/redshift: client must not be nil")
	}
	return &RedshiftLibrary{client: client}, nil
}

// VacuumLibrary provides vacuum and analyze operations for Redshift.
type VacuumLibrary struct {
	client *gopgbase.Client
}

// Vacuum returns a VacuumLibrary for maintenance operations.
func (l *RedshiftLibrary) Vacuum() *VacuumLibrary {
	return &VacuumLibrary{client: l.client}
}

// Full runs a full VACUUM on the specified table.
func (v *VacuumLibrary) Full(ctx context.Context, table string) error {
	query := fmt.Sprintf("VACUUM FULL %s", quoteIdentifier(table))
	_, err := v.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: vacuum full: %w", err)
	}
	return nil
}

// SortOnly runs a VACUUM SORT ONLY on the specified table.
func (v *VacuumLibrary) SortOnly(ctx context.Context, table string) error {
	query := fmt.Sprintf("VACUUM SORT ONLY %s", quoteIdentifier(table))
	_, err := v.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: vacuum sort only: %w", err)
	}
	return nil
}

// DeleteOnly runs a VACUUM DELETE ONLY on the specified table.
func (v *VacuumLibrary) DeleteOnly(ctx context.Context, table string) error {
	query := fmt.Sprintf("VACUUM DELETE ONLY %s", quoteIdentifier(table))
	_, err := v.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: vacuum delete only: %w", err)
	}
	return nil
}

// AutoVacuumTune runs ANALYZE on a table to update statistics,
// which helps the query optimizer choose better execution plans.
func (l *RedshiftLibrary) AutoVacuumTune(ctx context.Context, table string) error {
	query := fmt.Sprintf("ANALYZE %s", quoteIdentifier(table))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: auto vacuum tune: %w", err)
	}
	return nil
}

// AnalyticsLibrary provides analytical query helpers for Redshift.
type AnalyticsLibrary struct {
	client *gopgbase.Client
}

// Analytics returns an AnalyticsLibrary for analytical operations.
func (l *RedshiftLibrary) Analytics() *AnalyticsLibrary {
	return &AnalyticsLibrary{client: l.client}
}

// TableStats returns disk usage and row count statistics for a table.
func (a *AnalyticsLibrary) TableStats(ctx context.Context, table string) (map[string]any, error) {
	query := `
		SELECT "table", size, tbl_rows, skew_rows
		FROM svv_table_info
		WHERE "table" = $1
	`
	row := a.client.DataStore().QueryRowContext(ctx, query, table)
	var tableName string
	var size int64
	var tblRows int64
	var skewRows float64
	if err := row.Scan(&tableName, &size, &tblRows, &skewRows); err != nil {
		return nil, fmt.Errorf("gopgbase/redshift: table stats: %w", err)
	}
	return map[string]any{
		"table":     tableName,
		"size_mb":   size,
		"rows":      tblRows,
		"skew_rows": skewRows,
	}, nil
}

// MaterializedView creates a materialized view in Redshift.
func (l *RedshiftLibrary) MaterializedView(ctx context.Context, name, query string) error {
	stmt := fmt.Sprintf(
		"CREATE MATERIALIZED VIEW IF NOT EXISTS %s AS %s",
		quoteIdentifier(name), query,
	)
	_, err := l.client.DataStore().ExecContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: materialized view: %w", err)
	}
	return nil
}

// RefreshMaterializedView refreshes an existing materialized view.
func (l *RedshiftLibrary) RefreshMaterializedView(ctx context.Context, name string) error {
	query := fmt.Sprintf("REFRESH MATERIALIZED VIEW %s", quoteIdentifier(name))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: refresh materialized view: %w", err)
	}
	return nil
}

// WLMQueue sets the WLM queue for the current session.
func (l *RedshiftLibrary) WLMQueue(ctx context.Context, queueName string) error {
	query := fmt.Sprintf("SET query_group TO '%s'", strings.ReplaceAll(queueName, "'", "''"))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: wlm queue: %w", err)
	}
	return nil
}

// ConcurrencyScaling enables or disables concurrency scaling for the cluster.
func (l *RedshiftLibrary) ConcurrencyScaling(ctx context.Context, enable bool) error {
	value := "off"
	if enable {
		value = "auto"
	}
	query := fmt.Sprintf("SET enable_result_cache_for_session = %s", value)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: concurrency scaling: %w", err)
	}
	return nil
}

// CreateExternalTable creates a Redshift Spectrum external table
// that references data in S3.
//
// Parameters:
//   - table: external table name (schema-qualified, e.g., "spectrum.events")
//   - s3Path: S3 path to the data (e.g., "s3://bucket/path/")
//   - format: data format (e.g., "PARQUET", "CSV", "JSON")
//   - columns: column definitions (e.g., "id INT, name VARCHAR(100)")
func (l *RedshiftLibrary) CreateExternalTable(ctx context.Context, table, s3Path, format, columns string) error {
	query := fmt.Sprintf(
		`CREATE EXTERNAL TABLE IF NOT EXISTS %s (%s)
		STORED AS %s
		LOCATION '%s'`,
		table, // External tables use schema.table format, not quoted
		columns,
		format,
		strings.ReplaceAll(s3Path, "'", "''"),
	)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/redshift: create external table: %w", err)
	}
	return nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
