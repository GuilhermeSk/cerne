package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/app"
)

// Cursor runs the project's .claude hooks through its third-party import
// (F-0428). The fixtures in testdata/cursor are payloads from a real Cursor
// 3.25.6 session's hook log (user_email dropped, paths replaced by
// placeholders the tests fill in).

const cursorSess = "2cbcdf25-fae0-4ddd-abb5-8ce420078093"

// cursorFixture loads testdata/cursor/<name>, with __PROJ__ pointing at projDir
// and __CURSOR_HOME__ at a dir outside any project, as a mutable map.
func cursorFixture(t *testing.T, name, projDir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cursor", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(raw), "__PROJ__", projDir)
	s = strings.ReplaceAll(s, "__CURSOR_HOME__", t.TempDir())
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func payloadString(t *testing.T, m map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// cursorShellOutput is Cursor's tool_output: the shell result as a JSON string.
func cursorShellOutput(t *testing.T, output string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"output": output, "exitCode": 0})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func boundTo(t *testing.T, repoDir, sess string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(repoDir), "sessions", sess))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// TestHookCursorBindsFeatureStartFromToolOutput (F-0428 crit. 1, 10): the
// logged `cerne push && cerne feature start F-0425` call — Shell tool, empty
// cwd, the result in tool_output — binds the session, through the plain
// `cerne hook bind-work` the .claude wrapper runs.
func TestHookCursorBindsFeatureStartFromToolOutput(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	p := cursorFixture(t, "post-tool-use-feature-start.json", projDir)
	if out, err := execCerneStdin(t, payloadString(t, p), "hook", "bind-work"); err != nil {
		t.Fatalf("hook bind-work: %v\noutput: %s", err, out)
	}
	if got := boundTo(t, repoDir, cursorSess); got != "F-0425" {
		t.Fatalf("bound id = %q, want F-0425", got)
	}
}

// TestHookCursorWorkStartResolvesWorkspaceRoot (crit. 1b): a cwd outside the
// project (Cursor's own terminals dir) and an empty one both resolve to the
// workspace root, and the minted W- binds.
func TestHookCursorWorkStartResolvesWorkspaceRoot(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	if out, err := execCerne(t, "work", "start", "Investigar erro X", "--dir", projDir); err != nil {
		t.Fatalf("work start: %v\noutput: %s", err, out)
	}
	for _, cwd := range []string{"outside", ""} {
		p := cursorFixture(t, "post-tool-use-terminals-cwd.json", projDir)
		sess := "sess-cwd-" + cwd
		p["session_id"], p["conversation_id"] = sess, sess
		if cwd == "" {
			p["cwd"] = ""
		}
		p["tool_input"] = map[string]any{"command": `cerne work start "Investigar erro X"`, "cwd": p["cwd"]}
		p["tool_output"] = cursorShellOutput(t, "W-0001 created (active) — Investigar erro X (actor: agent)\n")
		if out, err := execCerneStdin(t, payloadString(t, p), "hook", "bind-work"); err != nil {
			t.Fatalf("cwd %q: %v\noutput: %s", cwd, err, out)
		}
		if got := boundTo(t, repoDir, sess); got != "W-0001" {
			t.Fatalf("cwd %q: bound id = %q, want W-0001", cwd, got)
		}
	}
}

// TestHookCursorConversationIDAsSession (crit. 1c): no session_id → the
// conversation_id keys the binding.
func TestHookCursorConversationIDAsSession(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	p := cursorFixture(t, "post-tool-use-feature-start.json", projDir)
	delete(p, "session_id")
	p["conversation_id"] = "conv-only"
	if out, err := execCerneStdin(t, payloadString(t, p), "hook", "bind-work"); err != nil {
		t.Fatalf("hook bind-work: %v\noutput: %s", err, out)
	}
	if got := boundTo(t, repoDir, "conv-only"); got != "F-0425" {
		t.Fatalf("bound id = %q, want F-0425", got)
	}
}

// TestHookCursorUnboundWarningEnvelope (crit. 2): the unbound-start warning
// reaches Cursor as additional_context, not Claude Code's hookSpecificOutput.
func TestHookCursorUnboundWarningEnvelope(t *testing.T) {
	projDir, _, _ := setupProject(t)
	p := cursorFixture(t, "post-tool-use-feature-start.json", projDir)
	p["tool_input"] = map[string]any{"command": `cerne work start "X" > /dev/null`, "cwd": ""}
	p["tool_output"] = cursorShellOutput(t, "")
	out, err := execCerneStdin(t, payloadString(t, p), "hook", "bind-work")
	if err != nil {
		t.Fatalf("hook bind-work: %v\noutput: %s", err, out)
	}
	assertCursorContext(t, out, "additional_context", "NOT bound")
}

