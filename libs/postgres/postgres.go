// Package postgres provides PostgreSQL-specific convenience operations
// including extension management, maintenance, and monitoring.
package postgres

import (
	"context"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// PostgresLibrary provides PostgreSQL-specific operations.
type PostgresLibrary struct {
	client *gopgbase.Client
}

// NewPostgresLibrary creates a new PostgresLibrary backed by the given Client.
func NewPostgresLibrary(client *gopgbase.Client) (*PostgresLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/postgres: client must not be nil")
	}
	return &PostgresLibrary{client: client}, nil
}

// ExtensionManager returns an ExtensionManager for enabling/disabling extensions.
func (l *PostgresLibrary) ExtensionManager(_ context.Context) *ExtensionManager {
	return &ExtensionManager{client: l.client}
}

// ExtensionManager manages PostgreSQL extensions.
type ExtensionManager struct {
	client *gopgbase.Client
}

// Enable creates a PostgreSQL extension if it does not already exist.
func (em *ExtensionManager) Enable(ctx context.Context, extension string) error {
	query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s", quoteIdentifier(extension))
	_, err := em.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/postgres: enable extension %s: %w", extension, err)
	}
	return nil
}

// EnablePostGIS enables the PostGIS extension.
func (em *ExtensionManager) EnablePostGIS(ctx context.Context) error {
	return em.Enable(ctx, "postgis")
}

// EnableUUID enables the uuid-ossp extension.
func (em *ExtensionManager) EnableUUID(ctx context.Context) error {
	return em.Enable(ctx, "uuid-ossp")
}

// List returns all installed extensions.
func (em *ExtensionManager) List(ctx context.Context) ([]string, error) {
	rows, err := em.client.DataStore().QueryContext(ctx, "SELECT extname FROM pg_extension ORDER BY extname")
	if err != nil {
		return nil, fmt.Errorf("gopgbase/postgres: list extensions: %w", err)
	}
	defer rows.Close()

	var exts []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("gopgbase/postgres: list extensions scan: %w", err)
		}
		exts = append(exts, name)
	}
	return exts, rows.Err()
}

// VacuumAnalyze runs VACUUM ANALYZE on the specified table.
// If analyzeOnly is true, only ANALYZE is run (no VACUUM).
func (l *PostgresLibrary) VacuumAnalyze(ctx context.Context, table string, analyzeOnly bool) error {
	var query string
	if analyzeOnly {
		query = fmt.Sprintf("ANALYZE %s", quoteIdentifier(table))
	} else {
		query = fmt.Sprintf("VACUUM ANALYZE %s", quoteIdentifier(table))
	}

	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/postgres: vacuum analyze: %w", err)
	}
	return nil
}

// IndexRecommendation represents a suggested index.
type IndexRecommendation struct {
	Table       string  `json:"table"`
	Columns     string  `json:"columns"`
	SeqScans    int64   `json:"seq_scans"`
	SeqTupRead  int64   `json:"seq_tup_read"`
	IdxScans    int64   `json:"idx_scans"`
	Suggestion  string  `json:"suggestion"`
}

// IndexAdvisor analyzes pg_stat_user_tables to recommend indexes for
// tables with high sequential scan counts.
func (l *PostgresLibrary) IndexAdvisor(ctx context.Context, table string) ([]IndexRecommendation, error) {
	query := `
		SELECT relname, seq_scan, seq_tup_read, COALESCE(idx_scan, 0)
		FROM pg_stat_user_tables
		WHERE seq_scan > 100 AND ($1 = '' OR relname = $1)
		ORDER BY seq_scan DESC
		LIMIT 20
	`

	rows, err := l.client.DataStore().QueryContext(ctx, query, table)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/postgres: index advisor: %w", err)
	}
	defer rows.Close()

	var recs []IndexRecommendation
	for rows.Next() {
		var r IndexRecommendation
		if err := rows.Scan(&r.Table, &r.SeqScans, &r.SeqTupRead, &r.IdxScans); err != nil {
			return nil, fmt.Errorf("gopgbase/postgres: index advisor scan: %w", err)
		}
		r.Suggestion = fmt.Sprintf("Table %q has %d sequential scans vs %d index scans — consider adding indexes", r.Table, r.SeqScans, r.IdxScans)
		recs = append(recs, r)
	}

	return recs, rows.Err()
}

