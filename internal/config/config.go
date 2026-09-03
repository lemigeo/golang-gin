package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	defaultHTTPAddr   = ":8080"
	defaultSessionTTL = 20 * time.Minute
)

// HTTPAddr returns the address the API server listens on, taken from HTTP_ADDR.
// A malformed value fails here rather than surfacing later from net.Listen.
// The offending value is deliberately not echoed: it comes from the environment
// and must not reach a log line unsanitized.
func HTTPAddr() (string, error) {
	raw := os.Getenv("HTTP_ADDR")
	if raw == "" {
		return defaultHTTPAddr, nil
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return "", fmt.Errorf("HTTP_ADDR is not a host:port address: %w", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("HTTP_ADDR port must be numeric")
	}

	return net.JoinHostPort(host, port), nil
}

// DBDSN returns the MySQL DSN from DB_DSN. It is required: an API that cannot
// reach its database should fail at boot, not on the first request.
func DBDSN() (string, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return "", errors.New("DB_DSN is required")
	}
	return dsn, nil
}

// RedisAddr returns the Redis address from REDIS_ADDR. Sessions live there, so
// the API cannot serve logins without it.
func RedisAddr() (string, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "", errors.New("REDIS_ADDR is required")
	}
	return addr, nil
}

// SessionSecret returns the HMAC key that signs session tokens.
//
// A short secret is refused rather than padded: it is the only thing stopping
// someone from minting a token for any customer id they like.
func SessionSecret() ([]byte, error) {
	secret := os.Getenv("SESSION_SECRET")
	if len(secret) < 32 {
		return nil, errors.New("SESSION_SECRET is required and must be at least 32 characters")
	}
	return []byte(secret), nil
}

// SessionTTL returns how long a session lives, from SESSION_TTL (a Go duration
// such as "20m"). Defaults to 20 minutes.
func SessionTTL() (time.Duration, error) {
	raw := os.Getenv("SESSION_TTL")
	if raw == "" {
		return defaultSessionTTL, nil
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errors.New("SESSION_TTL must be a duration such as 20m")
	}
	if ttl <= 0 {
		return 0, errors.New("SESSION_TTL must be positive")
	}
	return ttl, nil
}
