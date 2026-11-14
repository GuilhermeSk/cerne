package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/henriquesakata/cerne/internal/doclang"
	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/spectmpl"
	"github.com/henriquesakata/cerne/internal/store"
)

// newBug mints a B- report as actor (T-0006); N-0058: an agent session
// inherits CERNE_ACTOR, so every test sets it explicitly.
func newBug(t *testing.T, actor, fixes, onBehalf string, facets, components []string) (string, string, store.Feature, error) {
	t.Helper()
	t.Setenv("CERNE_ACTOR", actor)
	repoDir, projDir, _ := newFeatureBackend(t)
	body := spectmpl.BugBodyIn(doclang.PT, "Quebra ao salvar", "1. salvar", "esperava salvar")
	f, err := CreateBugWithSpec(repoDir, projDir, "Salvar quebra", "", fixes, facets, components, body, false, "", "", onBehalf, DocAuthor{})
	return repoDir, projDir, f, err
}

// criterion 2: a report with a human origin is born approved and the creation
// event records the approval-by-report.
func TestBugHumanBornApproved(t *testing.T) {
	repoDir, _, f, err := newBug(t, "", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != "B-0001" || f.State != domain.StateApproved || f.Origin != "human" {
		t.Fatalf("bug = %+v, want B-0001 approved human", f)
	}
	evs, _ := store.ReadAllEvents(repoDir)
	for _, e := range evs {
		if e.EntityID == f.ID && e.Type == "feature.created" {
			if e.Payload["approved"] != "by report" {
				t.Fatalf("payload = %v, want approved=by report", e.Payload)
			}
			return
		}
	}
	t.Fatal("no feature.created event")
}

// An agent relaying a quoted human request is a human origin too (T-0005 rule).
func TestBugOnBehalfBornApproved(t *testing.T) {
	_, _, f, err := newBug(t, "agent", "", "o salvar quebrou", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.State != domain.StateApproved || f.Origin != "human" {
		t.Fatalf("bug = %+v, want approved human", f)
	}
}

func TestBugAgentBornCandidate(t *testing.T) {
	repoDir, _, f, err := newBug(t, "agent", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.State != domain.StateCandidate || f.Origin != "agent" {
		t.Fatalf("bug = %+v, want candidate agent", f)
	}
	evs, _ := store.ReadAllEvents(repoDir)
	for _, e := range evs {
		if e.EntityID == f.ID && e.Payload["approved"] != "" {
			t.Fatalf("an agent-origin bug must not record an approval: %v", e.Payload)
		}
	}
}

// criterion 4: --fixes records the link and inherits what the line left out.
func TestBugFixesInheritsTags(t *testing.T) {
	t.Setenv("CERNE_ACTOR", "")
	repoDir, projDir, _ := newFeatureBackend(t)
	tgt, err := CreateFeature(repoDir, projDir, "Entregue", "", "", []string{"code"}, []string{"cli"})
	if err != nil {
		t.Fatal(err)
	}
	tgt.Capabilities = []string{"C-0001"}
	setFeatureState(t, repoDir, tgt, tgt.State) // saves and commits the capability
	bug, err := CreateBugWithSpec(repoDir, projDir, "Corrige entregue", "", tgt.ID, nil, nil,
		spectmpl.BugBodyIn(doclang.PT, "c", "s", "e"), false, "", "", "", DocAuthor{})
	if err != nil {
		t.Fatal(err)
	}
	if bug.Fixes != tgt.ID || !slices.Equal(bug.Facets, []string{"code"}) ||
		!slices.Equal(bug.Components, []string{"cli"}) || !slices.Equal(bug.Capabilities, []string{"C-0001"}) {
		t.Fatalf("bug = %+v, want fixes + inherited tags", bug)
	}
	// Explicit tags win over the target's.
	over, err := CreateBugWithSpec(repoDir, projDir, "Outro defeito", "", tgt.ID, []string{"infra"}, nil,
		spectmpl.BugBodyIn(doclang.PT, "c2", "s", "e"), false, "", "", "", DocAuthor{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(over.Facets, []string{"infra"}) || !slices.Equal(over.Components, []string{"cli"}) {
		t.Fatalf("bug = %+v, want explicit facets and inherited components", over)
	}
}

// criterion 3: start needs no plan; complete still needs a real result.
func TestStartBugNeedsNoPlan(t *testing.T) {
	repoDir, projDir, f, err := newBug(t, "", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := TransitionFeature(repoDir, projDir, f.ID, domain.ActionStart, "", "", nil)
	if err != nil || out.To != domain.StateInDevelopment {
		t.Fatalf("start = %+v, %v; want in_development without a plan", out, err)
	}
}

func TestCompleteBugNeedsResult(t *testing.T) {
	repoDir, projDir, f, err := newBug(t, "", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionFeature(repoDir, projDir, f.ID, domain.ActionStart, "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionFeature(repoDir, projDir, f.ID, domain.ActionComplete, "", "", nil); !errors.Is(err, domain.ErrGateResultRequired) {
		t.Fatalf("complete err = %v, want ErrGateResultRequired", err)
	}
}
