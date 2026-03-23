// Package common provides shared utility functions that work across all
// PostgreSQL-compatible adaptors in gopgbase.
package common

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// CommonLibrary provides cross-provider convenience operations
// for pagination, soft-delete, audit trails, schema inspection, and more.
type CommonLibrary struct {
	client *gopgbase.Client
}

// NewCommonLibrary creates a new CommonLibrary backed by the given Client.
func NewCommonLibrary(client *gopgbase.Client) (*CommonLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/common: client must not be nil")
	}
	return &CommonLibrary{client: client}, nil
}

// Pagination returns paginated results from a table as a slice of maps.
//
// Parameters:
//   - table: table name (must be a trusted identifier)
//   - page: 1-based page number
//   - perPage: rows per page
//   - orderBy: ORDER BY clause (e.g., "created_at DESC")
//   - where: optional WHERE condition with placeholders
//   - args: placeholder values
func (l *CommonLibrary) Pagination(ctx context.Context, table string, page, perPage int, orderBy, where string, args ...any) ([]map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	offset := (page - 1) * perPage
	query := fmt.Sprintf("SELECT * FROM %s", quoteIdentifier(table))

	if where != "" {
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", perPage, offset)

	rows, err := l.client.DataStore().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/common: pagination: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanRowsToMaps(rows)
}

// SoftDelete marks a row as deleted by setting a soft-delete column
// (e.g., "deleted_at") to the current timestamp.
//
// The softDeleteCol defaults to "deleted_at" if empty.
func (l *CommonLibrary) SoftDelete(ctx context.Context, table string, id any, softDeleteCol string) error {
	if softDeleteCol == "" {
		softDeleteCol = "deleted_at"
	}
	query := fmt.Sprintf(
		"UPDATE %s SET %s = NOW() WHERE id = $1",
		quoteIdentifier(table), quoteIdentifier(softDeleteCol),
	)
	_, err := l.client.DataStore().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("gopgbase/common: soft delete: %w", err)
	}
	return nil
}

// AuditTrail creates an audit trigger on the given table that logs
// all INSERT, UPDATE, and DELETE operations to the specified audit table.
//
// The audit table is created automatically if it does not exist.
func (l *CommonLibrary) AuditTrail(ctx context.Context, table, auditTable string) error {
	if auditTable == "" {
		auditTable = table + "_audit"
	}

	createAuditTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			operation TEXT NOT NULL,
			table_name TEXT NOT NULL,
			old_data JSONB,
			new_data JSONB,
			changed_at TIMESTAMPTZ DEFAULT NOW(),
			changed_by TEXT
		)
	`, quoteIdentifier(auditTable))

	funcName := fmt.Sprintf("audit_%s_fn", table)
	triggerName := fmt.Sprintf("audit_%s_trigger", table)

	createFunction := fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %s() RETURNS TRIGGER AS $body$
		BEGIN
			IF TG_OP = 'DELETE' THEN
				INSERT INTO %s (operation, table_name, old_data)
				VALUES ('DELETE', TG_TABLE_NAME, row_to_json(OLD)::jsonb);
				RETURN OLD;
			ELSIF TG_OP = 'UPDATE' THEN
				INSERT INTO %s (operation, table_name, old_data, new_data)
				VALUES ('UPDATE', TG_TABLE_NAME, row_to_json(OLD)::jsonb, row_to_json(NEW)::jsonb);
				RETURN NEW;
			ELSIF TG_OP = 'INSERT' THEN
				INSERT INTO %s (operation, table_name, new_data)
				VALUES ('INSERT', TG_TABLE_NAME, row_to_json(NEW)::jsonb);
				RETURN NEW;
			END IF;
			RETURN NULL;
		END;
		$body$ LANGUAGE plpgsql
	`, quoteIdentifier(funcName),
		quoteIdentifier(auditTable),
		quoteIdentifier(auditTable),
		quoteIdentifier(auditTable))

	createTrigger := fmt.Sprintf(`
		DROP TRIGGER IF EXISTS %s ON %s;
		CREATE TRIGGER %s
		AFTER INSERT OR UPDATE OR DELETE ON %s
		FOR EACH ROW EXECUTE FUNCTION %s()
	`, quoteIdentifier(triggerName), quoteIdentifier(table),
		quoteIdentifier(triggerName), quoteIdentifier(table),
		quoteIdentifier(funcName))

	for _, stmt := range []string{createAuditTable, createFunction, createTrigger} {
		if _, err := l.client.DataStore().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("gopgbase/common: audit trail: %w", err)
		}
	}

	return nil
}