// ReplicationLag returns the replication lag for logical replication slots.
func (l *PostgresLibrary) ReplicationLag(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT slot_name,
		       pg_wal_lsn_diff(pg_current_wal_lsn(), confirmed_flush_lsn)::text AS lag_bytes
		FROM pg_replication_slots
		WHERE slot_type = 'logical'
	`

	rows, err := l.client.DataStore().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/postgres: replication lag: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var slot, lag string
		if err := rows.Scan(&slot, &lag); err != nil {
			return nil, fmt.Errorf("gopgbase/postgres: replication lag scan: %w", err)
		}
		result[slot] = lag
	}

	return result, rows.Err()
}

// PgStatStatement represents a row from pg_stat_statements.
type PgStatStatement struct {
	Query         string  `json:"query"`
	Calls         int64   `json:"calls"`
	TotalTimeMS   float64 `json:"total_time_ms"`
	MeanTimeMS    float64 `json:"mean_time_ms"`
	Rows          int64   `json:"rows"`
}

// PgStatStatements returns the top N queries from pg_stat_statements.
// If normalize is true, queries are returned in their normalized form.
func (l *PostgresLibrary) PgStatStatements(ctx context.Context, topN int, _ bool) ([]PgStatStatement, error) {
	if topN <= 0 {
		topN = 20
	}

	query := `
		SELECT query, calls, total_exec_time, mean_exec_time, rows
		FROM pg_stat_statements
		ORDER BY total_exec_time DESC
		LIMIT $1
	`

	rows, err := l.client.DataStore().QueryContext(ctx, query, topN)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/postgres: pg_stat_statements: %w", err)
	}
	defer rows.Close()

	var stmts []PgStatStatement
	for rows.Next() {
		var s PgStatStatement
		if err := rows.Scan(&s.Query, &s.Calls, &s.TotalTimeMS, &s.MeanTimeMS, &s.Rows); err != nil {
			return nil, fmt.Errorf("gopgbase/postgres: pg_stat_statements scan: %w", err)
		}
		stmts = append(stmts, s)
	}

	return stmts, rows.Err()
}

// BlockingQuery represents a blocking/waiting query pair.
type BlockingQuery struct {
	BlockedPID    int    `json:"blocked_pid"`
	BlockedQuery  string `json:"blocked_query"`
	BlockingPID   int    `json:"blocking_pid"`
	BlockingQuery string `json:"blocking_query"`
}

// LockWatcher detects blocking queries in the database.
func (l *PostgresLibrary) LockWatcher(ctx context.Context) ([]BlockingQuery, error) {
	query := `
		SELECT
			blocked.pid AS blocked_pid,
			blocked.query AS blocked_query,
			blocking.pid AS blocking_pid,
			blocking.query AS blocking_query
		FROM pg_stat_activity blocked
		JOIN pg_locks blocked_locks ON blocked.pid = blocked_locks.pid
		JOIN pg_locks blocking_locks ON blocked_locks.locktype = blocking_locks.locktype
			AND blocked_locks.relation = blocking_locks.relation
			AND blocked_locks.pid != blocking_locks.pid
		JOIN pg_stat_activity blocking ON blocking_locks.pid = blocking.pid
		WHERE NOT blocked_locks.granted AND blocking_locks.granted
		LIMIT 50
	`

	rows, err := l.client.DataStore().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/postgres: lock watcher: %w", err)
	}
	defer rows.Close()

	var locks []BlockingQuery
	for rows.Next() {
		var bq BlockingQuery
		if err := rows.Scan(&bq.BlockedPID, &bq.BlockedQuery, &bq.BlockingPID, &bq.BlockingQuery); err != nil {
			return nil, fmt.Errorf("gopgbase/postgres: lock watcher scan: %w", err)
		}
		locks = append(locks, bq)
	}

	return locks, rows.Err()
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
