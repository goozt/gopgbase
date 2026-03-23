package common

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	gopgbase "github.com/goozt/gopgbase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLib(t *testing.T) (*CommonLibrary, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := gopgbase.NewClient(db)
	lib, err := NewCommonLibrary(client)
	require.NoError(t, err)
	return lib, mock
}

func TestNewCommonLibrary_NilClient(t *testing.T) {
	_, err := NewCommonLibrary(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client must not be nil")
}

// --- Pagination ---

func TestPagination_Success(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT \* FROM "users" ORDER BY id LIMIT 10 OFFSET 0`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "alice").
			AddRow(2, "bob"))

	results, err := lib.Pagination(context.Background(), "users", 1, 10, "id", "")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, int64(1), results[0]["id"])
	assert.Equal(t, "bob", results[1]["name"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagination_WithWhere(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE active = \$1 ORDER BY id LIMIT 20 OFFSET 20`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	results, err := lib.Pagination(context.Background(), "users", 2, 20, "id", "active = $1", true)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagination_DefaultsForInvalidPageAndPerPage(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`LIMIT 20 OFFSET 0`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	results, err := lib.Pagination(context.Background(), "users", 0, -1, "", "")
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPagination_QueryError(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db error"))

	_, err := lib.Pagination(context.Background(), "users", 1, 10, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagination")
}

// --- SoftDelete ---

func TestSoftDelete_Success(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectExec(`UPDATE "users" SET "deleted_at" = NOW\(\) WHERE id = \$1`).
		WithArgs(42).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := lib.SoftDelete(context.Background(), "users", 42, "")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSoftDelete_CustomColumn(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectExec(`UPDATE "users" SET "removed_at" = NOW\(\) WHERE id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := lib.SoftDelete(context.Background(), "users", 1, "removed_at")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSoftDelete_ExecError(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectExec(`UPDATE`).WillReturnError(errors.New("db error"))

	err := lib.SoftDelete(context.Background(), "users", 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "soft delete")
}

// --- SchemaDiff ---

func TestSchemaDiff_MissingTable(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}))

	changes, err := lib.SchemaDiff(context.Background(), map[string]map[string]string{
		"users": {"id": "integer"},
	})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "missing_table", changes[0].ChangeType)
	assert.Equal(t, "users", changes[0].Table)
}

func TestSchemaDiff_MissingColumn(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT column_name, data_type`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
			AddRow("id", "integer"))

	changes, err := lib.SchemaDiff(context.Background(), map[string]map[string]string{
		"users": {"id": "integer", "email": "text"},
	})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "missing_column", changes[0].ChangeType)
	assert.Equal(t, "email", changes[0].Column)
}

func TestSchemaDiff_TypeMismatch(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT column_name, data_type`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
			AddRow("id", "bigint"))

	changes, err := lib.SchemaDiff(context.Background(), map[string]map[string]string{
		"users": {"id": "integer"},
	})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "type_mismatch", changes[0].ChangeType)
	assert.Equal(t, "integer", changes[0].Expected)
	assert.Equal(t, "bigint", changes[0].Actual)
}

func TestSchemaDiff_NoChanges(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT column_name, data_type`).
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
			AddRow("id", "integer").
			AddRow("name", "text"))

	changes, err := lib.SchemaDiff(context.Background(), map[string]map[string]string{
		"users": {"id": "integer", "name": "text"},
	})
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestSchemaDiff_QueryError(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT column_name`).WillReturnError(errors.New("db error"))

	_, err := lib.SchemaDiff(context.Background(), map[string]map[string]string{
		"users": {"id": "integer"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema diff")
}

// --- Prefetch ---

func TestPrefetch_Success(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE "user_id" IN \(\$1, \$2\)`).
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).
			AddRow(10, 1).
			AddRow(11, 2))

	result, err := lib.Prefetch(context.Background(), "users", []any{1, 2}, map[string]string{
		"orders": "orders.user_id",
	})
	require.NoError(t, err)
	assert.Len(t, result["orders"], 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrefetch_InvalidRelation(t *testing.T) {
	lib, _ := newTestLib(t)

	_, err := lib.Prefetch(context.Background(), "users", []any{1}, map[string]string{
		"bad": "no_dot_here",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid relation")
}

func TestPrefetch_QueryError(t *testing.T) {
	lib, mock := newTestLib(t)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db error"))

	_, err := lib.Prefetch(context.Background(), "users", []any{1}, map[string]string{
		"orders": "orders.user_id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefetch")
}

// --- scanRowsToMaps ---

func TestScanRowsToMaps_Empty(t *testing.T) {
	_, mock := newTestLib(t)
	rows := sqlmock.NewRows([]string{"id"})

	db, _, _ := sqlmock.New()
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// Use the mock's db to get actual *sql.Rows
	lib, mock2 := newTestLib(t)
	_ = lib
	mock2.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	sqlRows, err := lib.client.DataStore().QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)
	defer func() { _ = sqlRows.Close() }()

	results, err := scanRowsToMaps(sqlRows)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- quoteIdentifier ---

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"users", `"users"`},
		{`user"name`, `"user""name"`},
		{"", `""`},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, quoteIdentifier(tt.input))
		})
	}
}
