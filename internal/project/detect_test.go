package project

import (
	"errors"
	"testing"
)

func fakeLook(have ...string) func(string) (string, error) {
	return func(n string) (string, error) {
		for _, h := range have {
			if h == n {
				return "/bin/" + n, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestDetectCLIOrderAndNone(t *testing.T) {
	t.Setenv("CERNE_NO_CLI_DETECT", "")
	cases := []struct {
		have []string
		want string
	}{
		{[]string{"gemini", "codex", "claude"}, "claude"},
		{[]string{"gemini", "codex"}, "codex"},
		{[]string{"gemini"}, "gemini"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := firstCLI(fakeLook(c.have...)); got != c.want {
			t.Errorf("have %v: got %q want %q", c.have, got, c.want)
		}
	}
}

func TestDetectCLIDisabledSkipsPATH(t *testing.T) {
	t.Setenv("CERNE_NO_CLI_DETECT", "1")
	look := func(string) (string, error) { t.Fatal("PATH consulted"); return "", nil }
	if got := firstCLI(look); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestTestMainDisablesDetection(t *testing.T) {
	if DetectSubscriptionCLI() != "" {
		t.Fatal("detection must be off under go test")
	}
}

func firstCLI(look func(string) (string, error)) string {
	if f := foundCLIs(look); len(f) > 0 {
		return f[0]
	}
	return ""
}

func TestFoundCLIsListsAllInOrder(t *testing.T) {
	t.Setenv("CERNE_NO_CLI_DETECT", "")
	got := foundCLIs(fakeLook("gemini", "claude"))
	if len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Fatalf("got %v", got)
	}
}
