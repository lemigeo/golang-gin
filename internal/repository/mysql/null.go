package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
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
// A nullable INT UNSIGNED column comes back as sql.NullInt32, so a value above
// MaxInt32 cannot survive the round trip. The request bindings already cap
// these fields, which is why this is an error rather than a clamp: a value
// this large means validation was bypassed, and silently storing a different
// number would be worse than refusing.

// ErrValueOutOfRange means a value cannot be stored in its column.
var ErrValueOutOfRange = errors.New("value out of range for the column")

// nullConv converts domain pointers to sql.Null* types, collecting the first
// range error so a long parameter list stays readable at the call site.
type nullConv struct {
	err error
}

func (n *nullConv) int32(v *uint32) sql.NullInt32 {
	if v == nil {
		return sql.NullInt32{}
	}
	if *v > math.MaxInt32 {
		if n.err == nil {
			n.err = fmt.Errorf("%w: %d", ErrValueOutOfRange, *v)
		}
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
