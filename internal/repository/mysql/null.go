package mysql

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// mysqlErrCheckConstraint is ER_CHECK_CONSTRAINT_VIOLATED.
const mysqlErrCheckConstraint = 3819

// isCheckViolation reports whether err is a CHECK constraint failure on the
// named constraint.
func isCheckViolation(err error, name string) bool {
	var myErr *mysql.MySQLError
	if !errors.As(err, &myErr) || myErr.Number != mysqlErrCheckConstraint {
		return false
	}
	return name == "" || strings.Contains(myErr.Message, name)
}

// The helpers below translate between domain pointers and the sql.Null* types
// sqlc generates for nullable columns. Pointers keep "no limit" (NULL) and
// "zero" distinguishable, which a plain uint32 would not.
//
// Callers must have bounded the value at MaxInt32 already; the request
// bindings do that.

func nullInt32(v *uint32) sql.NullInt32 {
	if v == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*v), Valid: true}
}

func uint32Ptr(v sql.NullInt32) *uint32 {
	if !v.Valid || v.Int32 < 0 {
		return nil
	}
	out := uint32(v.Int32)
	return &out
}

func nullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	out := v.String
	return &out
}

func nullTime(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

func timePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	out := v.Time
	return &out
}
