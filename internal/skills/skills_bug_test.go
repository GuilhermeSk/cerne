package skills

import (
	"strings"
	"testing"
)

// TestCerneAuthoringTeachesBugRoute pins T-0006 criterion 6: a human's bug
// report in chat becomes a B- through `cerne bug new --on-behalf` and is
// started without waiting for an approve; a bug the agent found itself is
// born candidate and waits for the human.
func TestCerneAuthoringTeachesBugRoute(t *testing.T) {
	c := fullAuthoring(t)
	for _, sub := range []string{
		"cerne bug new",
		"--on-behalf",
		"cerne feature start B-NNNN",
		"without waiting for an approve",
		"you found it yourself",
		"accept stays human",
	} {
		if !strings.Contains(c, sub) {
			t.Errorf("skill missing %q", sub)
		}
	}
}
