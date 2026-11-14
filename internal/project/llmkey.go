package project

// providerKeyEnv is the variable (and credentials-file name) each byo-key
// provider's key lives under (F-0414): one name per vendor, so saving or
// exporting one provider's key can never reach another provider's endpoint.
var providerKeyEnv = map[string]string{
	llmProviderAnthropic:  "ANTHROPIC_API_KEY",
	llmProviderOpenAI:     "OPENAI_API_KEY",
	llmProviderGemini:     "GEMINI_API_KEY",
	llmProviderOpenRouter: "OPENROUTER_API_KEY",
	llmProviderCompatible: "CERNE_COMPATIBLE_API_KEY",
}

// ProviderKeyEnv is the variable provider's key lives under; "" for a provider
// cerne does not know. The empty provider is the default, anthropic.
func ProviderKeyEnv(provider string) string {
	if provider == "" {
		provider = defaultLLMProvider
	}
	return providerKeyEnv[provider]
}

// ResolveKey finds the API key of the configured provider (c is the raw
// config, before WithDefaults): an explicit key_env alone, else the provider's
// own variable, then — for anthropic and openai only, F-0323's single shared
// name — the legacy CERNE_LLM_API_KEY. Each name is looked up in the process
// environment, then in the host credentials file. The compatible provider
// never reads the legacy name: its endpoint is not the vendor that key is for.
// env is the variable the key was found under, the provider's own when none was.
func (c LLMConfig) ResolveKey() (value, source, env string, err error) {
	// An explicit key_env is the user pinning one variable: nothing else is read.
	if c.KeyEnv != "" {
		v, src, err := ResolveAPIKey(c.KeyEnv)
		return v, src, c.KeyEnv, err
	}
	names := []string{}
	own := ProviderKeyEnv(c.Provider)
	if own != "" {
		names = append(names, own)
	}
	if p := c.Provider; p == "" || p == llmProviderAnthropic || p == llmProviderOpenAI {
		names = append(names, defaultLLMKeyEnv)
	}
	for _, n := range names {
		v, src, err := ResolveAPIKey(n)
		if err != nil || v != "" {
			return v, src, n, err
		}
	}
	return "", "", c.KeyVar(), nil
}

// KeyVar is the variable the key is read from first: the explicit key_env, else
// the provider's own.
func (c LLMConfig) KeyVar() string {
	if c.KeyEnv != "" {
		return c.KeyEnv
	}
	return ProviderKeyEnv(c.Provider)
}
