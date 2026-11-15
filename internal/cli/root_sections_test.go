package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestTopLevelHelpListsSectionsInOrder(t *testing.T) {
	out, err := execCerne(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	last := -1
	for _, title := range []string{
		"Start here:", "Read the state:", "Author and move the work:",
		"Sync with the backend:", "Agents and coordination:",
		"Administration and housekeeping:", "Additional Commands:",
	} {
		i := strings.Index(out, title)
		if i <= last {
			t.Fatalf("%q at %d, want after %d:\n%s", title, i, last, out)
		}
		last = i
	}
}

func TestEveryTopLevelCommandHasASection(t *testing.T) {
	root := Root()
	known := map[string]bool{}
	for _, g := range root.Groups() {
		known[g.ID] = true
	}
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		if c.GroupID == "" || !known[c.GroupID] {
			t.Errorf("top-level command %q has no known help section (GroupID=%q)", c.Name(), c.GroupID)
		}
	}
}

func TestStartSectionIsTheOnRamp(t *testing.T) {
	var got []string
	for _, c := range Root().Commands() {
		if c.GroupID == "start" {
			got = append(got, c.Name())
		}
	}
	for _, want := range []string{"init", "ui", "demo", "guide", "import"} {
		if !slices.Contains(got, want) {
			t.Errorf("start section lacks %q: %v", want, got)
		}
	}
	for _, not := range []string{"retire", "forget-alias", "serve", "doctor", "lock", "guard"} {
		if slices.Contains(got, not) {
			t.Errorf("start section must not hold %q: %v", not, got)
		}
	}
}

func TestTopLevelShortsCiteNoInternalID(t *testing.T) {
	re := regexp.MustCompile(`\b[FDN]-\d{4}\b`)
	for _, c := range Root().Commands() {
		if re.MatchString(c.Short) {
			t.Errorf("%q short cites an internal id: %q", c.Name(), c.Short)
		}
	}
}
