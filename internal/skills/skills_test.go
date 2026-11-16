package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllNonEmptyWithFrontmatter(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("All() is empty")
	}
	for _, s := range all {
		if s.Name == "" || s.RelPath == "" {
			t.Fatalf("skill missing name/relpath: %+v", s)
		}
		for _, want := range []string{"name:", "description:"} {
			if !strings.Contains(s.Content, want) {
				t.Errorf("%s content missing frontmatter %q", s.Name, want)
			}
		}
		if len(s.Content) < 200 {
			t.Errorf("%s content too short (%d bytes), looks like a stub", s.Name, len(s.Content))
		}
	}
}

func TestAllIncludesSelfLearning(t *testing.T) {
	for _, s := range All() {
		if s.Name == "cerne-self-learning" {
			if s.RelPath != "cerne-self-learning/SKILL.md" {
				t.Fatalf("relpath = %s, want cerne-self-learning/SKILL.md", s.RelPath)
			}
			for _, want := range []string{"golden path", "cerne runbook new", "cerne note new", "cerne search"} {
				if !strings.Contains(s.Content, want) {
					t.Errorf("cerne-self-learning content missing %q", want)
				}
			}
			return
		}
	}
	t.Fatal("All() missing cerne-self-learning")
}

func TestInstallWritesSelfLearning(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	dst := filepath.Join(dir, ".claude", "skills", "cerne-self-learning", "SKILL.md")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}
}

func TestInstallCreatesUpdatesUnchanged(t *testing.T) {
	dir := t.TempDir()

	res, err := Install(dir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(res) != len(All()) {
		t.Fatalf("results = %d, want %d", len(res), len(All()))
	}
	for _, r := range res {
		if r.Status != StatusCreated {
			t.Errorf("%s status = %s, want created", r.Skill, r.Status)
		}
	}
	dst := filepath.Join(dir, ".claude", "skills", "cerne-authoring", "SKILL.md")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	if string(got) != All()[0].Content {
		t.Fatalf("installed content != embedded canonical")
	}

	// Second run: byte-identical → unchanged.
	res, _ = Install(dir)
	for _, r := range res {
		if r.Status != StatusUnchanged {
			t.Errorf("%s status = %s, want unchanged", r.Skill, r.Status)
		}
	}

	// Local divergence → overwritten to canonical, reported as updated.
	if err := os.WriteFile(dst, []byte("locally edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _ = Install(dir)
	for _, r := range res {
		if r.Skill == "cerne-authoring" && r.Status != StatusUpdated {
			t.Errorf("status = %s, want updated", r.Status)
		}
	}
	got, _ = os.ReadFile(dst)
	if string(got) != All()[0].Content {
		t.Fatalf("after update, installed content != canonical (overwrite expected)")
	}
}

// TestStale (F-0405): Stale names the canonical skills missing or diverging
// from the embed — SKILL.md or a reference file — and is empty when current.
func TestStale(t *testing.T) {
	dir := t.TempDir()
	if got := Stale(dir); len(got) != len(All()) {
		t.Fatalf("empty project: Stale = %v, want every skill", got)
	}
	if _, err := Install(dir); err != nil {
		t.Fatal(err)
	}
	if got := Stale(dir); len(got) != 0 {
		t.Fatalf("fresh install: Stale = %v, want none", got)
	}
	skill := filepath.Join(dir, ".claude", "skills", "cerne-self-learning", "SKILL.md")
	if err := os.WriteFile(skill, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Stale(dir); len(got) != 1 || got[0] != "cerne-self-learning" {
		t.Fatalf("diverging SKILL.md: Stale = %v", got)
	}
	if _, err := Install(dir); err != nil {
		t.Fatal(err)
	}
	var ref string
	for _, s := range All() {
		if len(s.References) > 0 {
			ref = filepath.Join(dir, ".claude", "skills", s.Name, filepath.FromSlash(s.References[0].RelPath))
		}
	}
	if err := os.WriteFile(ref, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Stale(dir); len(got) != 1 || got[0] != "cerne-authoring" {
		t.Fatalf("diverging reference: Stale = %v", got)
	}
}

// TestStaleUnreadableIsNotStale (F-0405): a read error other than not-exist
// leaves the verdict unknown, so the skill is not listed.
func TestStaleUnreadableIsNotStale(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dir, ".claude", "skills", "cerne-self-learning", "SKILL.md")
	if err := os.Chmod(skill, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(skill, 0o644)
	if got := Stale(dir); len(got) != 0 {
		t.Fatalf("unreadable SKILL.md: Stale = %v, want none", got)
	}
}
