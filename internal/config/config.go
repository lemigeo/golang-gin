package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

const defaultHTTPAddr = ":8080"

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
