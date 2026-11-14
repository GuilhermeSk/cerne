package app

import (
	"os"
	"testing"
)

// TestMain keeps the subscription-CLI detection (F-0414) off: no test of this
// package may reach a real claude/codex/gemini on PATH. It also drops Cursor's
// agent-shell marker (F-0428) inherited from the invoking environment: a suite
// run from a Cursor agent would otherwise act as an agent wherever a test
// clears CERNE_ACTOR to play the human (N-0058); tests that need it set it per
// case.
func TestMain(m *testing.M) {
	os.Setenv("CERNE_NO_CLI_DETECT", "1")
	os.Unsetenv("CURSOR_AGENT")
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
