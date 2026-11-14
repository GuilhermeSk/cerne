package app

import (
	"os"
	"strings"
	"testing"
)

// F-0414 (D5): the host settings.toml [llm] reaches every LoadConfig, so a test
// binary must never see the developer's real one.
func TestTestMainIsolatesTheConfigDir(t *testing.T) {
	dir := os.Getenv("CERNE_CONFIG_DIR")
	if dir == "" || !strings.Contains(dir, "cerne-config-") {
		t.Fatalf("CERNE_CONFIG_DIR = %q, want a throwaway dir from TestMain", dir)
	}
}
