package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptStateOKDivergentMissing(t *testing.T) {
	proj := t.TempDir()
	if _, err := Install(proj); err != nil {
		t.Fatal(err)
	}
	for _, a := range Assets() {
		if got := ScriptState(proj, a.Script); got != FileOK {
			t.Errorf("%s after install = %s, want ok", a.Script, got)
		}
	}
	target := filepath.Join(proj, ".claude", "hooks", Assets()[0].Script)
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ScriptState(proj, Assets()[0].Script); got != FileDivergent {
		t.Errorf("one flipped byte = %s, want divergent", got)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if got := ScriptState(proj, Assets()[0].Script); got != FileMissing {
		t.Errorf("removed = %s, want missing", got)
	}
}

func TestGitHookStates(t *testing.T) {
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, s := range GitHookStates(proj) {
		states[s.Name] = s.State
	}
	if states[trailerHookName] != FileMissing || states[guardHookName] != FileMissing {
		t.Errorf("empty .git/hooks = %v, want both missing", states)
	}
	if _, err := Install(proj); err != nil {
		t.Fatal(err)
	}
	for _, s := range GitHookStates(proj) {
		if s.State != FileOK {
			t.Errorf("%s after install = %s, want ok", s.Name, s.State)
		}
	}
	// Foreign content in the slot is divergent with the manual snippet.
	if err := os.WriteFile(filepath.Join(proj, ".git", "hooks", trailerHookName), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range GitHookStates(proj) {
		if s.Name == trailerHookName {
			if s.State != FileDivergent || s.Detail == "" {
				t.Errorf("foreign trailer = %+v, want divergent with detail", s)
			}
		}
	}
}

// TestStale (F-0405): Stale lists the managed scripts missing or diverging from
// the embed — the same verdict ScriptState gives the harness — and never writes.
func TestStale(t *testing.T) {
	proj := t.TempDir()
	if got := Stale(proj); len(got) != len(Assets()) {
		t.Fatalf("empty project: Stale = %v, want every script", got)
	}
	if _, err := Install(proj); err != nil {
		t.Fatal(err)
	}
	if got := Stale(proj); len(got) != 0 {
		t.Fatalf("fresh install: Stale = %v", got)
	}
	script := Assets()[1].Script
	path := filepath.Join(proj, ".claude", "hooks", script)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n# local\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Stale(proj); len(got) != 1 || got[0] != script {
		t.Fatalf("diverging script: Stale = %v", got)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "#!/bin/sh\n# local\n" {
		t.Fatal("Stale rewrote the script")
	}
}
