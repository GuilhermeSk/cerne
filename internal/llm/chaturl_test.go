package llm

import "testing"

func TestChatURL(t *testing.T) {
	for base, want := range map[string]string{
		"https://api.openai.com":                                  "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/":                                 "https://api.openai.com/v1/chat/completions",
		"https://openrouter.ai/api":                               "https://openrouter.ai/api/v1/chat/completions",
		"https://generativelanguage.googleapis.com/v1beta/openai": "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		"http://localhost:11434/v1":                               "http://localhost:11434/v1/chat/completions",
		"http://localhost:1234/v1/":                               "http://localhost:1234/v1/chat/completions",
		"http://localhost:8080":                                   "http://localhost:8080/v1/chat/completions",
		"http://proxy.local/v1beta":                               "http://proxy.local/v1beta/chat/completions",
	} {
		if got := chatURL(base); got != want {
			t.Errorf("chatURL(%q) = %q, want %q", base, got, want)
		}
	}
}
