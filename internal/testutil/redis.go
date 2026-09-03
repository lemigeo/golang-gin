package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
	redisctr "github.com/testcontainers/testcontainers-go/modules/redis"
)

// testRedis is the container-backed client shared by every test in a package.
var testRedis redis.UniversalClient

// Redis returns the shared client. Only valid after RunWithDB has started.
func Redis() redis.UniversalClient { return testRedis }

// startRedis boots a Redis container for the package. Like the MySQL one it
// starts once; per-test isolation comes from unique keys, since every session
// token is random.
func startRedis(ctx context.Context) (func(), error) {
	container, err := redisctr.Run(ctx, "redis:7-alpine")
	if err != nil {
		return nil, fmt.Errorf("start redis container: %w", err)
	}

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("redis connection string: %w", err)
	}

	opts, err := redis.ParseURL(uri)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	testRedis = redis.NewClient(opts)
	if pingErr := testRedis.Ping(ctx).Err(); pingErr != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("ping redis: %w", pingErr)
	}

	return func() {
		_ = testRedis.Close()
		_ = container.Terminate(ctx)
	}, nil
}

// FlushRedis empties the session store between tests that count keys.
func FlushRedis(t *testing.T) {
	t.Helper()
	if err := testRedis.FlushAll(context.Background()).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
}
