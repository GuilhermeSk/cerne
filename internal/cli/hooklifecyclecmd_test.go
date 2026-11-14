package cli

import "testing"

// TestHookLifecycleReminderRetired (F-0426 crit. 6): the retired hook stays
// callable for projects installed before F-0426, but emits nothing on the
// payloads that used to trigger it — the command's own stdout carries the
// guidance, so Claude Code does not get it twice.
func TestHookLifecycleReminderRetired(t *testing.T) {
	for _, c := range []struct{ command, stdout string }{
		{`cerne feature new "Nova coisa" --context "why"`, "F-0042 created (candidate) with its context seeded into the spec."},
		{"cerne idea promote I-0001", "I-0001: active -> promoted (F-0042)"},
		{"cerne feature start F-0042", "F-0042: approved → in_development"},
	} {
		out, err := execCerneStdin(t, hookJSON(t, "sess-lc", "/tmp/proj", "Bash",
			map[string]any{"command": c.command}, map[string]any{"stdout": c.stdout}),
			"hook", "lifecycle-reminder")
		if err != nil || out != "" {
			t.Fatalf("%s: retired hook = %q, %v; want silent exit 0", c.command, out, err)
		}
	}
}
