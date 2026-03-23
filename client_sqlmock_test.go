package gopgbase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unwrapDB wraps *sql.DB to satisfy both DataStore and Unwrapper.
type unwrapDB struct {
	*sql.DB
}

func (u *unwrapDB) Unwrap() *sql.DB { return u.DB }

func newUnwrapClient(t *testing.T) (*Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewClient(&unwrapDB{db}), mock
}

func newPingClient(t *testing.T) (*Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.NewWithDSN("sqlmock_ping", sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewClient(db), mock
}

func TestHealthCheckHandler_Healthy(t *testing.T) {
	client, mock := newPingClient(t)
	mock.ExpectPing()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	client.HealthCheckHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"healthy":true`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthCheckHandler_Unhealthy(t *testing.T) {
	client, mock := newPingClient(t)
	mock.ExpectPing().WillReturnError(errors.New("db down"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	client.HealthCheckHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"healthy":false`)
	assert.Contains(t, rec.Body.String(), `db down`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTunePool_WithUnwrapper(t *testing.T) {
	client, _ := newUnwrapClient(t)

	err := client.TunePool(context.Background(), 4, 1000)
	require.NoError(t, err)
}

func TestTunePool_HighQPS(t *testing.T) {
	client, _ := newUnwrapClient(t)

	err := client.TunePool(context.Background(), 2, 50000)
	require.NoError(t, err)
}

func TestTunePool_LowCores(t *testing.T) {
	client, _ := newUnwrapClient(t)

	err := client.TunePool(context.Background(), 1, 0)
	require.NoError(t, err)
}

func TestEnablePreparedStatements_WithUnwrapper(t *testing.T) {
	client, _ := newUnwrapClient(t)

	err := client.EnablePreparedStatements(context.Background())
	require.NoError(t, err)
}

func TestCount_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	n, err := client.Count(context.Background(), "users", "")
	require.NoError(t, err)
	assert.Equal(t, int64(42), n)
}

func TestCount_WithCondition(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "users" WHERE active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	n, err := client.Count(context.Background(), "users", "active = $1", true)
	require.NoError(t, err)
	assert.Equal(t, int64(10), n)
}

func TestCount_Error(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT COUNT`).WillReturnError(errors.New("query failed"))

	_, err := client.Count(context.Background(), "users", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count")
}

func TestExists_True(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("alice@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	ok, err := client.Exists(context.Background(), "SELECT 1 FROM users WHERE email = $1", "alice@example.com")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestExists_False(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT EXISTS`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	ok, err := client.Exists(context.Background(), "SELECT 1 FROM users WHERE id = $1", 999)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestExists_Error(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT EXISTS`).WillReturnError(errors.New("query failed"))

	_, err := client.Exists(context.Background(), "SELECT 1 FROM users", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exists")
}

func TestBulkInsert_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectExec(`INSERT INTO "users"`).
		WithArgs("alice", 30, "bob", 25).
		WillReturnResult(sqlmock.NewResult(0, 2))

	n, err := client.BulkInsert(context.Background(), "users",
		[]string{"name", "age"},
		[][]any{{"alice", 30}, {"bob", 25}})

	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestBulkInsert_ExecError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectExec(`INSERT INTO`).WillReturnError(errors.New("insert failed"))

	_, err := client.BulkInsert(context.Background(), "t",
		[]string{"a"}, [][]any{{1}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk insert")
}

func TestBulkCopy_FallbackToInsert(t *testing.T) {
	// mockDataStore doesn't implement Unwrapper, so BulkCopy falls back.
	ds := &mockDataStore{
		execResult: sqlmock.NewResult(0, 1),
	}
	client := NewClient(ds)

	n, err := client.BulkCopy(context.Background(), "t",
		[]string{"a"}, [][]any{{1}})

	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestStructScan_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice"))

	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	rows, err := client.DataStore().QueryContext(context.Background(), "SELECT id, name FROM users")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next())
	var u User
	err = client.StructScan(context.Background(), rows, &u)
	require.NoError(t, err)
	assert.Equal(t, 1, u.ID)
	assert.Equal(t, "alice", u.Name)
}

func TestStructScan_NotPointer(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	type User struct{ ID int }
	rows, err := client.DataStore().QueryContext(context.Background(), "SELECT id FROM users")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next())
	err = client.StructScan(context.Background(), rows, User{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pointer to a struct")
}

func TestStructScan_UnmappedColumn(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "extra"}).AddRow(1, "ignored"))

	type User struct {
		ID int `db:"id"`
	}

	rows, err := client.DataStore().QueryContext(context.Background(), "SELECT id, extra FROM users")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next())
	var u User
	err = client.StructScan(context.Background(), rows, &u)
	require.NoError(t, err)
	assert.Equal(t, 1, u.ID)
}

func TestForEachRow_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "alice").
			AddRow(2, "bob"))

	var names []string
	err := client.ForEachRow(context.Background(), "SELECT id, name FROM users", nil,
		func(row map[string]any) error {
			names = append(names, row["name"].(string))
			return nil
		})

	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, names)
}

func TestForEachRow_FnError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := client.ForEachRow(context.Background(), "SELECT id FROM t", nil,
		func(_ map[string]any) error {
			return errors.New("stop")
		})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop")
}

func TestForEachRow_QueryError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("query failed"))

	err := client.ForEachRow(context.Background(), "SELECT 1", nil,
		func(_ map[string]any) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "for each row")
}

func TestQueryBuilder_Query(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT \* FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := client.QueryBuilder().Select("users").Query(context.Background())
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	assert.True(t, rows.Next())
}

func TestQueryBuilder_Query_Error(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectQuery(`SELECT \* FROM "users"`).
		WillReturnError(fmt.Errorf("db error"))

	rows, err := client.QueryBuilder().Query(context.Background())
	require.Error(t, err)
	assert.Nil(t, rows)
}

func TestQueryBuilder_Exec(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectExec(`SELECT \* FROM "users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := client.QueryBuilder().Select("users").Exec(context.Background())
	require.NoError(t, err)
}

func TestQueryBuilder_Exec_Error(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectExec(`SELECT \* FROM "users"`).
		WillReturnError(fmt.Errorf("db error"))

	_, err := client.QueryBuilder().Exec(context.Background())
	require.Error(t, err)
}
