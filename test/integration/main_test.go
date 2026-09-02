package integration

import (
	"flag"
	"log"
	"os"
	"testing"

	"golang-gin/internal/testutil"
)

func TestMain(m *testing.M) {
	// testing.Short() reads a flag, and in TestMain the flags are not parsed
	// yet: m.Run() is what normally parses them. Checking before that panics
	// with "Short called before Parse", so parse here first.
	flag.Parse()

	if testing.Short() {
		log.Println("skipping integration tests: -short (these need Docker)")
		os.Exit(0)
	}

	os.Exit(testutil.RunWithDB(m))
}
