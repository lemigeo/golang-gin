package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tx opens a transaction that is rolled back when the test ends.
//
// It satisfies mysql.DB, so it can be handed to handler.NewEngine: the whole
// request path runs inside it and leaves nothing behind. That is why the
// repositories take a mysql.DB rather than a *sql.DB.
func Tx(t *testing.T) *sql.Tx {
	t.Helper()

	tx, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}
