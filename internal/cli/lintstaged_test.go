package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/gitback"
	"github.com/henriquesakata/cerne/internal/lint"
	"github.com/henriquesakata/cerne/internal/project"
	"github.com/henriquesakata/cerne/internal/store"
)

var otherLineRe = regexp.MustCompile(`cerne: (\d+) other findings outside your edits \(run cerne lint --staged --all for the list\)`)

// stagedDebtProject is a pulled project (F-0001, clean) plus old debt the user
// did not touch: F-0002 with a broken link, already in the backend.
func stagedDebtProject(t *testing.T) (projDir, specPath string) {
	t.Helper()
	projDir, _, repoDir, specPath := pulledProject(t)
	now := time.Now().UTC()
	if err := store.SaveFeature(repoDir, store.Feature{ID: "F-0002", Title: "Divida antiga", State: domain.StateCandidate, Facets: []string{"code"}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveFeature: %v", err)
	}
	writeBackendDoc(t, repoDir, "content/features/F-0002/spec.md",
		"---\ncerne_id: F-0002\n---\n\n# Divida antiga\n\nVeja [x](./inexistente.md).\n")
	if _, err := gitback.CommitAndPush(repoDir, "seed old debt"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}
	if out, err := execCerne(t, "pull", "--dir", projDir); err != nil {
		t.Fatalf("pull: %v\n%s", err, out)
	}
	return projDir, specPath
}

// debtSpecPath is the materialized spec of the old-debt feature F-0002.
func debtSpecPath(projDir string) string {
	return filepath.Join(projDir, ".docs", "current", "features", "F-0002-divida-antiga", "spec.md")
}

const otherHint = "run cerne lint --staged --all for the list"

// cleanSpec carries every template section and the bar, so it lints clean.
func cleanSpec(extra string) string {
	return "---\ncerne_id: F-0001\n---\n\n# Parcelamento\n\n## Problema\n\nDividir compras." + extra + "\n\n## Objetivo e critérios de aceitação\n\n1. Parcelar.\n\n## Decisões do design\n\nNenhuma.\n\n## Desenho\n\nSimples.\n\n## Verificação\n\nBarra: fixture de teste — referência nomeada\nMétodo: comparação cega\nSaída: vencer ou empatar\nPergunta ao juiz: qual é melhor?\n\n## Execução recomendada (agente)\n\nUm agente.\n\n## Fora de escopo\n\n- Nada.\n"
}

var brokenEdit = cleanSpec("\n\nver [detalhe](./sumiu.md)")

// F-0382 crit. 1+2: only the edit's findings are listed and fail; the rest is a count.
func TestLintStagedFailsOnlyOnEditedEntity(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	mustWrite(t, specPath, brokenEdit)
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err == nil {
		t.Fatalf("want failure on the edit's broken link:\n%s", out)
	}
	if !strings.Contains(out, "sumiu.md") || strings.Contains(out, "inexistente.md") {
		t.Fatalf("listing must hold the edit's finding only:\n%s", out)
	}
	if m := otherLineRe.FindStringSubmatch(out); m == nil || m[1] == "0" {
		t.Fatalf("missing the outside-count line:\n%s", out)
	}
}

// F-0382 crit. 2: a clean edit passes even with old debt, and still counts it.
func TestLintStagedCleanEditPassesWithDebtCount(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	mustWrite(t, specPath, cleanSpec(""))
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil {
		t.Fatalf("old debt must not fail a clean edit: %v\n%s", err, out)
	}
	if !otherLineRe.MatchString(out) {
		t.Fatalf("missing the outside-count line:\n%s", out)
	}
}

// F-0382 crit. 3.
func TestLintStagedNoLocalEdits(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil || !strings.Contains(out, "cerne: no local edits to lint") {
		t.Fatalf("want exit 0 and the no-edits line, err=%v:\n%s", err, out)
	}
}

// F-0382 crit. 4: --all is today's behavior.
func TestLintStagedAllKeepsOldBehavior(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	mustWrite(t, specPath, cleanSpec(""))
	out, err := execCerne(t, "lint", "--staged", "--all", "--dir", projDir)
	if err == nil || !strings.Contains(out, "inexistente.md") || otherLineRe.MatchString(out) {
		t.Fatalf("--all must list and fail on the old debt, err=%v:\n%s", err, out)
	}
}

// F-0382 crit. 6: --json carries both groups.
func TestLintStagedJSONObject(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	mustWrite(t, specPath, brokenEdit)
	out, _ := execCerne(t, "lint", "--staged", "--json", "--dir", projDir)
	out, _, _ = strings.Cut(out, "\nError:")
	var got struct {
		Findings   []struct{ Rule, File string } `json:"findings"`
		OtherCount int                           `json:"other_count"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, out)
	}
	if len(got.Findings) != 1 || got.Findings[0].Rule != "broken-link" || got.OtherCount == 0 {
		t.Fatalf("got %+v", got)
	}
}

// F-0382 crit. 7: an explicit --feature scope wins over the edit set.
func TestLintStagedFeatureScopeWins(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	out, err := execCerne(t, "lint", "--staged", "--feature", "F-0002", "--dir", projDir)
	if err == nil || !strings.Contains(out, "inexistente.md") || otherLineRe.MatchString(out) || strings.Contains(out, "no local edits") {
		t.Fatalf("--feature must keep today's behavior, err=%v:\n%s", err, out)
	}
}

// Round 2 crit. 1: an old finding in an edited doc stays old debt (the line shifts, the key does not).
func TestLintStagedOldFindingInEditedDocStaysOther(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	p := debtSpecPath(projDir)
	b, _ := os.ReadFile(p)
	mustWrite(t, p, strings.Replace(string(b), "# Divida antiga\n", "# Divida antiga\n\nParagrafo novo.\n", 1))
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil || strings.Contains(out, "inexistente.md") {
		t.Fatalf("old finding of an edited doc must not fail or be listed, err=%v:\n%s", err, out)
	}
	if m := otherLineRe.FindStringSubmatch(out); m == nil || !strings.Contains(out, otherHint) {
		t.Fatalf("want the count line with the --staged --all hint:\n%s", out)
	}
	if strings.Contains(out, "fixed by your edits") {
		t.Fatalf("nothing was fixed, no fixed line expected:\n%s", out)
	}
}

// Round 2 crit. 1: a detail change is a new finding (and the old one is fixed).
func TestLintStagedDetailChangeIsNew(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	p := debtSpecPath(projDir)
	b, _ := os.ReadFile(p)
	mustWrite(t, p, strings.Replace(string(b), "inexistente.md", "outro.md", 1))
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err == nil || !strings.Contains(out, "outro.md") || strings.Contains(out, "inexistente.md") {
		t.Fatalf("a changed detail must be listed as new, err=%v:\n%s", err, out)
	}
	if !strings.Contains(out, "cerne: 1 findings fixed by your edits") {
		t.Fatalf("the old detail is gone, want the fixed line:\n%s", out)
	}
}

// Round 2 crit. 1: an edit that clears an old finding says so.
func TestLintStagedFixedLine(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	p := debtSpecPath(projDir)
	b, _ := os.ReadFile(p)
	mustWrite(t, p, strings.Replace(string(b), "Veja [x](./inexistente.md).", "Sem link.", 1))
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil || !strings.Contains(out, "cerne: 1 findings fixed by your edits") || strings.Contains(out, "inexistente.md") {
		t.Fatalf("want exit 0 and the fixed line, err=%v:\n%s", err, out)
	}
}

// Round 2: --json carries fixed_count.
func TestLintStagedJSONFixedCount(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	p := debtSpecPath(projDir)
	b, _ := os.ReadFile(p)
	mustWrite(t, p, strings.Replace(string(b), "Veja [x](./inexistente.md).", "Sem link.", 1))
	out, _ := execCerne(t, "lint", "--staged", "--json", "--dir", projDir)
	var got struct {
		FixedCount int `json:"fixed_count"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.FixedCount != 1 {
		t.Fatalf("want fixed_count 1, err=%v:\n%s", err, out)
	}
}

// Round 2, language bug: an English project's staged tree must print English section names.
func TestLintStagedFollowsDocsLanguage(t *testing.T) {
	projDir, _, repoDir, specPath := pulledProject(t)
	if err := store.SaveProject(repoDir, store.Project{Name: "demo", DocsLanguage: "en"}); err != nil {
		t.Fatal(err)
	}
	if _, err := gitback.CommitAndPush(repoDir, "docs language en"); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, specPath, "---\ncerne_id: F-0001\n---\n\n# Parcelamento\n\n## Problem\n\nx\n")
	for _, args := range [][]string{{"lint", "--staged", "--all"}, {"lint", "--staged"}} {
		out, _ := execCerne(t, append(args, "--dir", projDir)...)
		if !strings.Contains(out, "Goal and acceptance criteria") || strings.Contains(out, "Objetivo e critérios") {
			t.Fatalf("%v: want English section names:\n%s", args, out)
		}
	}
}

func TestLintAllNeedsStaged(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	out, err := execCerne(t, "lint", "--all", "--dir", projDir)
	if err == nil || !strings.Contains(out, "--all needs --staged") {
		t.Fatalf("want the --all needs --staged error, err=%v:\n%s", err, out)
	}
}

// Round 2 crit. 1 and 5: the diff of findings. A finding absent from the base is the
// edit's wherever it lands (a plan-older-than-spec on the plan after a spec edit);
// the line is not part of the identity; a base finding that vanished is fixed.
func TestDiffFindings(t *testing.T) {
	old := lint.Finding{Rule: lint.RuleBrokenLink, Entity: "F-1", File: "content/features/F-1/spec.md", Detail: "broken link a", Line: 3}
	moved := old
	moved.Line = 9
	plan := lint.Finding{Rule: lint.RulePlanOlderThanSpec, Entity: "F-1", File: "content/features/F-1/plan.md", Detail: "plan older"}
	gone := lint.Finding{Rule: lint.RuleBrokenLink, Entity: "F-2", File: "content/features/F-2/spec.md", Detail: "broken link b"}
	added, kept, fixed := diffFindings([]lint.Finding{moved, plan}, []lint.Finding{old, gone})
	if len(added) != 1 || added[0].Rule != lint.RulePlanOlderThanSpec || len(kept) != 1 || fixed != 1 {
		t.Fatalf("added=%v kept=%v fixed=%d", added, kept, fixed)
	}
}

// noLintOrCopy makes the project lint and the tree copy fail the test if called.
func noLintOrCopy(t *testing.T) {
	t.Helper()
	oldLint, oldCopy := projectLint, copyTree
	projectLint = func(string, string, project.Config) ([]lint.Finding, error) {
		t.Fatalf("project lint ran with no local edits")
		return nil, nil
	}
	copyTree = func(string, string) error {
		t.Fatalf("clone copied with no local edits")
		return nil
	}
	t.Cleanup(func() { projectLint, copyTree = oldLint, oldCopy })
}

// F-0393 crit. 1: no edits answers at once, without the copy or the project lint.
func TestLintStagedNoEditsSkipsLintAndCopy(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	noLintOrCopy(t)
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil || strings.TrimSpace(out) != "cerne: no local edits to lint" {
		t.Fatalf("want exit 0 and only the no-edits line, err=%v:\n%s", err, out)
	}
}

// F-0393 crit. 2: --json with no edits is the fixed payload.
func TestLintStagedNoEditsJSONSkipsLint(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	noLintOrCopy(t)
	out, err := execCerne(t, "lint", "--staged", "--json", "--dir", projDir)
	if err != nil {
		t.Fatalf("no edits must pass: %v\n%s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 2 || got["edits"] != float64(0) {
		t.Fatalf("want {findings: [], edits: 0}, err=%v got=%v", err, got)
	}
	if f, ok := got["findings"].([]any); !ok || len(f) != 0 {
		t.Fatalf("findings must be an empty array: %s", out)
	}
}

// F-0393 crit. 4: an edit to a frozen document is refused as before, not short-circuited.
func TestLintStagedFrozenEditRefused(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatalf("resolvePaths: %v", err)
	}
	repoDir := ctx.repoDir
	mustWrite(t, specPath, cleanSpec("\n\nmais"))
	f, err := store.LoadFeature(repoDir, "F-0001")
	if err != nil {
		t.Fatalf("LoadFeature: %v", err)
	}
	f.State = domain.StateImplemented
	if err := store.SaveFeature(repoDir, f); err != nil {
		t.Fatalf("SaveFeature: %v", err)
	}
	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err == nil || strings.Contains(out, "no local edits") {
		t.Fatalf("want the frozen refusal, err=%v:\n%s", err, out)
	}
}

// F-0393 crit. 2: --json with edits carries the number of edited documents.
func TestLintStagedJSONEditsCount(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	mustWrite(t, specPath, cleanSpec(""))
	out, _ := execCerne(t, "lint", "--staged", "--json", "--dir", projDir)
	var got struct {
		Edits *int `json:"edits"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Edits == nil || *got.Edits != 1 {
		t.Fatalf("want edits 1, err=%v:\n%s", err, out)
	}
}

// seedImplemented seeds F-0003: implemented, completed by event after the
// Gauntlet/evidence adoption cutoffs, spec with one criterion, and the given
// result.md (F-0403). Returns the pulled project.
func seedImplemented(t *testing.T, result string) (projDir, repoDir string) {
	t.Helper()
	projDir, _, repoDir, _ = pulledProject(t)
	now := time.Now().UTC()
	if err := store.SaveFeature(repoDir, store.Feature{ID: "F-0003", Title: "Orchard ledger", State: domain.StateImplemented, Facets: []string{"code"}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveFeature: %v", err)
	}
	writeBackendDoc(t, repoDir, "content/features/F-0003/spec.md",
		"---\ncerne_id: F-0003\n---\n\n# Orchard ledger\n\n## Objetivo e critérios de aceitação\n\n1. Post entries.\n")
	writeBackendDoc(t, repoDir, "content/features/F-0003/result.md", result)
	if err := store.AppendEvent(repoDir, domain.Event{EntityID: "F-0003", Type: "feature.completed", At: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if _, err := gitback.CommitAndPush(repoDir, "seed implemented F-0003"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}
	if out, err := execCerne(t, "pull", "--dir", projDir); err != nil {
		t.Fatalf("pull: %v\n%s", err, out)
	}
	return projDir, repoDir
}

const resultNoGauntlet = "---\ncerne_id: F-0003\n---\n\n# Orchard ledger\n\n## Resultado\n\nDone.\n"

const resultWithGauntlet = "---\ncerne_id: F-0003\n---\n\n# Orchard ledger\n\n## Resultado\n\nDone.\n\n## Aceitação\n\n- critério 1 → TestPost\n\n## Gauntlet\n\nBarra: não aplicável (parity fix)\n"

// implementedNoGauntlet: F-0003 whose result lacks evidence and Gauntlet.
func implementedNoGauntlet(t *testing.T) (string, string) {
	t.Helper()
	return seedImplemented(t, resultNoGauntlet)
}

func lintKeys(fs []lint.Finding) []string {
	var o []string
	for _, f := range fs {
		o = append(o, f.Rule+"|"+f.Entity+"|"+f.File+"|"+f.Detail)
	}
	sort.Strings(o)
	return o
}

// treeFindings assembles the staged tree (no edits) and lints it.
func treeFindings(t *testing.T, projDir, repoDir string, overlay map[string]string) []lint.Finding {
	t.Helper()
	cfg, _, err := project.LoadConfig(projDir)
	if err != nil {
		t.Fatal(err)
	}
	root, _, cleanup, err := stagedLintRoot(repoDir, projDir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for rel, body := range overlay {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	got, err := projectLint(root, projDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func hasRule(fs []lint.Finding, rule, entity string) bool {
	for _, f := range fs {
		if f.Rule == rule && f.Entity == entity {
			return true
		}
	}
	return false
}

// F-0403 crit. 1+4: the staged tree without edits yields the clone's findings
// (same rule, entity, file, detail); fails when a rule reads a dir the tree lacks.
func TestLintStagedTreeParity(t *testing.T) {
	projDir, repoDir := implementedNoGauntlet(t)
	cfg, _, err := project.LoadConfig(projDir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := projectLint(repoDir, projDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(want, lint.RuleResultMissingGauntlet, "F-0003") {
		t.Fatalf("fixture must trip result-missing-gauntlet on the clone: %v", want)
	}
	got := treeFindings(t, projDir, repoDir, nil)
	if !reflect.DeepEqual(lintKeys(got), lintKeys(want)) {
		t.Fatalf("tree findings differ from clone:\n got %v\nwant %v", lintKeys(got), lintKeys(want))
	}
}

// F-0403 crit. 1 at the CLI: `lint --staged --all --json` with no edit lists the
// findings of `lint --json`, minus the modes only the default run has.
func TestLintStagedAllJSONMatchesLintJSON(t *testing.T) {
	projDir, _ := implementedNoGauntlet(t)
	keys := func(args ...string) []string {
		out, _ := execCerne(t, append(args, "--json", "--dir", projDir)...)
		var rows []map[string]any
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&rows); err != nil {
			t.Fatalf("%v: not a findings array: %v\n%s", args, err, out)
		}
		var o []string
		for _, r := range rows {
			if r["rule"] == lint.RuleDriftSuspect || r["rule"] == lint.RuleHookCitesMissingRule {
				continue
			}
			b, _ := json.Marshal(r)
			o = append(o, string(b))
		}
		sort.Strings(o)
		return o
	}
	clone, staged := keys("lint"), keys("lint", "--staged", "--all")
	if len(clone) == 0 || !reflect.DeepEqual(clone, staged) {
		t.Fatalf("staged --all differs from lint:\n clone  %v\n staged %v", clone, staged)
	}
}

// F-0403: a fresh backend has no events/; the tree still assembles and lints.
func TestLintStagedTreeWithoutEventsDir(t *testing.T) {
	projDir, repoDir := implementedNoGauntlet(t)
	if err := os.RemoveAll(filepath.Join(repoDir, "events")); err != nil {
		t.Fatal(err)
	}
	got := treeFindings(t, projDir, repoDir, nil)
	if len(got) == 0 {
		t.Fatal("expected the unrelated findings to still lint")
	}
}

// skipEventsCopy stubs copyTree to behave like the tree before F-0403.
func skipEventsCopy(t *testing.T) {
	t.Helper()
	old := copyTree
	copyTree = func(src, dst string) error {
		if filepath.Base(src) == "events" {
			return nil
		}
		return old(src, dst)
	}
	t.Cleanup(func() { copyTree = old })
}

// F-0403 crit. 2: an edit that touches nothing a rule reads counts no finding as
// fixed. The control (tree without events/) reports fixed findings, which is the
// bug this feature removes.
func TestLintStagedUnrelatedEditFixesNothing(t *testing.T) {
	projDir, _ := implementedNoGauntlet(t)
	specPath := filepath.Join(projDir, ".docs", "current", "features", "F-0001-parcelamento", "spec.md")
	b, _ := os.ReadFile(specPath)
	mustWrite(t, specPath, string(b)+"\nmais uma linha\n")
	fixed := func() (int, string) {
		out, err := execCerne(t, "lint", "--staged", "--json", "--dir", projDir)
		var got map[string]any
		if jerr := json.NewDecoder(strings.NewReader(out)).Decode(&got); jerr != nil {
			t.Fatalf("not JSON (err=%v): %v\n%s", err, jerr, out)
		}
		n, ok := got["fixed_count"].(float64)
		if !ok {
			t.Fatalf("fixed_count missing:\n%s", out)
		}
		return int(n), out
	}
	n, _ := fixed()
	text, _ := execCerne(t, "lint", "--staged", "--dir", projDir)
	if n != 0 || strings.Contains(text, "fixed by your edits") {
		t.Fatalf("unrelated edit must fix nothing, fixed_count=%d:\n%s", n, text)
	}
	skipEventsCopy(t)
	if n, out := fixed(); n == 0 {
		t.Fatalf("control: without events/ the clone's result findings should look fixed:\n%s", out)
	}
}

// F-0403 crit. 3: the clone's result has evidence and a Gauntlet; overlaying a
// version without "## Gauntlet" on the assembled tree trips result-missing-gauntlet,
// and the unedited tree does not. (An implemented feature's .docs result is frozen
// for push, so the edit is overlaid on the tree directly.)
func TestLintStagedTreeGauntletGate(t *testing.T) {
	projDir, repoDir := seedImplemented(t, resultWithGauntlet)
	if hasRule(treeFindings(t, projDir, repoDir, nil), lint.RuleResultMissingGauntlet, "F-0003") {
		t.Fatal("unedited tree must not report result-missing-gauntlet")
	}
	edited := treeFindings(t, projDir, repoDir, map[string]string{"content/features/F-0003/result.md": resultNoGauntlet})
	if !hasRule(edited, lint.RuleResultMissingGauntlet, "F-0003") {
		t.Fatalf("overlay without Gauntlet must trip result-missing-gauntlet: %v", lintKeys(edited))
	}
}

// F-0404 crit. 2: the tree pass and the base pass overlap; each waits for the
// other, so a sequential run times out instead of passing.
func TestLintStagedPassesRunConcurrently(t *testing.T) {
	projDir, _ := stagedDebtProject(t)
	p := debtSpecPath(projDir)
	b, _ := os.ReadFile(p)
	mustWrite(t, p, strings.Replace(string(b), "Veja [x](./inexistente.md).", "Sem link.", 1))

	var mu sync.Mutex
	started := 0
	both := make(chan struct{})
	old := projectLint
	projectLint = func(repo, proj string, cfg project.Config) ([]lint.Finding, error) {
		mu.Lock()
		started++
		if started == 2 {
			close(both)
		}
		mu.Unlock()
		select {
		case <-both:
		case <-time.After(10 * time.Second):
			t.Errorf("the other pass never started: passes are sequential")
		}
		return old(repo, proj, cfg)
	}
	t.Cleanup(func() { projectLint = old })

	out, err := execCerne(t, "lint", "--staged", "--dir", projDir)
	if err != nil || !strings.Contains(out, "cerne: 1 findings fixed by your edits") {
		t.Fatalf("output must match the sequential run, err=%v:\n%s", err, out)
	}
}

// hashTree maps every file under dir (rel path) to its sha256.
func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		sum := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// F-0404 crit. 3: the staged tree hard-links the files the edit did not touch,
// inside the project cache dir, and the edited file is a new file, not the link.
func TestLintStagedTreeLinksUneditedFiles(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	b, _ := os.ReadFile(specPath)
	mustWrite(t, specPath, string(b)+"\nmais uma linha\n")
	repoDir := mustCtxRepoDir(t, projDir)

	root, edits, cleanup, err := stagedLintRoot(repoDir, projDir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if edits.count != 1 {
		t.Fatalf("want 1 edit, got %d", edits.count)
	}
	if filepath.Dir(root) != filepath.Dir(repoDir) || !strings.HasPrefix(filepath.Base(root), "stagedlint-") {
		t.Fatalf("tree %s must live beside the clone %s as stagedlint-*", root, repoDir)
	}
	linked, copied := 0, 0
	for _, sub := range []string{"entities", "content", "events"} {
		_ = filepath.WalkDir(filepath.Join(repoDir, sub), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(repoDir, p)
			a, _ := os.Stat(p)
			c, err := os.Stat(filepath.Join(root, rel))
			if err != nil {
				t.Errorf("%s missing from the tree", rel)
				return nil
			}
			if os.SameFile(a, c) {
				linked++
			} else {
				copied++
				if rel != "content/features/F-0001/spec.md" {
					t.Errorf("%s was copied, want a hard link", rel)
				}
			}
			return nil
		})
	}
	if linked == 0 || copied != 1 {
		t.Fatalf("want every file linked but the edited one, linked=%d copied=%d", linked, copied)
	}
}

// F-0404 crit. 4: lint --staged with an edit leaves every clone file byte
// identical, including the one the edit overlays.
func TestLintStagedNeverWritesTheClone(t *testing.T) {
	projDir, specPath := stagedDebtProject(t)
	repoDir := mustCtxRepoDir(t, projDir)
	before := hashTree(t, repoDir)
	b, _ := os.ReadFile(specPath)
	mustWrite(t, specPath, string(b)+"\nmais uma linha\n")
	if _, err := execCerne(t, "lint", "--staged", "--dir", projDir); err != nil {
		t.Fatal(err)
	}
	if after := hashTree(t, repoDir); !reflect.DeepEqual(before, after) {
		t.Fatalf("lint --staged changed the clone")
	}
}

func mustCtxRepoDir(t *testing.T, projDir string) string {
	t.Helper()
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatalf("resolvePaths: %v", err)
	}
	return ctx.repoDir
}
