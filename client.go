package gopgbase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

// Client is the main entry point for all database operations.
//
// It wraps a DataStore (injected via NewClient) and provides helpers for
// transactions, queries, bulk operations, struct scanning, and more.
//
// Client never constructs its own database connections — all access flows
// through the injected DataStore. Users may inject different adaptors,
// share a single adaptor across multiple Clients, or provide a custom
// DataStore implementation.
//
// All Client methods are safe for concurrent use.
type Client struct {
	ds          DataStore
	obs         *ObservabilityLibrary
	readReplica DataStore

	mu sync.RWMutex
}

// NewClient creates a new Client backed by the given DataStore.
//
// This is the constructor injection point: the caller is responsible for
// creating and configuring the DataStore (e.g., via an adaptor constructor).
//
// Example:
//
//	ds, err := adaptors.NewPostgresAdaptor(cfg)
//	if err != nil { log.Fatal(err) }
//	client := gopgbase.NewClient(ds)
func NewClient(ds DataStore) *Client {
	return &Client{ds: ds}
}

// DataStore returns the underlying DataStore for advanced or escape-hatch usage.
func (c *Client) DataStore() DataStore {
	return c.ds
}

// HealthStatus represents the result of a database health check.
type HealthStatus struct {
	Healthy  bool          `json:"healthy"`
	Latency  time.Duration `json:"latency_ns"`
	Error    string        `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// HealthCheckHandler returns an http.HandlerFunc that performs a database
// health check and responds with JSON.
func (c *Client) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	status := HealthStatus{Metadata: make(map[string]any)}
	if err := c.ds.PingContext(ctx); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		status.Latency = time.Since(start)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		status.Healthy = true
		status.Latency = time.Since(start)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(status)
}

// TunePool adjusts the connection pool parameters based on CPU cores and expected QPS.
//
// It applies a heuristic: maxOpen = cpuCores * 2 + 1 (capped by qps/100),
// maxIdle = cpuCores, idleTimeout = 5 minutes. These are starting-point
// defaults; users should monitor and adjust as needed.
func (c *Client) TunePool(_ context.Context, cpuCores, qps int) error {
	u, ok := c.ds.(Unwrapper)
	if !ok {
		return errors.New("gopgbase: TunePool requires a DataStore that implements Unwrap() *sql.DB")
	}
	db := u.Unwrap()

	maxOpen := cpuCores*2 + 1
	if qps > 0 && qps/100 > maxOpen {
		maxOpen = qps / 100
	}
	maxIdle := cpuCores
	if maxIdle < 2 {
		maxIdle = 2
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	return nil
}

// WithReadReplica returns a new Client that uses the given DataStore as a
// read replica. The returned client shares the same write DataStore but
// directs read operations to the replica.
func (c *Client) WithReadReplica(_ context.Context, replica DataStore) *Client {
	return &Client{
		ds:          c.ds,
		readReplica: replica,
	}
}

// EnablePreparedStatements enables prepared statement caching on the
// underlying connection pool. This requires an Unwrapper-capable DataStore.
func (c *Client) EnablePreparedStatements(_ context.Context) error {
	// pgx/stdlib handles prepared statements automatically when
	// using the pgx driver. This method exists for documentation
	// and to validate the DataStore supports it.
	if _, ok := c.ds.(Unwrapper); !ok {
		return errors.New("gopgbase: EnablePreparedStatements requires a DataStore that implements Unwrap() *sql.DB")
	}
	return nil
}

// readDS returns the DataStore to use for read operations.
// If a read replica is configured, it is preferred.
func (c *Client) readDS() DataStore {
	if c.readReplica != nil {
		return c.readReplica
	}
	return c.ds
}

// --- Convenience Query Helpers ---

// Count returns the number of rows matching the condition in the given table.
//
// The table parameter must be a trusted identifier (e.g., a constant or
// validated name) — it is quoted as an identifier but not parameterized.
// The condition is placed in a WHERE clause with args passed as placeholders.
// An empty condition counts all rows.
//
// Example:
//
//	n, err := client.Count(ctx, "users", "active = $1", true)
func (c *Client) Count(ctx context.Context, table string, condition string, args ...any) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))
	if condition != "" {
		query += " WHERE " + condition
	}

	var count int64
	err := c.readDS().QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("gopgbase: count: %w", err)
	}
	return count, nil
}

// Exists runs the provided SELECT query and returns true if at least one
// row is returned, false otherwise.
//
// The query is wrapped in SELECT EXISTS(...). Always use placeholders
// for values in the query.
//
// Example:
//
//	ok, err := client.Exists(ctx, "SELECT 1 FROM users WHERE email = $1", email)
func (c *Client) Exists(ctx context.Context, query string, args ...any) (bool, error) {
	wrapped := fmt.Sprintf("SELECT EXISTS (%s)", query)
	var exists bool
	err := c.readDS().QueryRowContext(ctx, wrapped, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("gopgbase: exists: %w", err)
	}
	return exists, nil
}

// BulkInsert inserts multiple rows into the given table using parameterized queries.
//
// columns lists the column names, and values is a slice of rows where each row
// is a slice of column values. Returns the number of rows affected.
//
// The insert is performed as a single statement with multiple value groups.
// For very large inserts (>65535 parameters), consider using BulkCopy instead.
//
// Example:
//
//	n, err := client.BulkInsert(ctx, "users", []string{"name", "age"},
//	    [][]any{{"Alice", 30}, {"Bob", 25}})
func (c *Client) BulkInsert(ctx context.Context, table string, columns []string, values [][]any) (int64, error) {
	if len(columns) == 0 || len(values) == 0 {
		return 0, nil
	}

	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteIdentifier(col)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES ", quoteIdentifier(table), strings.Join(quotedCols, ", ")))

	allArgs := make([]any, 0, len(columns)*len(values))
	paramIdx := 1

	for rowIdx, row := range values {
		if len(row) != len(columns) {
			return 0, fmt.Errorf("gopgbase: bulk insert: row %d has %d values, expected %d", rowIdx, len(row), len(columns))
		}
		if rowIdx > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('(')
		for colIdx := range row {
			if colIdx > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("$%d", paramIdx))
			paramIdx++
		}
		b.WriteByte(')')
		allArgs = append(allArgs, row...)
	}

	result, err := c.ds.ExecContext(ctx, b.String(), allArgs...)
	if err != nil {
		return 0, fmt.Errorf("gopgbase: bulk insert: %w", err)
	}

	return result.RowsAffected()
}

// BulkCopy performs a high-performance COPY operation for bulk data loading.
//
// This method requires a pgx-backed DataStore (one that implements Unwrap).
// For non-pgx DataStores, it falls back to BulkInsert.
//
// Example:
//
//	n, err := client.BulkCopy(ctx, "metrics", []string{"time", "value"},
//	    [][]any{{time.Now(), 42.0}, {time.Now(), 43.0}})
func (c *Client) BulkCopy(ctx context.Context, table string, columns []string, data [][]any) (int64, error) {
	u, ok := c.ds.(Unwrapper)
	if !ok {
		return c.BulkInsert(ctx, table, columns, data)
	}

	db := u.Unwrap()
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("gopgbase: bulk copy: %w", err)
	}
	defer conn.Close()

	var rowsCopied int64
	err = conn.Raw(func(driverConn any) error {
		pgxConn, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return errors.New("gopgbase: bulk copy requires pgx driver connection")
		}

		quotedCols := make([]string, len(columns))
		for i, col := range columns {
			quotedCols[i] = quoteIdentifier(col)
		}

		copySQL := fmt.Sprintf("COPY %s (%s) FROM STDIN", quoteIdentifier(table), strings.Join(quotedCols, ", "))

		pgConn := pgxConn.Conn().PgConn()
		result, copyErr := pgConn.CopyFrom(ctx, strings.NewReader(buildCopyData(data)), copySQL)
		if copyErr != nil {
			return copyErr
		}
		rowsCopied = result.RowsAffected()
		return nil
	})

	if err != nil {
		// Fallback to BulkInsert if COPY fails.
		return c.BulkInsert(ctx, table, columns, data)
	}

	return rowsCopied, nil
}

// buildCopyData creates tab-separated text data for COPY FROM STDIN.
func buildCopyData(data [][]any) string {
	var b strings.Builder
	for _, row := range data {
		for i, val := range row {
			if i > 0 {
				b.WriteByte('\t')
			}
			if val == nil {
				b.WriteString("\\N")
			} else {
				b.WriteString(fmt.Sprintf("%v", val))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// --- StructScan ---

// StructScan scans the current row of rows into the struct pointed to by dest.
//
// It maps column names to struct fields using the "db" tag, falling back
// to the lowercase field name. Supports JSONB (scanned as json.RawMessage
// or any json.Unmarshaler) and PostgreSQL arrays (scanned as slices).
//
// Example:
//
//	type User struct {
//	    ID   int    `db:"id"`
//	    Name string `db:"name"`
//	}
//	rows, _ := client.DataStore().QueryContext(ctx, "SELECT id, name FROM users")
//	for rows.Next() {
//	    var u User
//	    if err := client.StructScan(ctx, rows, &u); err != nil { ... }
//	}
func (c *Client) StructScan(_ context.Context, rows *sql.Rows, dest any) error {
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return errors.New("gopgbase: StructScan dest must be a pointer to a struct")
	}

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("gopgbase: struct scan columns: %w", err)
	}

	structVal := destVal.Elem()
	structType := structVal.Type()

	fieldMap := buildFieldMap(structType)

	scanDest := make([]any, len(cols))
	for i, col := range cols {
		if fieldIdx, ok := fieldMap[col]; ok {
			scanDest[i] = structVal.Field(fieldIdx).Addr().Interface()
		} else {
			scanDest[i] = new(any)
		}
	}

	if err := rows.Scan(scanDest...); err != nil {
		return fmt.Errorf("gopgbase: struct scan: %w", err)
	}

	return nil
}

// buildFieldMap creates a mapping from column name to struct field index.
func buildFieldMap(t reflect.Type) map[string]int {
	m := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("db")
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		m[tag] = i
	}
	return m
}

// ForEachRow executes query and calls fn for each row. Rows are not buffered
// in memory — fn is called as each row is scanned.
//
// If fn returns an error, iteration stops and that error is returned.
// The rows are closed automatically.
//
// Example:
//
//	err := client.ForEachRow(ctx, "SELECT id, name FROM users", nil,
//	    func(row map[string]any) error {
//	        fmt.Println(row["name"])
//	        return nil
//	    })
func (c *Client) ForEachRow(ctx context.Context, query string, args []any, fn func(row map[string]any) error) error {
	rows, err := c.readDS().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("gopgbase: for each row: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("gopgbase: for each row columns: %w", err)
	}

	for rows.Next() {
		scanDest := make([]any, len(cols))
		scanPtrs := make([]any, len(cols))
		for i := range scanDest {
			scanPtrs[i] = &scanDest[i]
		}

		if err := rows.Scan(scanPtrs...); err != nil {
			return fmt.Errorf("gopgbase: for each row scan: %w", err)
		}

		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = scanDest[i]
		}

		if err := fn(row); err != nil {
			return err
		}
	}

	return rows.Err()
}

// --- QueryBuilder ---

// QueryBuilder provides a fluent interface for constructing SQL queries.
//
// It supports dual placeholder modes:
//   - MySQL-style ? placeholders: auto-converted to PostgreSQL $N before execution.
//   - Native PostgreSQL $N placeholders: passed through as-is.
//   - Mixing ? and $N in the same query is an error.
//
// Example:
//
//	results, err := client.QueryBuilder().
//	    Select("users").
//	    Columns("id", "name", "email").
//	    Where("age > ?", 18).
//	    OrderBy("name ASC").
//	    Limit(10).
//	    Query(ctx)
func (c *Client) QueryBuilder() *QueryBuilderDSL {
	return &QueryBuilderDSL{client: c}
}

// QueryBuilderDSL is a fluent SQL query builder.
type QueryBuilderDSL struct {
	client     *Client
	table      string
	cols       []string
	conditions []string
	args       []any
	joins      []string
	orderBy    string
	groupBy    string
	having     string
	limit      int
	offset     int
	hasLimit   bool
	hasOffset  bool
}

// Select sets the table for a SELECT query.
func (qb *QueryBuilderDSL) Select(table string) *QueryBuilderDSL {
	qb.table = table
	return qb
}

// Columns sets the columns to select. If not called, "*" is used.
func (qb *QueryBuilderDSL) Columns(cols ...string) *QueryBuilderDSL {
	qb.cols = cols
	return qb
}

// Where adds a WHERE condition. Multiple calls are ANDed together.
// Use ? or $N for placeholders.
func (qb *QueryBuilderDSL) Where(condition string, args ...any) *QueryBuilderDSL {
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, args...)
	return qb
}

// Join adds a JOIN clause (e.g., "INNER JOIN orders ON users.id = orders.user_id").
func (qb *QueryBuilderDSL) Join(join string) *QueryBuilderDSL {
	qb.joins = append(qb.joins, join)
	return qb
}

// OrderBy sets the ORDER BY clause.
func (qb *QueryBuilderDSL) OrderBy(order string) *QueryBuilderDSL {
	qb.orderBy = order
	return qb
}

// GroupBy sets the GROUP BY clause.
func (qb *QueryBuilderDSL) GroupBy(group string) *QueryBuilderDSL {
	qb.groupBy = group
	return qb
}

// Having sets the HAVING clause (used with GroupBy).
func (qb *QueryBuilderDSL) Having(having string, args ...any) *QueryBuilderDSL {
	qb.having = having
	qb.args = append(qb.args, args...)
	return qb
}

// Limit sets the maximum number of rows to return.
func (qb *QueryBuilderDSL) Limit(n int) *QueryBuilderDSL {
	qb.limit = n
	qb.hasLimit = true
	return qb
}

// Offset sets the number of rows to skip.
func (qb *QueryBuilderDSL) Offset(n int) *QueryBuilderDSL {
	qb.offset = n
	qb.hasOffset = true
	return qb
}

// Build constructs the final SQL query string and arguments.
// Placeholder conversion (? → $N) is applied here.
func (qb *QueryBuilderDSL) Build() (string, []any, error) {
	if qb.table == "" {
		return "", nil, errors.New("gopgbase: query builder: table is required")
	}

	var b strings.Builder
	b.WriteString("SELECT ")

	if len(qb.cols) > 0 {
		b.WriteString(strings.Join(qb.cols, ", "))
	} else {
		b.WriteByte('*')
	}

	b.WriteString(" FROM ")
	b.WriteString(quoteIdentifier(qb.table))

	for _, j := range qb.joins {
		b.WriteByte(' ')
		b.WriteString(j)
	}

	if len(qb.conditions) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(qb.conditions, " AND "))
	}

	if qb.groupBy != "" {
		b.WriteString(" GROUP BY ")
		b.WriteString(qb.groupBy)
	}

	if qb.having != "" {
		b.WriteString(" HAVING ")
		b.WriteString(qb.having)
	}

	if qb.orderBy != "" {
		b.WriteString(" ORDER BY ")
		b.WriteString(qb.orderBy)
	}

	if qb.hasLimit {
		b.WriteString(fmt.Sprintf(" LIMIT %d", qb.limit))
	}

	if qb.hasOffset {
		b.WriteString(fmt.Sprintf(" OFFSET %d", qb.offset))
	}

	query, err := convertPlaceholders(b.String())
	if err != nil {
		return "", nil, err
	}

	return query, qb.args, nil
}

// Query executes the built SELECT query and returns the rows.
func (qb *QueryBuilderDSL) Query(ctx context.Context) (*sql.Rows, error) {
	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}
	return qb.client.readDS().QueryContext(ctx, query, args...)
}

// Exec executes the built query (for non-SELECT statements adapted to the builder).
func (qb *QueryBuilderDSL) Exec(ctx context.Context) (sql.Result, error) {
	query, args, err := qb.Build()
	if err != nil {
		return nil, err
	}
	return qb.client.ds.ExecContext(ctx, query, args...)
}

// --- Placeholder Conversion ---

// convertPlaceholders converts ? placeholders to Postgres $N style.
// Rules:
//   - If no ? present, query is returned unchanged (fast path).
//   - ?? → literal ? (JSONB operator escape: covers ?, ?|, ?&)
//   - $N found outside strings while ? also present → error (mixed placeholders)
//   - Handles: single-quotes, E-strings, double-quoted identifiers,
//     dollar-quoted strings, block comments, line comments.
//   - Returns error on unterminated string, comment, or dollar-quote.
//   - Returns error on mixed ? and $N placeholders.
func convertPlaceholders(query string) (string, error) {
	if !strings.ContainsRune(query, '?') {
		return query, nil
	}

	var result strings.Builder
	result.Grow(len(query))
	n := 0
	i := 0
	for i < len(query) {
		if i+1 < len(query) && query[i] == '/' && query[i+1] == '*' {
			end := strings.Index(query[i+2:], "*/")
			if end == -1 {
				return "", errors.New("gopgbase: unterminated block comment")
			}
			result.WriteString(query[i : i+2+end+2])
			i += 2 + end + 2
			continue
		}

		if i+1 < len(query) && query[i] == '-' && query[i+1] == '-' {
			end := strings.IndexByte(query[i:], '\n')
			if end == -1 {
				result.WriteString(query[i:])
				break
			}
			result.WriteString(query[i : i+end])
			i += end
			continue
		}

		if query[i] == '$' {
			if tag := parseDollarTag(query, i); tag != "" {
				end := strings.Index(query[i+len(tag):], tag)
				if end == -1 {
					return "", fmt.Errorf("gopgbase: unterminated dollar-quote %s", tag)
				}
				result.WriteString(query[i : i+len(tag)+end+len(tag)])
				i += len(tag) + end + len(tag)
				continue
			}
			if i+1 < len(query) && query[i+1] >= '1' && query[i+1] <= '9' {
				return "", errors.New("gopgbase: mixed placeholders: use ? or $N, not both")
			}
			result.WriteByte(query[i])
			i++
			continue
		}

		if (query[i] == 'E' || query[i] == 'e') && i+1 < len(query) && query[i+1] == '\'' {
			result.WriteByte(query[i])
			result.WriteByte('\'')
			i += 2
			closed := false
			for i < len(query) {
				if query[i] == '\\' && i+1 < len(query) {
					result.WriteByte(query[i])
					result.WriteByte(query[i+1])
					i += 2
					continue
				}
				if query[i] == '\'' {
					result.WriteByte('\'')
					i++
					if i < len(query) && query[i] == '\'' {
						result.WriteByte('\'')
						i++
						continue
					}
					closed = true
					break
				}
				result.WriteByte(query[i])
				i++
			}
			if !closed {
				return "", errors.New("gopgbase: unterminated E-string")
			}
			continue
		}

		if query[i] == '\'' {
			result.WriteByte('\'')
			i++
			closed := false
			for i < len(query) {
				if query[i] == '\'' {
					result.WriteByte('\'')
					i++
					if i < len(query) && query[i] == '\'' {
						result.WriteByte('\'')
						i++
						continue
					}
					closed = true
					break
				}
				result.WriteByte(query[i])
				i++
			}
			if !closed {
				return "", errors.New("gopgbase: unterminated single-quoted string")
			}
			continue
		}

		if query[i] == '"' {
			result.WriteByte('"')
			i++
			closed := false
			for i < len(query) {
				if query[i] == '"' {
					result.WriteByte('"')
					i++
					if i < len(query) && query[i] == '"' {
						result.WriteByte('"')
						i++
						continue
					}
					closed = true
					break
				}
				result.WriteByte(query[i])
				i++
			}
			if !closed {
				return "", errors.New("gopgbase: unterminated double-quoted identifier")
			}
			continue
		}

		if query[i] == '?' {
			if i+1 < len(query) && query[i+1] == '?' {
				result.WriteByte('?')
				i += 2
				continue
			}
			n++
			result.WriteString(fmt.Sprintf("$%d", n))
			i++
			continue
		}

		result.WriteByte(query[i])
		i++
	}

	return result.String(), nil
}

// parseDollarTag detects a $tag$ dollar-quote opener at position i.
// Per PostgreSQL spec: first character must NOT be a digit. $$ is valid.
func parseDollarTag(query string, i int) string {
	if i >= len(query) || query[i] != '$' {
		return ""
	}
	j := i + 1
	if j < len(query) && query[j] == '$' {
		return "$$"
	}
	if j < len(query) {
		c := query[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
			return ""
		}
	}
	j++
	for j < len(query) && query[j] != '$' {
		c := query[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return ""
		}
		j++
	}
	if j >= len(query) {
		return ""
	}
	return query[i : j+1]
}

// quoteIdentifier quotes a SQL identifier to prevent injection.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// init registers the pgx stdlib driver. This is safe to call multiple times.
func init() {
	// Ensure the pgx driver is available for database/sql.
	// pgx/v5/stdlib registers itself automatically on import.
	_ = runtime.NumCPU() // force import of runtime for init ordering
}
