package mysql

import (
	"context"
	"database/sql"
	"fmt"
)

// TxRunner runs a function inside one database transaction.
//
// It accepts the same DB the repositories take. With a *sql.DB it opens
// a real transaction; with anything else — in practice the *sql.Tx an
// integration test opens and rolls back — it runs the function on that handle
// as is, because the outer transaction already provides atomicity and MySQL
// has no nested transactions.
type TxRunner struct {
	db DB
}

func NewTxRunner(db DB) *TxRunner { return &TxRunner{db: db} }

func (t *TxRunner) Run(ctx context.Context, fn func(DB) error) error {
	db, ok := t.db.(*sql.DB)
	if !ok {
		return fn(t.db)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// No-op once the transaction is committed.
		_ = tx.Rollback()
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
