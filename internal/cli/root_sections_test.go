package cli

import (
	"slices"
	"testing"
)

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
