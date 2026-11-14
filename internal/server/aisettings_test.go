package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/llm"
	"github.com/henriquesakata/cerne/internal/project"
)

const aiSecret = "sk-live-NEVER-SHOW-THIS-1234"

type fakeProv struct {
	reply string
	err   error
}

func (f fakeProv) Complete(context.Context, string) (string, error) { return f.reply, f.err }

// aiServer is a local ui server over a project with no [llm], an isolated
// config dir and a stubbed PATH probe.
func aiServer(t *testing.T, found ...string) (http.Handler, string) {
	t.Helper()
	t.Setenv("CERNE_CONFIG_DIR", t.TempDir())
	t.Setenv("CERNE_LLM_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	repoDir, projDir := newBackend(t)
	if err := project.SaveConfig(projDir, project.Config{ProjectID: "p", Backend: "b"}); err != nil {
		t.Fatal(err)
	}
	origF, origD := project.FoundSubscriptionCLIs, project.DetectSubscriptionCLI
	t.Cleanup(func() { project.FoundSubscriptionCLIs, project.DetectSubscriptionCLI = origF, origD })
	project.FoundSubscriptionCLIs = func() []string { return found }
	project.DetectSubscriptionCLI = func() string {
		if len(found) > 0 {
			return found[0]
		}
		return ""
	}
	return New(repoDir, projDir, ""), projDir
}

// aiResult decodes the {ok,error} answer of the connection test.
func aiResult(t *testing.T, body string) (bool, string) {
	t.Helper()
	var r struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return r.OK, r.Error
}

func aiDo(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

type aiStatus struct {
	Mode     string `json:"mode"`
	Source   string `json:"source"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
	CLIs     []struct {
		Name  string `json:"name"`
		Mode  string `json:"mode"`
		Found bool   `json:"found"`
	} `json:"clis"`
	Key struct {
		Env    string `json:"env"`
		Set    bool   `json:"set"`
		Source string `json:"source"`
		Last4  string `json:"last4"`
	} `json:"key"`
}

func TestAIStatusReportsDetectionAndKeyWithoutTheValue(t *testing.T) {
	h, _ := aiServer(t, "codex")
	code, body := aiDo(t, h, http.MethodGet, "/api/config/llm", "")
	var st aiStatus
	if code != 200 || json.Unmarshal([]byte(body), &st) != nil {
		t.Fatalf("%d %s", code, body)
	}
	if st.Mode != "codex-cli" || st.Source != "detected" || st.Provider != "anthropic" || st.Key.Env != "CERNE_LLM_API_KEY" || st.Key.Set {
		t.Fatalf("status = %+v", st)
	}
	found := map[string]bool{}
	for _, c := range st.CLIs {
		found[c.Mode] = c.Found
	}
	if len(st.CLIs) != 3 || !found["codex-cli"] || found["claude-cli"] || found["gemini-cli"] {
		t.Fatalf("clis = %+v", st.CLIs)
	}

	t.Setenv("CERNE_LLM_API_KEY", aiSecret)
	_, body = aiDo(t, h, http.MethodGet, "/api/config/llm", "")
	json.Unmarshal([]byte(body), &st)
	if !st.Key.Set || st.Key.Source != "env" || st.Key.Last4 != "1234" || strings.Contains(body, aiSecret) {
		t.Fatalf("env key: %+v in %s", st.Key, body)
	}

	t.Setenv("CERNE_LLM_API_KEY", "")
	if err := project.WriteCredential("CERNE_LLM_API_KEY", aiSecret); err != nil {
		t.Fatal(err)
	}
	_, body = aiDo(t, h, http.MethodGet, "/api/config/llm", "")
	json.Unmarshal([]byte(body), &st)
	if !st.Key.Set || st.Key.Source != "file" || strings.Contains(body, aiSecret) {
		t.Fatalf("file key: %+v in %s", st.Key, body)
	}
}

func TestAIConnectionTest(t *testing.T) {
	h, _ := aiServer(t, "claude")
	var gotMode string
	orig := newLLMProvider
	t.Cleanup(func() { newLLMProvider = orig })
	reply, perr := "ok", error(nil)
	newLLMProvider = func(cfg project.LLMConfig) (llm.Provider, error) {
		gotMode = cfg.Mode
		return fakeProv{reply, perr}, nil
	}

	_, body := aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{"mode":"codex-cli"}`)
	if ok, _ := aiResult(t, body); gotMode != "codex-cli" || !ok {
		t.Fatalf("requested mode: got %q, body %s", gotMode, body)
	}
	aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{}`)
	if gotMode != "" {
		t.Fatalf("no mode requested: provider got %q, want the config's (empty)", gotMode)
	}

	perr = errors.New("codex-cli invocation failed: run 'codex login'")
	_, body = aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{}`)
	if ok, msg := aiResult(t, body); ok || !strings.Contains(msg, "codex login") {
		t.Fatalf("failure must carry the fix, got %s", body)
	}

	newLLMProvider = func(project.LLMConfig) (llm.Provider, error) { return nil, llm.ErrNoProviderForDelegation }
	_, body = aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{"mode":"delegation"}`)
	if ok, msg := aiResult(t, body); ok || !strings.Contains(msg, "delegation") {
		t.Fatalf("delegation: %s", body)
	}
	if code, _ := aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{"mode":"nope"}`); code != 400 {
		t.Fatalf("invalid mode: %d", code)
	}
}

