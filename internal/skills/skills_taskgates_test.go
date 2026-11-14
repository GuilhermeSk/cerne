package skills

import (
	"strings"
	"testing"
)

// TestCerneAuthoringDocumentsTaskGates pins T-0007 in the authoring skill: a
// task declares its own bar in one of four forms — the inherited one only on a
// task, leaning on the parent's named bar and round —, its start refuses a spec
// with none, and the parent's approve and accept carry the children along.
func TestCerneAuthoringDocumentsTaskGates(t *testing.T) {
	c := fullAuthoring(t)
	for _, sub := range []string{
		"`Barra: herdada de E-NNNN`",
		"`Bar: inherited from E-NNNN`",
		"`cerne feature start T-NNNN` refuses a spec with no valid bar",
		"the parent then owes the round in its own result",
		"approves along every candidate child",
		"accepts along every implemented task",
		"`standard-without-bar`",
	} {
		if !strings.Contains(c, sub) {
			t.Errorf("skill missing %q", sub)
		}
	}
}
