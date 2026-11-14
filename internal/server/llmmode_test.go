package server

import "testing"

func TestValidLLMModeAcceptsSubscriptionCLIs(t *testing.T) {
	for _, m := range []string{"claude-cli", "codex-cli", "gemini-cli", "byo-key", "delegation"} {
		if !validLLMMode(m) {
			t.Errorf("%q rejected", m)
		}
	}
	if validLLMMode("nope") || validLLMMode("") {
		t.Error("invalid mode accepted")
	}
}
