package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/henriquesakata/cerne/internal/doccheck"
	"github.com/henriquesakata/cerne/internal/doclang"
	"github.com/henriquesakata/cerne/internal/store"
)

// ErrTaskBarRequired is the start refusal for a task (T-) whose spec declares
// no valid bar of its own (T-0007): a task has no approve, so its start is the
// first gate before the work.
var ErrTaskBarRequired = errors.New("task spec declares no valid bar")

// isTask reports a task id (T-, F-0203).
func isTask(id string) bool { return strings.HasPrefix(id, "T-") }

// checkTaskBar is the start's bar gate of a task (T-0007): its spec declares a
// valid bar in one of the four forms, and an inherited one names the task's
// own parent, whose spec names a bar a Gauntlet round answers to. Any other
// feature passes; so does every start in light mode (the caller skips it).
func checkTaskBar(repoDir string, f store.Feature) error {
	if !isTask(f.ID) {
		return nil
	}
	spec, err := os.ReadFile(FeatureDocPath(repoDir, f.ID, "spec.md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	l := store.DocsLanguage(repoDir)
	if _, err := doccheck.SpecBarIn(spec, l); err != nil {
		bar := doclang.Display(doclang.BarLabel, l)
		return fmt.Errorf(`%w: %s — %v; declare the task's own bar under the spec's verification section, one of: "%s: <named reference + access>", "%s: %s (…)", "%s: %s (…)" or "%s: %s %s", then 'cerne push'`,
			ErrTaskBarRequired, f.ID, err, bar, bar, doclang.Display(doclang.NotApplicable, l), bar, doclang.Display(doclang.Waived, l), bar, doclang.Display(doclang.Inherited, l), parentOrSlot(f))
	}
	parent, inherited := doccheck.BarInherited(spec)
	if !inherited {
		return nil
	}
	if parent != f.Parent {
		return fmt.Errorf("%w: %s inherits its bar from %s, which is not its parent (%s)", ErrTaskBarRequired, f.ID, parent, parentOrSlot(f))
	}
	pspec, err := os.ReadFile(FeatureDocPath(repoDir, parent, "spec.md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !doccheck.BarNamed(pspec) {
		return fmt.Errorf("%w: %s inherits its bar from %s, whose spec names no bar of its own — an inherited bar needs the parent's named bar and Gauntlet round; declare the task's own bar instead", ErrTaskBarRequired, f.ID, parent)
	}
	return nil
}

// specBarFor is doccheck.SpecBarIn for feature id's spec: the inherited form
// is a task's only (T-0007), so an F-/E- spec carrying it has a malformed bar.
func specBarFor(id string, spec []byte, l doclang.Lang) (string, error) {
	bar, err := doccheck.SpecBarIn(spec, l)
	if _, inherited := doccheck.BarInherited(spec); err == nil && inherited && !isTask(id) {
		return "", fmt.Errorf(`%w: "%s" is a form only a task (T-) declares — %s needs a bar of its own`, doccheck.ErrBarMalformed, doclang.Display(doclang.Inherited, l), id)
	}
	return bar, err
}

// parentOrSlot is the task's parent id, or the slot to fill when it has none.
func parentOrSlot(f store.Feature) string {
	if f.Parent == "" {
		return "<parent id>"
	}
	return f.Parent
}
