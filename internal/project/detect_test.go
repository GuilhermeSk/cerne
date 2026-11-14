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
		if got := detectCLI(fakeLook(c.have...)); got != c.want {
			t.Errorf("have %v: got %q want %q", c.have, got, c.want)
		}
	}
}

func TestDetectCLIDisabledSkipsPATH(t *testing.T) {
	t.Setenv("CERNE_NO_CLI_DETECT", "1")
	look := func(string) (string, error) { t.Fatal("PATH consulted"); return "", nil }
	if got := detectCLI(look); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestTestMainDisablesDetection(t *testing.T) {
	if DetectSubscriptionCLI() != "" {
		t.Fatal("detection must be off under go test")
	}
}
