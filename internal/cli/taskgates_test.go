package cli

import (
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// T-0007: the CLI side of the task gates — the cascade lines, the level
// advice and the task new hint.

const exemptTaskSpec = "---\ncerne_id: %s\n---\n\n# Passo\n\n**Nível:** padrão\n\nCorpo real do documento.\n\n## Verificação\n\nBarra: não aplicável (regra de gate) — padrão alternativo: testes\n"

// seedCard persists f with spec as its spec.md and commits the backend.
func seedCard(t *testing.T, repoDir string, f store.Feature, spec string) {
	t.Helper()
	if f.Title == "" {
		f.Title = "Card " + f.ID
	}
	if err := store.SaveFeature(repoDir, f); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, repoDir, f.ID, "spec.md", spec)
	commitBackend(t, repoDir)
}

// TestFeatureAcceptPrintsCascade (T-0007 crit. 3): the parent's accept lists
// each task it accepted along; --all-implemented does not trip over a task
// the cascade already accepted.
func TestFeatureAcceptPrintsCascade(t *testing.T) {
	for _, args := range [][]string{{"E-0001"}, {"--all-implemented", "--reason", "migração"}} {
		t.Run(args[0], func(t *testing.T) {
			projDir, _, repoDir := setupProject(t)
			seedCard(t, repoDir, store.Feature{ID: "E-0001", State: domain.StateImplemented}, realDoc("E-0001", "Épico."))
			seedCard(t, repoDir, store.Feature{ID: "T-0002", Parent: "E-0001", State: domain.StateImplemented}, strings.ReplaceAll(exemptTaskSpec, "%s", "T-0002"))
			out, err := execCerne(t, append([]string{"feature", "accept"}, append(args, "--dir", projDir)...)...)
			if err != nil {
				t.Fatalf("accept: %v\n%s", err, out)
			}
			if !strings.Contains(out, "  ↳ T-0002: implemented → done along with E-0001\n") {
				t.Fatalf("accept output lacks the cascaded task:\n%s", out)
			}
			if f, _ := store.LoadFeature(repoDir, "T-0002"); f.State != domain.StateDone {
				t.Fatalf("T-0002 state = %s, want done", f.State)
			}
			if args[0] == "--all-implemented" && !strings.Contains(out, "2 feature(s) accepted.") {
				t.Fatalf("the bulk summary counts the cascaded task too:\n%s", out)
			}
		})
	}
}

// TestFeatureApprovePrintsCascade (T-0007 crit. 1): the parent's approve
// lists the child it approved along and the one it left, with why.
func TestFeatureApprovePrintsCascade(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	tags := func(f store.Feature) store.Feature { f.Components = []string{"test"}; return f }
	seedCard(t, repoDir, tags(store.Feature{ID: "E-0001", State: domain.StateCandidate}), realDoc("E-0001", "Épico."))
	seedCard(t, repoDir, tags(store.Feature{ID: "F-0002", Parent: "E-0001", State: domain.StateCandidate}), realDoc("F-0002", "Filha pronta."))
	seedCard(t, repoDir, store.Feature{ID: "F-0003", Parent: "E-0001", State: domain.StateCandidate}, realDoc("F-0003", "Filha sem tag."))
	out, err := execCerne(t, "feature", "approve", "E-0001", "--dir", projDir)
	if err != nil {
		t.Fatalf("approve: %v\n%s", err, out)
	}
	for _, want := range []string{"  ↳ F-0002: candidate → approved along with E-0001\n", "  ↳ F-0003 stays in candidate: untagged"} {
		if !strings.Contains(out, want) {
			t.Errorf("approve output lacks %q:\n%s", want, out)
		}
	}
}
