package gopgbase

import (
	"context"
	"database/sql"
	"fmt"
)

// Transaction executes fn within a database transaction.
//
// Behavior:
//   - Starts a read/write transaction with default isolation via BeginTx.
//   - If fn returns nil, the transaction is committed.
//   - If fn returns an error, the transaction is rolled back and the error is returned.
//   - If fn panics, the transaction is rolled back and the panic is re-raised.
//   - Respects ctx cancellation and deadlines throughout.
//
// Transaction is safe for concurrent use — each call gets its own *sql.Tx.
func (c *Client) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return c.transactionWithOpts(ctx, nil, fn)
}

// TransactionWithIsolation executes fn within a transaction at the given isolation level.
//
// See sql.IsolationLevel constants (e.g., sql.LevelSerializable).
func (c *Client) TransactionWithIsolation(ctx context.Context, level sql.IsolationLevel, fn func(tx *sql.Tx) error) error {
	return c.transactionWithOpts(ctx, &sql.TxOptions{Isolation: level}, fn)
}

// ReadOnlyTransaction executes fn within a read-only transaction.
//
// The database will reject any write operations (INSERT, UPDATE, DELETE)
// inside fn, which is useful for ensuring SELECT-only logic.
func (c *Client) ReadOnlyTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return c.transactionWithOpts(ctx, &sql.TxOptions{ReadOnly: true}, fn)
}

// Savepoint executes fn within a named savepoint inside an existing transaction.
//
// If fn returns an error, the savepoint is rolled back (but the outer
// transaction remains active). If fn succeeds, the savepoint is released.
func (c *Client) Savepoint(ctx context.Context, tx *sql.Tx, name string, fn func(tx *sql.Tx) error) (err error) {
	if _, execErr := tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %s", quoteIdentifier(name))); execErr != nil {
		return fmt.Errorf("gopgbase: savepoint %q: %w", name, execErr)
	}

	defer func() {
		if p := recover(); p != nil {
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", quoteIdentifier(name)))
			panic(p)
		}
		if err != nil {
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", quoteIdentifier(name)))
		} else {
			if _, relErr := tx.ExecContext(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", quoteIdentifier(name))); relErr != nil {
				err = fmt.Errorf("gopgbase: release savepoint %q: %w", name, relErr)
			}
		}
	}()

	err = fn(tx)
	return err
}

// BatchTransaction executes multiple operations sequentially within a single transaction.
//
// If any operation returns an error, the entire transaction is rolled back.
func (c *Client) BatchTransaction(ctx context.Context, operations ...func(tx *sql.Tx) error) error {
	return c.Transaction(ctx, func(tx *sql.Tx) error {
		for i, op := range operations {
			if err := op(tx); err != nil {
				return fmt.Errorf("gopgbase: batch operation %d: %w", i, err)
			}
		}
		return nil
	})
}

// transactionWithOpts is the shared implementation for all transaction methods.
func (c *Client) transactionWithOpts(ctx context.Context, opts *sql.TxOptions, fn func(tx *sql.Tx) error) (err error) {
	tx, err := c.ds.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("gopgbase: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("gopgbase: commit tx: %w", err)
	}
	return nil
}
