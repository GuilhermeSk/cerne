package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/gitback"
	"github.com/henriquesakata/cerne/internal/store"
)

// TestFunnelHumanActsBugReport (E-0001 criterion 16, bug part): a bug a human
// reported in the chat reaches in_development with zero human lifecycle acts
// (no approve, no accept) and one document — the three-field report; no plan.
func TestFunnelHumanActsBugReport(t *testing.T) {
	t.Setenv("CERNE_CACHE_DIR", t.TempDir())
	origin := filepath.Join(t.TempDir(), "origin.git")
	if err := gitback.InitBare(origin); err != nil {
		t.Fatalf("InitBare: %v", err)
	}
	proj := t.TempDir()
	mustCerne(t, "init", "--docs-language", "pt-BR", "--name", "Demo", "--backend", origin, "--dir", proj)
	t.Setenv("CERNE_ACTOR", "agent") // the agent relays the human's words

	mustCerne(t, "bug", "new", "Salvar quebra", "--context", "O salvar falha na tela de ajustes",
		"--on-behalf", "o salvar quebrou de novo", "--dir", proj)
	mustCerne(t, "feature", "start", "B-0001", "--dir", proj)

	verify := filepath.Join(t.TempDir(), "verify")
	if err := gitback.Clone(origin, verify); err != nil {
		t.Fatalf("Clone origin: %v", err)
	}
	f, err := store.LoadFeature(verify, "B-0001")
	if err != nil || f.State != domain.StateInDevelopment || f.Origin != "human" {
		t.Fatalf("B-0001 = %+v, %v; want in_development with human origin", f, err)
	}
	logs, _ := filepath.Glob(filepath.Join(verify, "events", "*.ndjson"))
	humanActs := 0
	for _, l := range logs {
		body, err := os.ReadFile(l)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(body), "\n") {
			var ev struct {
				EntityID string `json:"entity_id"`
				Type     string `json:"type"`
			}
			if json.Unmarshal([]byte(line), &ev) == nil && ev.EntityID == "B-0001" &&
				(ev.Type == "feature.approved" || ev.Type == "feature.accepted") {
				humanActs++
			}
		}
	}
	if humanActs != 0 {
		t.Fatalf("human lifecycle acts = %d, want 0", humanActs)
	}
	if _, err := os.Stat(filepath.Join(verify, "content", "features", "B-0001", "plan.md")); err == nil {
		t.Fatal("a bug report needs no plan, but plan.md exists")
	}
	if _, err := os.Stat(filepath.Join(verify, "content", "features", "B-0001", "spec.md")); err != nil {
		t.Fatalf("the report (spec.md) is the one document: %v", err)
	}
}
