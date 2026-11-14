package project

import (
	"os"
	"os/exec"
	"sync"
)

// subscriptionCLIs is the detection order: the first one on PATH wins.
var subscriptionCLIs = []string{"claude", "codex", "gemini"}

// lookPath is the PATH probe; tests replace it.
var lookPath = exec.LookPath

// DetectSubscriptionCLI returns the first of claude, codex, gemini found on
// PATH, "" when none is (or when CERNE_NO_CLI_DETECT is set, which the test
// binaries do so no test reaches a real CLI). Resolved once per process.
var DetectSubscriptionCLI = sync.OnceValue(func() string {
	return detectCLI(lookPath)
})

func detectCLI(look func(string) (string, error)) string {
	if os.Getenv("CERNE_NO_CLI_DETECT") != "" {
		return ""
	}
	for _, name := range subscriptionCLIs {
		if _, err := look(name); err == nil {
			return name
		}
	}
	return ""
}
