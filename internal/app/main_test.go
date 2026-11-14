package app

import (
	"os"
	"testing"
)

// TestMain keeps the subscription-CLI detection (F-0414) off: no test of this
// package may reach a real claude/codex/gemini on PATH.
func TestMain(m *testing.M) {
	os.Setenv("CERNE_NO_CLI_DETECT", "1")
	// And the host settings (a [llm] saved from the UI reaches every LoadConfig).
	dir, err := os.MkdirTemp("", "cerne-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("CERNE_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
