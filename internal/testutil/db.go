// Package testutil holds the plumbing integration tests share: a real MySQL
// container, the migrations applied to it, transaction isolation, and a client
// that speaks to the assembled app.
package testutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	mysqlctr "github.com/testcontainers/testcontainers-go/modules/mysql"
)

// testDB is the container-backed pool shared by every test in a package.
var testDB *sql.DB

// DB returns the shared pool. Only valid after RunWithDB has started.
func DB() *sql.DB { return testDB }

// RunWithDB starts a MySQL container, applies migrations/, runs the package's
// tests, and tears the container down. Call it from TestMain.
//
// The container starts once per package, not once per test: starting MySQL
// costs seconds, and per-test isolation comes from Tx instead.
func RunWithDB(m *testing.M) int {
	code, err := runWithDB(m)
	if err != nil {
		// Reported here, after runWithDB's defers have torn the container down.
		log.Printf("integration test setup: %v", err)
		return 1
	}
	return code
}

// runWithDB returns instead of calling log.Fatal so that the container and the
// pool are always closed: a Fatal skips deferred functions and leaks the
// container.
func runWithDB(m *testing.M) (int, error) {
	ctx := context.Background()

	container, err := mysqlctr.Run(ctx,
		"mysql:8.0",
		mysqlctr.WithDatabase("app_test"),
		mysqlctr.WithUsername("root"),
		mysqlctr.WithPassword("root"),
	)
	if err != nil {
		return 0, fmt.Errorf("start mysql container: %w", err)
	}
	defer func() {
		if termErr := container.Terminate(ctx); termErr != nil {
			log.Printf("terminate container: %v", termErr)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		return 0, fmt.Errorf("connection string: %w", err)
	}

	testDB, err = sql.Open("mysql", dsn)
	if err != nil {
		return 0, fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = testDB.Close() }()

	if waitErr := waitForDB(ctx, testDB); waitErr != nil {
		return 0, waitErr
	}
	if migrateErr := migrateUp(testDB); migrateErr != nil {
		return 0, migrateErr
	}

	stopRedis, redisErr := startRedis(ctx)
	if redisErr != nil {
		return 0, redisErr
	}
	defer stopRedis()

	return m.Run(), nil
}

func waitForDB(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		err := db.PingContext(ctx)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database never became ready: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// migrateUp applies the real migrations/ directory. Tests deliberately do not
// keep their own schema DDL: if a migration breaks, the tests must break too.
func migrateUp(db *sql.DB) error {
	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir(), "mysql", driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// migrationsDir resolves migrations/ from this file's location, so tests work
// regardless of the package they run from.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}
