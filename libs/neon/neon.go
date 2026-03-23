// Package neon provides Neon serverless PostgreSQL-specific operations
// including database branching, compute scaling, connection pooler
// configuration, and pgvector support.
package neon

import (
	"context"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// NeonLibrary provides Neon-specific operations.
type NeonLibrary struct {
	client *gopgbase.Client
}

// NewNeonLibrary creates a new NeonLibrary backed by the given Client.
func NewNeonLibrary(client *gopgbase.Client) (*NeonLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/neon: client must not be nil")
	}
	return &NeonLibrary{client: client}, nil
}

// ServerlessScale returns the current compute endpoint scaling configuration.
// This queries Neon's internal configuration tables.
func (l *NeonLibrary) ServerlessScale(ctx context.Context) (map[string]any, error) {
	query := "SELECT current_setting('neon.max_cluster_size', true), current_setting('neon.min_cluster_size', true)"
	var maxSize, minSize string
	err := l.client.DataStore().QueryRowContext(ctx, query).Scan(&maxSize, &minSize)
	if err != nil {
		// Neon-specific settings may not be available; return gracefully.
		return map[string]any{
			"max_size": "unknown",
			"min_size": "unknown",
			"note":     "neon configuration settings not available via SQL",
		}, nil
	}
	return map[string]any{
		"max_size": maxSize,
		"min_size": minSize,
	}, nil
}

// Branch creates a database branch by creating a new schema that mirrors
// the current database state. Note: true Neon branching is an API operation;
// this provides a SQL-level approximation for development workflows.
func (l *NeonLibrary) Branch(ctx context.Context, branchName string) error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", quoteIdentifier(branchName))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/neon: create branch schema: %w", err)
	}
	return nil
}

// ScaleCompute sets the auto-suspend timeout for the Neon compute endpoint.
// This is primarily an API operation; this method sets the session-level
// idle timeout as an approximation.
func (l *NeonLibrary) ScaleCompute(ctx context.Context, autosuspendMinutes int) error {
	timeoutMS := autosuspendMinutes * 60 * 1000
	query := fmt.Sprintf("SET idle_in_transaction_session_timeout = '%d'", timeoutMS)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/neon: scale compute: %w", err)
	}
	return nil
}

// ConnectionPooler configures the connection pooler mode via session settings.
// poolerMode is typically "transaction" or "session".
func (l *NeonLibrary) ConnectionPooler(ctx context.Context, poolerMode string) error {
	// Neon's pgBouncer pooler is configured at the project level.
	// At the SQL level, we can set pool-friendly session parameters.
	if poolerMode == "transaction" {
		// In transaction mode, avoid session-level state.
		_, err := l.client.DataStore().ExecContext(ctx, "SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("gopgbase/neon: connection pooler: %w", err)
		}
	}
	return nil
}

// EnablePgVector enables the pgvector extension for vector similarity search.
func (l *NeonLibrary) EnablePgVector(ctx context.Context) error {
	_, err := l.client.DataStore().ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("gopgbase/neon: enable pgvector: %w", err)
	}
	return nil
}

// CreateVectorIndex creates an IVFFlat or HNSW index on a vector column.
//
// Parameters:
//   - table: table name
//   - column: vector column name
//   - indexType: "ivfflat" or "hnsw" (defaults to "hnsw")
//   - lists: number of lists for IVFFlat (ignored for HNSW)
func (l *NeonLibrary) CreateVectorIndex(ctx context.Context, table, column string, indexType ...string) error {
	itype := "hnsw"
	if len(indexType) > 0 && indexType[0] != "" {
		itype = indexType[0]
	}

	indexName := fmt.Sprintf("idx_%s_%s_%s", table, column, itype)
	var query string

	switch strings.ToLower(itype) {
	case "hnsw":
		query = fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s ON %s USING hnsw (%s vector_cosine_ops)",
			quoteIdentifier(indexName), quoteIdentifier(table), quoteIdentifier(column),
		)
	case "ivfflat":
		query = fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (%s vector_cosine_ops) WITH (lists = 100)",
			quoteIdentifier(indexName), quoteIdentifier(table), quoteIdentifier(column),
		)
	default:
		return fmt.Errorf("gopgbase/neon: unsupported vector index type: %s", itype)
	}

	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/neon: create vector index: %w", err)
	}
	return nil
}

// SimilaritySearch performs a vector similarity search using pgvector.
//
// Returns rows ordered by cosine distance to the query vector.
func (l *NeonLibrary) SimilaritySearch(ctx context.Context, table, vectorCol string, queryVector []float64, limit int) ([]map[string]any, error) {
	// Build vector string representation.
	parts := make([]string, len(queryVector))
	for i, v := range queryVector {
		parts[i] = fmt.Sprintf("%f", v)
	}
	vectorStr := "[" + strings.Join(parts, ",") + "]"

	query := fmt.Sprintf(
		"SELECT *, %s <=> $1::vector AS distance FROM %s ORDER BY distance LIMIT $2",
		quoteIdentifier(vectorCol), quoteIdentifier(table),
	)

	rows, err := l.client.DataStore().QueryContext(ctx, query, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/neon: similarity search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		scanDest := make([]any, len(cols))
		scanPtrs := make([]any, len(cols))
		for i := range scanDest {
			scanPtrs[i] = &scanDest[i]
		}
		if err := rows.Scan(scanPtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = scanDest[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
