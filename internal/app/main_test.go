package app

import (
	"os"
	"testing"
)

// TestMain keeps the subscription-CLI detection (F-0414) off: no test of this
// package may reach a real claude/codex/gemini on PATH.
func TestMain(m *testing.M) {
	os.Setenv("CERNE_NO_CLI_DETECT", "1")
	os.Exit(m.Run())
}
