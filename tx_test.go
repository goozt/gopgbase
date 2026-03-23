package gopgbase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockClient(t *testing.T) (*Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewClient(db), mock
}

func TestTransaction_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := client.Transaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransaction_FnError_Rollback(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := client.Transaction(context.Background(), func(_ *sql.Tx) error {
		return errors.New("something failed")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "something failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransaction_Panic_Rollback(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	assert.Panics(t, func() {
		_ = client.Transaction(context.Background(), func(_ *sql.Tx) error {
			panic("boom")
		})
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransaction_CommitError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	err := client.Transaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit tx")
}

func TestReadOnlyTransaction_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := client.ReadOnlyTransaction(context.Background(), func(_ *sql.Tx) error {
		return nil
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionWithIsolation_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := client.TransactionWithIsolation(context.Background(), sql.LevelSerializable, func(_ *sql.Tx) error {
		return nil
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchTransaction_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := client.BatchTransaction(context.Background(),
		func(_ *sql.Tx) error { return nil },
		func(_ *sql.Tx) error { return nil },
	)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchTransaction_PartialError_Rollback(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := client.BatchTransaction(context.Background(),
		func(_ *sql.Tx) error { return nil },
		func(_ *sql.Tx) error { return errors.New("op2 failed") },
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch operation 1")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSavepoint_Success(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`RELEASE SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		return client.Savepoint(context.Background(), tx, "sp1", func(_ *sql.Tx) error {
			return nil
		})
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSavepoint_FnError_RollbackToSavepoint(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ROLLBACK TO SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		return client.Savepoint(context.Background(), tx, "sp1", func(_ *sql.Tx) error {
			return errors.New("inner failed")
		})
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inner failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSavepoint_Panic_RollbackToSavepoint(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ROLLBACK TO SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	assert.Panics(t, func() {
		_ = client.Transaction(context.Background(), func(tx *sql.Tx) error {
			return client.Savepoint(context.Background(), tx, "sp1", func(_ *sql.Tx) error {
				panic("inner boom")
			})
		})
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSavepoint_BeginError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT`).WillReturnError(errors.New("savepoint failed"))
	mock.ExpectRollback()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		return client.Savepoint(context.Background(), tx, "sp1", func(_ *sql.Tx) error {
			return nil
		})
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "savepoint")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSavepoint_ReleaseError(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`RELEASE SAVEPOINT`).WillReturnError(fmt.Errorf("release failed"))
	mock.ExpectRollback()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		return client.Savepoint(context.Background(), tx, "sp1", func(_ *sql.Tx) error {
			return nil
		})
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "release savepoint")
	assert.NoError(t, mock.ExpectationsWereMet())
}