func TestAIKeyPutDelete(t *testing.T) {
	h, _ := aiServer(t)
	code, body := aiDo(t, h, http.MethodPut, "/api/config/llm/key", `{"provider":"openrouter","value":"`+aiSecret+`"}`)
	if code != 200 || strings.Contains(body, aiSecret) {
		t.Fatalf("PUT: %d %s", code, body)
	}
	var k struct {
		Env, Source, Last4 string
		Set                bool
	}
	json.Unmarshal([]byte(body), &k)
	if k.Env != "OPENROUTER_API_KEY" || !k.Set || k.Source != "file" || k.Last4 != "1234" {
		t.Fatalf("PUT body = %s", body)
	}
	dir := os.Getenv("CERNE_CONFIG_DIR")
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.env"))
	if err != nil || string(raw) != "OPENROUTER_API_KEY="+aiSecret+"\n" {
		t.Fatalf("file = %q (%v)", raw, err)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(filepath.Join(dir, "credentials.env")); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", fi.Mode().Perm())
		}
	}
	// Never in the project or the settings files.
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.toml")); strings.Contains(string(b), aiSecret) {
		t.Fatal("key leaked into settings.toml")
	}

	code, body = aiDo(t, h, http.MethodDelete, "/api/config/llm/key", `{"provider":"openrouter"}`)
	json.Unmarshal([]byte(body), &k)
	if code != 200 || k.Set || strings.Contains(body, aiSecret) {
		t.Fatalf("DELETE: %d %s", code, body)
	}
	if v, _, _ := project.ResolveAPIKey("OPENROUTER_API_KEY"); v != "" {
		t.Fatal("key still stored")
	}

	for _, bad := range []string{`{"value":""}`, `{"value":"a\nb"}`, `{"provider":"nope","value":"x"}`, `not json`} {
		if code, body := aiDo(t, h, http.MethodPut, "/api/config/llm/key", bad); code != 400 || strings.Contains(body, "a\nb") {
			t.Errorf("PUT %s: %d %s", bad, code, body)
		}
	}
}

func TestAIRoutesAreLocalOnly(t *testing.T) {
	want := map[string]bool{"POST /config/llm/test": false, "PUT /config/llm/key": false, "DELETE /config/llm/key": false}
	for _, r := range dataRoutes() {
		k := r.method + " " + r.pattern
		if _, ok := want[k]; ok {
			if r.remote {
				t.Errorf("%s is remote, want local-only", k)
			}
			want[k] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("%s not declared in dataRoutes()", k)
		}
	}
}

// F-0414: the test runs the unsaved form. The typed key is used for that one
// call, in memory: not stored, not logged, not echoed.
func TestAIConnectionTestUsesTheUnsavedForm(t *testing.T) {
	h, _ := aiServer(t)
	var gotAuth, gotModel string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer api.Close()

	form := `{"provider":"openrouter","model":"m-1","base_url":"` + api.URL + `","key":"` + aiSecret + `"}`
	_, body := aiDo(t, h, http.MethodPost, "/api/config/llm/test", form)
	if ok, msg := aiResult(t, body); !ok {
		t.Fatalf("unsaved byo-key test failed: %s", msg)
	}
	if gotAuth != "Bearer "+aiSecret || gotModel != "m-1" {
		t.Fatalf("provider saw auth=%q model=%q", gotAuth, gotModel)
	}
	if strings.Contains(body, aiSecret) {
		t.Fatalf("response echoes the key: %s", body)
	}
	// Nothing was stored: no credentials file, and a status read has no key.
	_, status := aiDo(t, h, http.MethodGet, "/api/config/llm", "")
	if strings.Contains(status, aiSecret) || strings.Contains(status, `"set":true`) {
		t.Fatalf("the typed key was kept: %s", status)
	}
	if v, _, _ := project.ResolveAPIKey("OPENROUTER_API_KEY"); v != "" {
		t.Fatal("the typed key reached the credentials file")
	}

	// A failing call does not echo the key either.
	_, body = aiDo(t, h, http.MethodPost, "/api/config/llm/test", `{"provider":"openrouter","model":"m","base_url":"http://127.0.0.1:1","key":"`+aiSecret+`"}`)
	if ok, _ := aiResult(t, body); ok || strings.Contains(body, aiSecret) {
		t.Fatalf("unreachable endpoint: %s", body)
	}
}
