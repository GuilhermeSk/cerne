package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/gitback"
	"github.com/henriquesakata/cerne/internal/store"
)

// T-0007: tasks inherit their parent's gates — approve and accept cascade,
// and each task declares its own bar. The tests are named per criterion.

const (
	taskBody      = "# Doc\n\n## Problema\n\nCorpo real do documento, não um stub.\n\n## Verificação\n\n"
	namedTaskBar  = "Barra: fixture de teste — referência nomeada\nPergunta ao juiz: qual versão é mais clara?\n"
	exemptTaskBar = "Barra: não aplicável (regra de gate) — padrão alternativo: testes\n"
)

// seedWithSpec persists f with spec as its spec.md (no bar appended) and
// commits, like seedFeatureInState.
func seedWithSpec(t *testing.T, repoDir string, f store.Feature, spec string) {
	t.Helper()
	if f.Title == "" {
		f.Title = "Feature " + f.ID
	}
	if err := store.SaveFeature(repoDir, f); err != nil {
		t.Fatalf("SaveFeature: %v", err)
	}
	path := FeatureDocPath(repoDir, f.ID, "spec.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitback.CommitAndPush(repoDir, "seed "+f.ID); err != nil {
		t.Fatalf("CommitAndPush seed: %v", err)
	}
}

// TestStartTaskRefusesWithoutBar (crit. 4): a task whose spec declares no
// valid bar is refused at start, before any write; an inherited bar must name
// the task's own parent, whose bar must be a named one. Light mode charges no
// bar, so the start passes there.
func TestStartTaskRefusesWithoutBar(t *testing.T) {
	for _, tc := range []struct {
		name, parentBar, taskBar string
	}{
		{"no bar", namedTaskBar, "Só testes.\n"},
		{"malformed bar", namedTaskBar, "Barra: não aplicável\n"},
		{"inherited from another feature", namedTaskBar, "Barra: herdada de F-0099\n"},
		{"inherited from a parent with no named bar", exemptTaskBar, "Barra: herdada de E-0001\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir, projDir, _ := newFeatureBackend(t)
			seedWithSpec(t, repoDir, store.Feature{ID: "E-0001", State: domain.StateInDevelopment}, taskBody+tc.parentBar)
			seedWithSpec(t, repoDir, store.Feature{ID: "T-0001", Parent: "E-0001", State: domain.StateApproved}, taskBody+tc.taskBar)
			before, err := store.ReadEvents(repoDir)
			if err != nil {
				t.Fatal(err)
			}
			_, err = TransitionFeature(repoDir, projDir, "T-0001", domain.ActionStart, "", "", nil)
			if !errors.Is(err, ErrTaskBarRequired) {
				t.Fatalf("start err = %v, want ErrTaskBarRequired", err)
			}
			if f := reload(t, repoDir, "T-0001"); f.State != domain.StateApproved {
				t.Fatalf("state = %s, want approved (nothing written)", f.State)
			}
			if after, _ := store.ReadEvents(repoDir); len(after) != len(before) {
				t.Fatalf("a refused start appended %d event(s)", len(after)-len(before))
			}
		})
	}
	t.Run("light mode", func(t *testing.T) {
		repoDir, projDir, _ := newFeatureBackend(t)
		saveGauntlet(t, projDir, "off")
		seedWithSpec(t, repoDir, store.Feature{ID: "T-0001", State: domain.StateApproved}, taskBody+"Só testes.\n")
		if _, err := TransitionFeature(repoDir, projDir, "T-0001", domain.ActionStart, "", "", nil); err != nil {
			t.Fatalf("light mode charges no bar, start err = %v", err)
		}
	})
}

// TestNextStepTaskBar (crit. 4, F-0371): NextStep calls the task's bar gate,
// so it never sends the agent to a start that refuses.
func TestNextStepTaskBar(t *testing.T) {
	proj, repo := nextStepRepo(t)
	if err := store.SaveFeature(repo, store.Feature{ID: "T-0001", Title: "t", State: domain.StateApproved}); err != nil {
		t.Fatal(err)
	}
	nextStepDoc(t, repo, "T-0001", "spec.md", taskBody+"Só testes.\n")
	s, err := NextStep(repo, proj, "T-0001")
	if err != nil {
		t.Fatal(err)
	}
	if s.Command != "" || !strings.Contains(s.Step, "the start would refuse") || !strings.Contains(s.Step, "spec.md") {
		t.Fatalf("step = %+v, want the start's bar refusal pointing at spec.md", s)
	}
}

// TestApproveRefusesInheritedBarOnFeature (crit. 4): the inherited form is a
// task's only — an F-/E- spec carrying it has no bar of its own, refused by the
// approve gate and listed by the check as a malformed bar.
func TestApproveRefusesInheritedBarOnFeature(t *testing.T) {
	repoDir, projDir, _ := newFeatureBackend(t)
	seedWithSpec(t, repoDir, store.Feature{ID: "E-0001", State: domain.StateApproved}, taskBody+namedTaskBar)
	seedWithSpec(t, repoDir, store.Feature{ID: "F-0001", Parent: "E-0001", State: domain.StateCandidate,
		Facets: []string{"code"}, Components: []string{"app"}}, taskBody+"Barra: herdada de E-0001\n")
	imps, err := ApproveReadiness(repoDir, projDir, "F-0001")
	if err != nil {
		t.Fatal(err)
	}
	if len(imps) != 1 || imps[0].Code != ImpedimentBarMalformed || !strings.Contains(imps[0].Message, "only a task") {
		t.Fatalf("impediments = %+v, want one bar-malformed naming the task-only form", imps)
	}
	if _, err := TransitionFeature(repoDir, projDir, "F-0001", domain.ActionApprove, "", "", nil); !errors.Is(err, ErrSpecMissingBar) {
		t.Fatalf("approve err = %v, want ErrSpecMissingBar", err)
	}
}

// TestStartTaskAcceptsInheritedBar (crit. 4): each of the four forms starts a
// task — the inherited one when it names the parent and the parent's bar is
// named.
func TestStartTaskAcceptsInheritedBar(t *testing.T) {
	for _, bar := range []string{
		namedTaskBar,
		exemptTaskBar,
		"Barra: dispensada por decisão humana (\"pode seguir\")\n",
		"Barra: herdada de E-0001\n",
		"Bar: inherited from e-0001\n",
	} {
		t.Run(bar, func(t *testing.T) {
			repoDir, projDir, _ := newFeatureBackend(t)
			seedWithSpec(t, repoDir, store.Feature{ID: "E-0001", State: domain.StateInDevelopment}, taskBody+namedTaskBar)
			seedWithSpec(t, repoDir, store.Feature{ID: "T-0001", Parent: "E-0001", State: domain.StateApproved}, taskBody+bar)
			out, err := TransitionFeature(repoDir, projDir, "T-0001", domain.ActionStart, "", "", nil)
			if err != nil {
				t.Fatalf("start err = %v, want the task started", err)
			}
			if out.To != domain.StateInDevelopment {
				t.Fatalf("state = %s, want in_development", out.To)
			}
		})
	}
}
