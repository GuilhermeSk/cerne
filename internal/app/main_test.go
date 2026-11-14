package app

import (
	"os"
	"testing"
)

// TestMain drops Cursor's agent-shell marker (F-0428) inherited from the
// invoking environment: a suite run from a Cursor agent would otherwise act as
// an agent wherever a test clears CERNE_ACTOR to play the human (N-0058).
// Tests that need it set it per case.
func TestMain(m *testing.M) {
	os.Unsetenv("CURSOR_AGENT")
	os.Exit(m.Run())
}
