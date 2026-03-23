package gopgbase

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDataStore implements DataStore for testing.
type mockDataStore struct {
	pingErr    error
	closeErr   error
	beginErr   error
	execErr    error
	queryErr   error
	execResult sql.Result
	rows       *sql.Rows
	tx         *sql.Tx
}

func (m *mockDataStore) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	return nil
}

func (m *mockDataStore) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return m.rows, m.queryErr
}

func (m *mockDataStore) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return m.execResult, m.execErr
}

func (m *mockDataStore) BeginTx(_ context.Context, _ *sql.TxOptions) (*sql.Tx, error) {
	return m.tx, m.beginErr
}

func (m *mockDataStore) PingContext(_ context.Context) error {
	return m.pingErr
}

func (m *mockDataStore) Close() error {
	return m.closeErr
}

func TestNewClient(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	assert.NotNil(t, client)
	assert.Equal(t, ds, client.DataStore())
}

func TestClient_DataStore(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)
	assert.Same(t, ds, client.DataStore())
}

func TestClient_Transaction_BeginError(t *testing.T) {
	ds := &mockDataStore{
		beginErr: errors.New("begin failed"),
	}
	client := NewClient(ds)

	err := client.Transaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin tx")
}

func TestClient_ReadOnlyTransaction_BeginError(t *testing.T) {
	ds := &mockDataStore{
		beginErr: errors.New("begin failed"),
	}
	client := NewClient(ds)

	err := client.ReadOnlyTransaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin tx")
}

func TestClient_TransactionWithIsolation_BeginError(t *testing.T) {
	ds := &mockDataStore{
		beginErr: errors.New("begin failed"),
	}
	client := NewClient(ds)

	err := client.TransactionWithIsolation(context.Background(), sql.LevelSerializable, func(_ *sql.Tx) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin tx")
}

func TestClient_BatchTransaction_BeginError(t *testing.T) {
	ds := &mockDataStore{
		beginErr: errors.New("begin failed"),
	}
	client := NewClient(ds)

	err := client.BatchTransaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.Error(t, err)
}

func TestClient_TunePool_NoUnwrapper(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	err := client.TunePool(context.Background(), 4, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unwrap")
}

func TestClient_EnablePreparedStatements_NoUnwrapper(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	err := client.EnablePreparedStatements(context.Background())
	require.Error(t, err)
}

func TestClient_WithReadReplica(t *testing.T) {
	ds := &mockDataStore{}
	replica := &mockDataStore{}
	client := NewClient(ds)

	replicaClient := client.WithReadReplica(context.Background(), replica)
	assert.NotNil(t, replicaClient)
	assert.Equal(t, ds, replicaClient.ds)
	assert.Equal(t, replica, replicaClient.readReplica)
	assert.Same(t, replica, replicaClient.readDS())
}

func TestClient_ReadDS_NoReplica(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	assert.Same(t, ds, client.readDS())
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", `"users"`},
		{`user"name`, `"user""name"`},
		{"", `""`},
		{"public.users", `"public.users"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, quoteIdentifier(tt.input))
		})
	}
}

func TestBuildFieldMap(t *testing.T) {
	type testStruct struct {
		Name    string `db:"name"`
		Ignored string `db:"-"`
		NoTag   string
		private string //nolint:unused // testing that private fields are skipped
		ID      int    `db:"id"`
	}

	fm := buildFieldMap(reflect.TypeOf(testStruct{}))

	assert.Equal(t, 4, fm["id"])
	assert.Equal(t, 0, fm["name"])
	_, hasIgnored := fm["-"]
	assert.False(t, hasIgnored)
	assert.Equal(t, 2, fm["notag"])
}

func TestBuildCopyData(t *testing.T) {
	data := [][]any{
		{1, "alice", nil},
		{2, "bob", 42},
	}

	result := buildCopyData(data)
	assert.Contains(t, result, "1\talice\t\\N\n")
	assert.Contains(t, result, "2\tbob\t42\n")
}

// --- QueryBuilder tests ---

func TestQueryBuilder_Build_NoTable(t *testing.T) {
	client := NewClient(&mockDataStore{})
	_, _, err := client.QueryBuilder().Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table is required")
}

func TestQueryBuilder_Build_Simple(t *testing.T) {
	client := NewClient(&mockDataStore{})

	query, args, err := client.QueryBuilder().
		Select("users").
		Build()

	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "users"`, query)
	assert.Empty(t, args)
}

func TestQueryBuilder_Build_AllClauses(t *testing.T) {
	client := NewClient(&mockDataStore{})

	query, args, err := client.QueryBuilder().
		Select("users").
		Columns("id", "name").
		Join("INNER JOIN orders ON users.id = orders.user_id").
		Where("age > $1", 18).
		Where("active = $2", true).
		GroupBy("name").
		Having("COUNT(*) > $3", 5).
		OrderBy("name ASC").
		Limit(10).
		Offset(20).
		Build()

	require.NoError(t, err)
	assert.Contains(t, query, `SELECT id, name FROM "users"`)
	assert.Contains(t, query, "INNER JOIN orders")
	assert.Contains(t, query, "WHERE age > $1 AND active = $2")
	assert.Contains(t, query, "GROUP BY name")
	assert.Contains(t, query, "HAVING COUNT(*) > $3")
	assert.Contains(t, query, "ORDER BY name ASC")
	assert.Contains(t, query, "LIMIT 10")
	assert.Contains(t, query, "OFFSET 20")
	assert.Equal(t, []any{18, true, 5}, args)
}

func TestQueryBuilder_Build_QuestionMarkPlaceholders(t *testing.T) {
	client := NewClient(&mockDataStore{})

	query, args, err := client.QueryBuilder().
		Select("users").
		Where("age > ?", 18).
		Where("name = ?", "alice").
		Build()

	require.NoError(t, err)
	assert.Contains(t, query, "age > $1")
	assert.Contains(t, query, "name = $2")
	assert.Equal(t, []any{18, "alice"}, args)
}

func TestQueryBuilder_Build_MixedPlaceholders(t *testing.T) {
	client := NewClient(&mockDataStore{})

	_, _, err := client.QueryBuilder().
		Select("users").
		Where("age > ? AND name = $2", 18, "alice").
		Build()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixed placeholders")
}

func TestClient_BulkInsert_Empty(t *testing.T) {
	client := NewClient(&mockDataStore{})

	n, err := client.BulkInsert(context.Background(), "t", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestClient_BulkInsert_MismatchedColumns(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	_, err := client.BulkInsert(context.Background(), "t", []string{"a", "b"}, [][]any{{1}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row 0 has 1 values, expected 2")
}

func TestClient_EnableObservability(t *testing.T) {
	ds := &mockDataStore{}
	client := NewClient(ds)

	obs := client.EnableObservability(context.Background())
	assert.NotNil(t, obs)

	// Calling again returns the same instance.
	obs2 := client.EnableObservability(context.Background())
	assert.Same(t, obs, obs2)
}
