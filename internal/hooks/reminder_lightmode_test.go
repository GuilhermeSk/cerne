package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runReminder runs the installed SessionStart script with PATH limited to the
// system dirs plus binDir (where a test may place a fake cerne).
func runReminder(t *testing.T, binDir string) string {
	t.Helper()
	cmd := exec.Command("bash", "assets/cerne-register-reminder.sh")
	cmd.Env = []string{"PATH=" + binDir + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reminder script: %v", err)
	}
	return string(out)
}

// fakeCerne writes a cerne stub: `hook gauntlet-rule` prints rule and exits
// with code; every other call exits 0 silently.
func fakeCerne(t *testing.T, rule string, code int) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$2\" = gauntlet-rule ]; then printf '%s' '" + rule + "'; exit " + string(rune('0'+code)) + "; fi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "cerne"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestReminder_LightMode (F-0364 critic 4): Claude Code's SessionStart text
// asks cerne for the GAUNTLET item. With the Gauntlet on — and when cerne is
// missing or fails, the fail-safe — the text is byte-for-byte the pre-light-
// mode one (golden); in light mode the item is gone.
func TestReminder_LightMode(t *testing.T) {
	golden, err := os.ReadFile("testdata/session-start-gauntlet-on.golden")
	if err != nil {
		t.Fatal(err)
	}
	if got := runReminder(t, t.TempDir()); got != string(golden) {
		t.Fatalf("no cerne on PATH: text differs from golden\n got: %q\nwant: %q", got, golden)
	}
	if got := runReminder(t, fakeCerne(t, "", 1)); got != string(golden) {
		t.Fatalf("cerne fails: text differs from golden\n got: %q", got)
	}
	light := runReminder(t, fakeCerne(t, "", 0))
	if strings.Contains(light, "GAUNTLET") || !strings.Contains(light, "HUMAN GATES") || !strings.Contains(light, "DUPLICATE GUARD") {
		t.Fatalf("light mode text:\n%s", light)
	}
	if strings.Contains(string(golden), "skip it when") {
		t.Fatal("golden carries the static skip clause")
	}
}

// TestGauntletRuleMatchesGolden: the item cerne prints is the golden's
// GAUNTLET line, so the Go copy and the script fallback cannot drift.
func TestGauntletRuleMatchesGolden(t *testing.T) {
	golden, err := os.ReadFile("testdata/session-start-gauntlet-on.golden")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(golden), "\n"+GauntletRule+"\n") {
		t.Fatalf("GauntletRule is not the golden's GAUNTLET line:\n%s", GauntletRule)
	}
}

// runReminderUnderCursor runs the SessionStart script as Cursor does (F-0428):
// CURSOR_VERSION and CURSOR_PROJECT_DIR set, the hook payload on stdin.
func runReminderUnderCursor(t *testing.T, binDir, projDir, stdin string) string {
	t.Helper()
	cmd := exec.Command("bash", "assets/cerne-register-reminder.sh")
	cmd.Env = []string{"PATH=" + binDir + ":/usr/bin:/bin", "HOME=" + t.TempDir(),
		"CURSOR_VERSION=3.25.6", "CURSOR_PROJECT_DIR=" + projDir}
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reminder script: %v", err)
	}
	return string(out)
}

// TestReminder_Cursor (F-0428 crit. 5): under Cursor the script prints only
// what `cerne hook session-context --format cursor` prints (the JSON envelope),
// handing it the payload; it stays silent without cerne, and when the
// project's .cursor/hooks.json already wires the native sessionStart.
func TestReminder_Cursor(t *testing.T) {
	bin := t.TempDir()
	stub := "#!/bin/sh\nprintf 'ARGS:%s STDIN:' \"$*\"; cat\n"
	if err := os.WriteFile(filepath.Join(bin, "cerne"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	projDir := t.TempDir()
	if got, want := runReminderUnderCursor(t, bin, projDir, `{"session_id":"s"}`),
		`ARGS:hook session-context --format cursor STDIN:{"session_id":"s"}`; got != want {
		t.Fatalf("under Cursor:\n got: %q\nwant: %q", got, want)
	}
	if got := runReminderUnderCursor(t, t.TempDir(), projDir, "{}"); got != "" {
		t.Fatalf("no cerne on PATH: want silence, got %q", got)
	}
	native := `{"hooks":{"sessionStart":[{"command":"sh -c 'cerne hook session-context --format cursor'"}]},"version":1}`
	if err := os.MkdirAll(filepath.Join(projDir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, ".cursor", "hooks.json"), []byte(native), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runReminderUnderCursor(t, bin, projDir, "{}"); got != "" {
		t.Fatalf("native sessionStart wired: want silence, got %q", got)
	}
}
