package harness

import (
	"os"
	"testing"
)

// TestMain keeps the user's real host settings ([llm], F-0414) and any
// subscription CLI on PATH out of this package's tests: meta_items loads the
// project config.
func TestMain(m *testing.M) {
	os.Setenv("CERNE_NO_CLI_DETECT", "1")
	dir, err := os.MkdirTemp("", "cerne-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("CERNE_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