// assertCursorContext: out is exactly one JSON object whose only key is key,
// carrying want, and nothing of Claude Code's envelope.
func assertCursorContext(t *testing.T, out, key, want string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	v, ok := m[key].(string)
	if len(m) != 1 || !ok || !strings.Contains(v, want) {
		t.Fatalf("want {%q: …%q…}, got:\n%s", key, want, out)
	}
}

// TestHookCursorProvenanceEnvelope (crit. 2): a push citing an orphan URL
// reminds Cursor through additional_context.
func TestHookCursorProvenanceEnvelope(t *testing.T) {
	projDir, _, _ := setupProject(t)
	writeAuthoredDoc(t, projDir, ".docs/current/features/F-0001-coisa/spec.md", "F-0001",
		"Baseado em https://exemplo.org/artigo-que-moldou.")
	p := cursorFixture(t, "post-tool-use-feature-start.json", projDir)
	p["tool_input"] = map[string]any{"command": "cerne push", "cwd": ""}
	p["tool_output"] = cursorShellOutput(t, "Sent: 1 document(s) (snapshot abc).\n")
	out, err := execCerneStdin(t, payloadString(t, p), "hook", "provenance")
	if err != nil {
		t.Fatalf("hook provenance: %v\noutput: %s", err, out)
	}
	assertCursorContext(t, out, "additional_context", "https://exemplo.org/artigo-que-moldou")
}

// TestHookCursorWorkGate (crit. 2): the logged Write payload (no cwd) is
// denied in Cursor's flat envelope while unbound, and passes silently once
// the session is bound.
func TestHookCursorWorkGate(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	enableWorkGate(t, projDir)
	p := cursorFixture(t, "pre-tool-use-write.json", projDir)
	out, err := execCerneStdin(t, payloadString(t, p), "hook", "work-gate")
	if err != nil {
		t.Fatalf("hook work-gate: %v\noutput: %s", err, out)
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(out), &d); err != nil || d["permission"] != "deny" || !strings.Contains(d["agent_message"].(string), "work start") || d["hookSpecificOutput"] != nil {
		t.Fatalf("want Cursor deny envelope, got:\n%s", out)
	}

	if out, err := execCerne(t, "work", "start", "Investigar erro X", "--dir", projDir); err != nil {
		t.Fatalf("work start: %v\noutput: %s", err, out)
	}
	if err := app.BindSession(filepath.Dir(repoDir), cursorSess, "W-0001"); err != nil {
		t.Fatal(err)
	}
	out, err = execCerneStdin(t, payloadString(t, p), "hook", "work-gate")
	if err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("bound session: want silence, got %v:\n%s", err, out)
	}
}

// TestHookCursorSessionContext (crit. 2): the sessionStart payload (no cwd)
// gets the rules and the live board as additional_context.
func TestHookCursorSessionContext(t *testing.T) {
	projDir, _, _ := setupProject(t)
	if out, err := execCerne(t, "work", "start", "Investigar erro X", "--dir", projDir); err != nil {
		t.Fatalf("work start: %v\noutput: %s", err, out)
	}
	p := cursorFixture(t, "session-start.json", projDir)
	out, err := execCerneStdin(t, payloadString(t, p), "hook", "session-context")
	if err != nil {
		t.Fatalf("hook session-context: %v\noutput: %s", err, out)
	}
	assertCursorContext(t, out, "additional_context", "W-0001")
	assertCursorContext(t, out, "additional_context", "cerne feature new")
}

// TestHookCursorStopFollowup (crit. 3): a completed turn of a bound session
// gets the closeout as a followup_message; an aborted one gets nothing and
// does not spend the reminder; a follow-up's own stop (loop_count > 0) is
// silent.
func TestHookCursorStopFollowup(t *testing.T) {
	projDir, _, repoDir := setupProject(t)
	if out, err := execCerne(t, "work", "start", "Investigar erro X", "--dir", projDir); err != nil {
		t.Fatalf("work start: %v\noutput: %s", err, out)
	}
	if err := app.BindSession(filepath.Dir(repoDir), cursorSess, "W-0001"); err != nil {
		t.Fatal(err)
	}
	stop := func(status string, loop int) string {
		t.Helper()
		p := cursorFixture(t, "stop.json", projDir)
		p["status"], p["loop_count"] = status, loop
		out, err := execCerneStdin(t, payloadString(t, p), "hook", "stop-check")
		if err != nil {
			t.Fatalf("stop-check: %v\noutput: %s", err, out)
		}
		return out
	}
	if out := stop("completed", 1); strings.TrimSpace(out) != "" {
		t.Fatalf("loop_count 1: want silence, got:\n%s", out)
	}
	if out := stop("aborted", 0); strings.TrimSpace(out) != "" {
		t.Fatalf("aborted: want silence, got:\n%s", out)
	}
	assertCursorContext(t, stop("completed", 0), "followup_message", "W-0001")
	if out := stop("completed", 0); strings.TrimSpace(out) != "" {
		t.Fatalf("second completed stop: want the rationed silence, got:\n%s", out)
	}
}
