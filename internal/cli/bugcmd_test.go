package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/app"
	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// T-0006 criterion 1: `bug new` mints B-0001 on its own sequence with the
// three-section report and the pre-filled bar line; a human at the terminal
// gets it already approved.
func TestBugNewMintsBPrefix(t *testing.T) {
	t.Setenv("CERNE_ACTOR", "")
	projDir, _, repoDir := setupProject(t)
	out, err := execCerne(t, "bug", "new", "Salvar quebra", "--context", "O salvar falha", "--dir", projDir)
	if err != nil {
		t.Fatalf("bug new: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "B-0001 created (approved)") || !strings.Contains(out, "cerne feature start B-0001") {
		t.Fatalf("output = %q, want the creation line and the start hint", out)
	}
	f, err := store.LoadFeature(repoDir, "B-0001")
	if err != nil || f.State != domain.StateApproved || f.Origin != "human" {
		t.Fatalf("LoadFeature = %+v, %v; want B-0001 approved human", f, err)
	}
	raw, err := os.ReadFile(app.FeatureDocPath(repoDir, "B-0001", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(raw)
	for _, want := range []string{"## Contexto\nO salvar falha", "## Como reproduzir", "## Esperado × obtido", "Barra: não aplicável (correção com teste de regressão)"} {
		if !strings.Contains(spec, want) {
			t.Errorf("spec missing %q:\n%s", want, spec)
		}
	}
}

// An agent session without --on-behalf asked for the bug itself: candidate.
func TestBugNewAgentBornCandidate(t *testing.T) {
	t.Setenv("CERNE_ACTOR", "agent")
	projDir, _, repoDir := setupProject(t)
	if out, err := execCerne(t, "bug", "new", "Achado pelo agente", "--context", "ctx", "--dir", projDir); err != nil {
		t.Fatalf("bug new: %v\noutput: %s", err, out)
	}
	f, _ := store.LoadFeature(repoDir, "B-0001")
	if f.State != domain.StateCandidate || f.Origin != "agent" {
		t.Fatalf("bug = %+v, want candidate agent", f)
	}
	if out, err := execCerne(t, "bug", "new", "Relatado", "--context", "ctx2", "--on-behalf", "salvar quebrou", "--dir", projDir); err != nil {
		t.Fatalf("bug new --on-behalf: %v\noutput: %s", err, out)
	}
	g, _ := store.LoadFeature(repoDir, "B-0002")
	if g.State != domain.StateApproved || g.Origin != "human" {
		t.Fatalf("bug = %+v, want approved human", g)
	}
}

func TestBugNewRequiresContext(t *testing.T) {
	projDir, _, _ := setupProject(t)
	if _, err := execCerne(t, "bug", "new", "Sem relato", "--dir", projDir); err == nil {
		t.Fatal("want the context gate for bug new")
	}
}