// SchemaChange represents a difference between expected and actual schema.
type SchemaChange struct {
	Table      string `json:"table"`
	Column     string `json:"column"`
	ChangeType string `json:"change_type"` // "missing_table", "missing_column", "type_mismatch"
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
}

// SchemaDiff compares the current database schema against expected schema
// definitions and returns a list of differences.
//
// expectedSchema maps table names to column definitions:
//
//	map[string]map[string]string{
//	    "users": {"id": "integer", "name": "text"},
//	}
func (l *CommonLibrary) SchemaDiff(ctx context.Context, expectedSchema map[string]map[string]string) ([]SchemaChange, error) {
	var changes []SchemaChange

	for table, expectedCols := range expectedSchema {
		rows, err := l.client.DataStore().QueryContext(ctx,
			`SELECT column_name, data_type FROM information_schema.columns
			 WHERE table_name = $1 AND table_schema = 'public'`, table)
		if err != nil {
			return nil, fmt.Errorf("gopgbase/common: schema diff: %w", err)
		}

		actualCols := make(map[string]string)
		for rows.Next() {
			var col, dtype string
			if err := rows.Scan(&col, &dtype); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("gopgbase/common: schema diff scan: %w", err)
			}
			actualCols[col] = dtype
		}
		_ = rows.Close()

		if len(actualCols) == 0 {
			changes = append(changes, SchemaChange{
				Table:      table,
				ChangeType: "missing_table",
				Expected:   table,
			})
			continue
		}

		for col, expectedType := range expectedCols {
			actualType, exists := actualCols[col]
			if !exists {
				changes = append(changes, SchemaChange{
					Table:      table,
					Column:     col,
					ChangeType: "missing_column",
					Expected:   expectedType,
				})
			} else if actualType != expectedType {
				changes = append(changes, SchemaChange{
					Table:      table,
					Column:     col,
					ChangeType: "type_mismatch",
					Expected:   expectedType,
					Actual:     actualType,
				})
			}
		}
	}

	return changes, nil
}

// Prefetch loads related records to prevent N+1 query patterns.
//
// It takes a main table query result and eagerly loads records from
// related tables based on the provided relations.
//
// Relations format: map of relation name → "related_table.foreign_key"
func (l *CommonLibrary) Prefetch(ctx context.Context, mainTable string, ids []any, relations map[string]string) (map[string][]map[string]any, error) {
	result := make(map[string][]map[string]any)

	for name, relation := range relations {
		parts := strings.SplitN(relation, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("gopgbase/common: prefetch: invalid relation %q, expected 'table.column'", relation)
		}
		relTable, relCol := parts[0], parts[1]

		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
			quoteIdentifier(relTable), quoteIdentifier(relCol),
			strings.Join(placeholders, ", "))

		rows, err := l.client.DataStore().QueryContext(ctx, query, ids...)
		if err != nil {
			return nil, fmt.Errorf("gopgbase/common: prefetch %s: %w", name, err)
		}

		maps, err := scanRowsToMaps(rows)
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		result[name] = maps
	}

	return result, nil
}

// scanRowsToMaps scans all rows into a slice of maps.
func scanRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("gopgbase/common: scan columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		scanDest := make([]any, len(cols))
		scanPtrs := make([]any, len(cols))
		for i := range scanDest {
			scanPtrs[i] = &scanDest[i]
		}
		if err := rows.Scan(scanPtrs...); err != nil {
			return nil, fmt.Errorf("gopgbase/common: scan row: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = scanDest[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

// quoteIdentifier quotes a SQL identifier to prevent injection.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
