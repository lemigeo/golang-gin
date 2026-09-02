package integration

import (
	"os"
	"testing"

	"golang-gin/internal/testutil"
)

func TestMain(m *testing.M) {
	if testing.Short() {
		// These tests need Docker. `go test -short ./...` skips them.
		os.Exit(0)
	}
	os.Exit(testutil.RunWithDB(m))
}
