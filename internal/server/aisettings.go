package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/henriquesakata/cerne/internal/llm"
	"github.com/henriquesakata/cerne/internal/project"
)

// cliStatus is one subscription CLI and whether it is on PATH (F-0414).
type cliStatus struct {
	Name  string `json:"name"`
	Mode  string `json:"mode"`
	Found bool   `json:"found"`
}

// cliStatuses lists the three subscription CLIs in detection order.
func cliStatuses() []cliStatus {
	found := project.FoundSubscriptionCLIs()
	var out []cliStatus
	for _, name := range []string{"claude", "codex", "gemini"} {
		out = append(out, cliStatus{Name: name, Mode: name + "-cli", Found: slices.Contains(found, name)})
	}
	return out
}

// keyStatus is the state of an API key: the variable it lives in, whether one
// is set and where (env wins over the host file), and its last four characters
// when it is long enough for that to reveal nothing. The value itself never
// leaves the process. Problem names a credentials file that was refused.
type keyStatus struct {
	Env     string `json:"env"`
	Set     bool   `json:"set"`
	Source  string `json:"source,omitempty"`
	Last4   string `json:"last4,omitempty"`
	Problem string `json:"problem,omitempty"`
}

func keyStatusFor(env string) keyStatus {
	st := keyStatus{Env: env}
	v, source, err := project.ResolveAPIKey(env)
	if err != nil {
		st.Problem = err.Error()
		return st
	}
	if v == "" {
		return st
	}
	st.Set, st.Source = true, source
	if len(v) >= 12 {
		st.Last4 = v[len(v)-4:]
	}
	return st
}

// llmTestTimeout bounds the connection test; a CLI cold start is seconds.
var llmTestTimeout = 60 * time.Second

// loadLLMConfig is the project's [llm] (host values included), or the zero one
// in serve mode, where the per-project dir has no .cernerc yet.
func loadLLMConfig(projDir string) (project.LLMConfig, error) {
	cfg, _, err := project.LoadConfig(projDir)
	if err != nil && !errors.Is(err, project.ErrNoProject) {
		return project.LLMConfig{}, err
	}
	return cfg.LLM, nil
}

// handleLLMTest makes one minimal call in the requested mode (default: the
// resolved one) and answers {ok} or {ok:false,error} — the provider's own error
// already names the fix (login command, install hint, variable). Not a 5xx: a
// failed connection is the answer, not a server fault.
func handleLLMTest(w http.ResponseWriter, r *http.Request, projDir string) {
	var req struct {
		Mode string `json:"mode"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("invalid body: "+err.Error()))
			return
		}
	}
	if req.Mode != "" && !validLLMMode(req.Mode) {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid mode"))
		return
	}
	cfg, err := loadLLMConfig(projDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody(err.Error()))
		return
	}
	if req.Mode != "" {
		cfg.Mode = req.Mode
	}
	prov, err := newLLMProvider(cfg)
	if errors.Is(err, llm.ErrNoProviderForDelegation) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "delegation mode has nothing to test: the agent already running does the work"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTestTimeout)
	defer cancel()
	if _, err := prov.Complete(ctx, "Reply with the single word: ok"); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// keyEnvFor is the variable the key of provider (default: the configured one)
// lives in. "" for an unknown provider.
func keyEnvFor(cfg project.LLMConfig, provider string) string {
	if provider == "" || provider == cfg.WithDefaults().Provider {
		return cfg.WithDefaults().KeyEnv
	}
	if !slices.Contains([]string{"anthropic", "openai", "gemini", "openrouter", "compatible"}, provider) {
		return ""
	}
	return project.LLMConfig{Provider: provider}.WithDefaults().KeyEnv
}

// handleLLMKey writes (PUT) or removes (DELETE) the provider's key in the host
// credentials file and answers the key's status — never its value.
func handleLLMKey(w http.ResponseWriter, r *http.Request, projDir string) {
	if !sameOriginJSON(r) {
		writeJSON(w, http.StatusForbidden, errorBody("cross-origin or non-JSON request refused"))
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid request body"))
		return
	}
	cfg, err := loadLLMConfig(projDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody(err.Error()))
		return
	}
	env := keyEnvFor(cfg, req.Provider)
	if env == "" {
		writeJSON(w, http.StatusBadRequest, errorBody("unknown provider"))
		return
	}
	if r.Method == http.MethodPut {
		err = project.WriteCredential(env, strings.TrimSpace(req.Value))
	} else {
		err = project.DeleteCredential(env)
	}
	switch {
	case errors.Is(err, project.ErrCredentialValue), errors.Is(err, project.ErrCredentialName):
		writeJSON(w, http.StatusBadRequest, errorBody("the key must be one non-empty line"))
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorBody(err.Error()))
	default:
		writeJSON(w, http.StatusOK, keyStatusFor(env))
	}
}
