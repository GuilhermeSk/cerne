package lint

import (
	"strings"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// T-0007 crit. 5: the complete gate (CheckResult) and the lint charge a task's
// Gauntlet by the bar line of the task's own spec.

const (
	taskNamedSpec    = "# t\n\n## Verificação\n\nBarra: middleware do finance — apps/finance/web/middleware.ts\nPergunta ao juiz: qual é melhor?\n"
	taskExemptSpec   = "# t\n\n## Verificação\n\nBarra: não aplicável (regra de gate) — padrão alternativo: testes\n"
	taskWaivedSpec   = "# t\n\n## Verificação\n\nBarra: dispensada por decisão humana (\"pode seguir\")\n"
	resultRound      = "## Resultado\n\nfeito\n\n## Gauntlet\n\nBarra: middleware do finance\nRodada 1 — vitória: \"A cobre o caso vazio\"\n"
	resultExemption  = "## Resultado\n\nfeito\n\n## Gauntlet\n\nBarra: não aplicável (achei que não precisava)\n"
	resultNoGauntlet = "## Resultado\n\nfeito\n"
)

// seedLintFeature writes f with its spec and result under dir.
func seedLintFeature(t *testing.T, dir string, f store.Feature, spec, result string) store.Feature {
	t.Helper()
	if f.Title == "" {
		f.Title = "t"
	}
	if err := store.SaveFeature(dir, f); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, dir, "content/features/"+f.ID+"/spec.md", spec)
	if result != "" {
		writeDoc(t, dir, "content/features/"+f.ID+"/result.md", result)
	}
	return f
}

// gauntletOf is the result-missing-gauntlet finding among fs, or nil.
func gauntletOf(fs []Finding) *Finding {
	for i := range fs {
		if fs[i].Rule == RuleResultMissingGauntlet {
			return &fs[i]
		}
	}
	return nil
}

// TestLintTaskNamedBarNeedsRound: a task naming a bar owes the round in its
// own result — an exemption written in the result does not count.
func TestLintTaskNamedBarNeedsRound(t *testing.T) {
	dir := t.TempDir()
	tk := seedLintFeature(t, dir, store.Feature{ID: "T-0001", State: domain.StateInDevelopment}, taskNamedSpec, resultExemption)
	if g := gauntletOf(CheckResult(dir, tk)); g == nil || !strings.Contains(g.Detail, "an exemption written in the result does not count") {
		t.Fatalf("finding = %+v, want the named bar's round charged over the result's exemption", g)
	}
	writeDoc(t, dir, "content/features/T-0001/result.md", resultRound)
	if g := gauntletOf(CheckResult(dir, tk)); g != nil {
		t.Fatalf("a recorded round is a complete record, got %+v", g)
	}
}

// TestLintTaskExemptNoRound: a task whose spec declares "não aplicável" or a
// waiver owes no round — no Gauntlet record in its result at all.
func TestLintTaskExemptNoRound(t *testing.T) {
	for _, spec := range []string{taskExemptSpec, taskWaivedSpec} {
		dir := t.TempDir()
		tk := seedLintFeature(t, dir, store.Feature{ID: "T-0001", State: domain.StateInDevelopment}, spec, resultNoGauntlet)
		if g := gauntletOf(CheckResult(dir, tk)); g != nil {
			t.Fatalf("an exempt task owes no round, got %+v", g)
		}
	}
	// An F- is still read by its result, as before (no spec rule).
	dir := t.TempDir()
	f := seedLintFeature(t, dir, store.Feature{ID: "F-0001", State: domain.StateInDevelopment}, taskExemptSpec, resultNoGauntlet)
	if g := gauntletOf(CheckResult(dir, f)); g == nil {
		t.Fatal("an F- with no Gauntlet record in its result is still charged")
	}
}

// TestLintInheritedBarChargesParent: an inherited task owes no round of its
// own while its parent names a bar; the parent then owes the round in its own
// result (an exemption there does not count) and, naming none, is charged.
func TestLintInheritedBarChargesParent(t *testing.T) {
	inherited := "# t\n\n## Verificação\n\nBarra: herdada de E-0001\n"
	completed := func(ids ...string) []domain.Event {
		var evs []domain.Event
		for _, id := range ids {
			evs = append(evs, domain.Event{EntityID: id, Type: "feature.completed", At: resultGauntletStrictSince.Add(time.Hour)})
		}
		return evs
	}

	dir := t.TempDir()
	e := seedLintFeature(t, dir, store.Feature{ID: "E-0001", State: domain.StateImplemented}, taskNamedSpec, resultExemption)
	tk := seedLintFeature(t, dir, store.Feature{ID: "T-0002", Parent: "E-0001", State: domain.StateImplemented}, inherited, resultNoGauntlet)
	if g := gauntletOf(CheckResult(dir, tk)); g != nil {
		t.Fatalf("the inherited task owes no round while its parent names a bar, got %+v", g)
	}
	got := resultGauntletFindings(dir, []store.Feature{e, tk}, completed("E-0001", "T-0002"))
	if len(got) != 1 || got[0].Entity != "E-0001" || !strings.Contains(got[0].Detail, "an exemption written in the result does not count") {
		t.Fatalf("findings = %+v, want E-0001 charged for the round its task inherits", got)
	}
	writeDoc(t, dir, "content/features/E-0001/result.md", resultRound)
	if got := resultGauntletFindings(dir, []store.Feature{e, tk}, completed("E-0001", "T-0002")); len(got) != 0 {
		t.Fatalf("the parent's recorded round covers both, got %+v", got)
	}

	// An abandoned task (rejected, superseded) inherits nothing: the parent
	// is read by its own result, as before.
	for _, s := range []domain.FeatureState{domain.StateRejected, domain.StateSuperseded} {
		dir := t.TempDir()
		e := seedLintFeature(t, dir, store.Feature{ID: "E-0001", State: domain.StateImplemented}, taskNamedSpec, resultExemption)
		tk := seedLintFeature(t, dir, store.Feature{ID: "T-0002", Parent: "E-0001", State: s}, inherited, resultNoGauntlet)
		if got := resultGauntletFindings(dir, []store.Feature{e, tk}, completed("E-0001")); len(got) != 0 {
			t.Fatalf("a %s inheriting task charged its parent: %+v", s, got)
		}
	}

	// A parent naming no bar of its own: the inheritance is no bar at all.
	dir = t.TempDir()
	e = seedLintFeature(t, dir, store.Feature{ID: "E-0001", State: domain.StateImplemented}, taskExemptSpec, resultNoGauntlet)
	tk = seedLintFeature(t, dir, store.Feature{ID: "T-0002", Parent: "E-0001", State: domain.StateImplemented}, inherited, resultNoGauntlet)
	got = resultGauntletFindings(dir, []store.Feature{e, tk}, completed("E-0001", "T-0002"))
	if len(got) != 2 || !strings.Contains(got[0].Detail+got[1].Detail, "names no bar of its own") {
		t.Fatalf("findings = %+v, want both charged: the parent names no bar its task can inherit", got)
	}
}
