package mysql

import "golang-gin/internal/repository/mysql/sqlc"

// DB is a handle the repositories can run on: *sql.DB in production, *sql.Tx
// inside a transaction or an integration test.
//
// It is an alias for the interface sqlc generates, re-exported here so that
// callers outside this package never have to import the generated package.
// Keeping sqlc types inside repository/mysql is a rule the linter enforces.
type DB = sqlc.DBTX
