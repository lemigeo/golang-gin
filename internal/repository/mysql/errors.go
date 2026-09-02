package mysql

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// mysqlErrDupEntry is ER_DUP_ENTRY: a unique constraint was violated.
const mysqlErrDupEntry = 1062

// isDuplicate reports whether err is a unique-key violation on the named index.
// The driver message names the violated key, e.g.
//
//	Duplicate entry 'a@b.com' for key 'customer.uk_customer_email'
//
// An empty index matches any duplicate.
func isDuplicate(err error, index string) bool {
	var myErr *mysql.MySQLError
	if !errors.As(err, &myErr) || myErr.Number != mysqlErrDupEntry {
		return false
	}
	return index == "" || strings.Contains(myErr.Message, index)
}
