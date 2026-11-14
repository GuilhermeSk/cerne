package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func llmProject(t *testing.T, llmBlock string) string {
	t.Helper()
	t.Setenv("CERNE_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	body := "project_id = \"x\"\nbackend = \"/tmp/b\"\n" + llmBlock
	if err := os.WriteFile(filepath.Join(dir, ".cernerc"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func resolved(t *testing.T, dir, id string) ResolvedSetting {
	t.Helper()
	for _, s := range ResolveSettings(dir).Settings {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("%s not in the registry", id)
	return ResolvedSetting{}
}

func TestLLMSettingsDeclared(t *testing.T) {
	dir := llmProject(t, "")
	for id, enum := range map[string][]string{
		"llm.mode":     {"claude-cli", "codex-cli", "gemini-cli", "byo-key", "delegation"},
		"llm.provider": {"anthropic", "openai", "gemini", "openrouter", "compatible"},
		"llm.model":    nil,
		"llm.base_url": nil,
	} {
		s := resolved(t, dir, id)
		if s.Group != "ai" || !slices.Equal(s.Scopes, []string{ScopeProject, ScopeHost}) || !slices.Equal(s.Enum, enum) {
			t.Errorf("%s: group=%q scopes=%v enum=%v", id, s.Group, s.Scopes, s.Enum)
		}
	}
	long := "https://example.com/" + strings.Repeat("a", 120)
	if err := SetSetting(dir, "llm.base_url", ScopeHost, long); err != nil {
		t.Errorf("a long base_url must be accepted: %v", err)
	}
}

func TestLLMModeDefaultIsTheDetectedCLI(t *testing.T) {
	dir := llmProject(t, "")
	orig := DetectSubscriptionCLI
	t.Cleanup(func() { DetectSubscriptionCLI = orig })

	DetectSubscriptionCLI = func() string { return "codex" }
	if s := resolved(t, dir, "llm.mode"); s.Value != "codex-cli" || s.Origin != OriginDetected {
		t.Fatalf("detected: %v from %q", s.Value, s.Origin)
	}
	DetectSubscriptionCLI = func() string { return "" }
	if s := resolved(t, dir, "llm.mode"); s.Value != "delegation" || s.Origin != OriginDefault {
		t.Fatalf("none: %v from %q", s.Value, s.Origin)
	}
	DetectSubscriptionCLI = func() string { return "codex" }
	if err := SetSetting(dir, "llm.mode", ScopeHost, "byo-key"); err != nil {
		t.Fatal(err)
	}
	if s := resolved(t, dir, "llm.mode"); s.Value != "byo-key" || s.Origin != ScopeHost {
		t.Fatalf("stored value must beat detection: %v from %q", s.Value, s.Origin)
	}
}

func TestLoadConfigFillsLLMFromHost(t *testing.T) {
	dir := llmProject(t, "")
	if err := SetSetting(dir, "llm.mode", ScopeHost, "codex-cli"); err != nil {
		t.Fatal(err)
	}
	if err := SetSetting(dir, "llm.model", ScopeHost, "gpt-5"); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Mode != "codex-cli" || cfg.LLM.Model != "gpt-5" {
		t.Fatalf("host [llm] did not reach LoadConfig: %+v", cfg.LLM)
	}
}

func TestLoadConfigProjectLLMWinsPerField(t *testing.T) {
	dir := llmProject(t, "\n[llm]\nmode = \"claude-cli\"\n")
	for id, v := range map[string]string{"llm.mode": "codex-cli", "llm.model": "gpt-5"} {
		if err := SetSetting(dir, id, ScopeHost, v); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _, _ := LoadConfig(dir)
	if cfg.LLM.Mode != "claude-cli" {
		t.Fatalf("project mode must win, got %q", cfg.LLM.Mode)
	}
	if cfg.LLM.Model != "gpt-5" {
		t.Fatalf("an empty project field takes the host value, got %q", cfg.LLM.Model)
	}
}

func TestLoadConfigIgnoresInvalidHostLLM(t *testing.T) {
	dir := llmProject(t, "")
	host := filepath.Join(os.Getenv("CERNE_CONFIG_DIR"), "settings.toml")
	os.WriteFile(host, []byte("[llm]\nmode = \"bogus\"\nmodel = 5\n"), 0o644)
	cfg, _, err := LoadConfig(dir)
	if err != nil || cfg.LLM.Mode != "" || cfg.LLM.Model != "" {
		t.Fatalf("invalid host values must fail open: %+v %v", cfg.LLM, err)
	}
}
