// Package agentauth is the foundation for "managed agent auth" (Phase 10B):
// a static registry of the coding-agent CLI providers and a host-side store
// of opaque credential directories. This A0 slice is READ-ONLY — it knows the
// providers, where their auth would live, and whether they're "connected"
// (a non-empty dir). It never creates provider dirs, parses tokens, or injects
// anything into a sandbox. Connect/import and per-task injection come later.
package agentauth

// Provider is one coding-agent CLI we can run as a task agent.
type Provider struct {
	ID     string // stable id used in the API ("opencode", "claude-code", "codex")
	Label  string // human label for the console
	Binary string // the CLI binary name, probed for "installed"
}

// registry is the fixed set of supported providers (owner-operated mode). It
// holds the coding-agent CLIs runtimed can drive AND any credential-only
// provider whose key the auth proxy injects on the wire (a model gateway that
// is reached through an agent's proxy path, not run as its own task agent).
var registry = []Provider{
	{ID: "opencode", Label: "OpenCode", Binary: "opencode"},
	{ID: "claude-code", Label: "Claude Code", Binary: "claude"},
	{ID: "codex", Label: "Codex", Binary: "codex"},
	// MiniMax is a credential-only provider: the owner connects a MiniMax API
	// key here, and the auth proxy injects it for the minimax upstreams. It has
	// no task-agent CLI and is never run by runtimed (Runnable=false), so the
	// console shows it as a credential-only entry, not a run picker option.
	{ID: "minimax", Label: "MiniMax", Binary: ""},

	// The following are additional credential-only model providers (same shape
	// as MiniMax above): the owner connects an API key in Settings, the auth
	// proxy injects it for that provider's upstream, and the model is used via
	// the "opencode" agent as "<id>/<model>" (see authproxy.upstreams and
	// cmd/runtimed/opencode.go's provider-prefixed routing). None of these run
	// their own task-agent CLI.
	//
	// Fully wired to the credential proxy (a stored key here is immediately
	// usable in a task/chat — see authproxy/proxy.go's creditOnlyProviders):
	{ID: "openai", Label: "OpenAI", Binary: ""},
	{ID: "deepseek", Label: "DeepSeek", Binary: ""},
	{ID: "openrouter", Label: "OpenRouter", Binary: ""},
	{ID: "cerebras", Label: "Cerebras", Binary: ""},
	{ID: "nvidia", Label: "NVIDIA", Binary: ""},
	{ID: "xai", Label: "xAI", Binary: ""},
	{ID: "ollama", Label: "Ollama Cloud", Binary: ""},
	{ID: "mistral", Label: "Mistral", Binary: ""},
	{ID: "vercel-ai-gateway", Label: "Vercel AI Gateway", Binary: ""},
	{ID: "huggingface", Label: "Hugging Face", Binary: ""},
	{ID: "zai", Label: "Z.AI", Binary: ""},
	// Google (Gemini API) is wired too, but with a header/response shape of
	// its own (x-goog-api-key, not Bearer) — see authproxy's isGoogle special
	// case and v1_agent_models.go's v1GoogleModels.
	{ID: "google", Label: "Google", Binary: ""},
	// Perplexity is wired for TASK EXECUTION (authproxy) but has no public
	// /models discovery endpoint — the model id is typed manually (e.g.
	// "perplexity/sonar-pro").
	{ID: "perplexity", Label: "Perplexity", Binary: ""},

	// Connectable here (key stored securely) but NOT YET routed by the auth
	// proxy — these need a non-static-bearer auth scheme (AWS SigV4, GCP
	// service account, Azure deployment/api-key headers, GitHub OAuth device
	// flow) or a per-account endpoint segment (Cloudflare AI Gateway needs an
	// account + gateway ID in the URL) that the generic bearer-token proxy
	// path doesn't cover yet. Connecting one here does not yet make it usable
	// in a task — use the OpenCode web "OpenCode" tab to connect and use
	// these directly inside the sandbox in the meantime.
	{ID: "amazon-bedrock", Label: "Amazon Bedrock", Binary: ""},
	{ID: "azure", Label: "Azure OpenAI", Binary: ""},
	{ID: "github-copilot", Label: "GitHub Copilot", Binary: ""},
	{ID: "cloudflare-ai-gateway", Label: "Cloudflare AI Gateway", Binary: ""},
}

// Providers returns a copy of the registry in display order.
func Providers() []Provider {
	out := make([]Provider, len(registry))
	copy(out, registry)
	return out
}

// Get returns a provider by id.
func Get(id string) (Provider, bool) {
	for _, p := range registry {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// runnable is the set of providers runtimed can actually run as a task agent.
// MUST be kept in sync with runtimed's selectAgent. All three have adapters;
// codex is runnable here but parked in the UI (its ChatGPT-subscription auth
// isn't proxyable yet), so the console hides it from the run picker.
var runnable = map[string]bool{
	"opencode":    true,
	"claude-code": true,
	"codex":       true,
}

// Runnable reports whether a provider has a runtimed task adapter.
func Runnable(id string) bool { return runnable[id] }

// credentialFiles maps a provider to the HOME-relative file its login writes the
// long-lived token to. Used as the opaque target for credential import (the
// "connect subscription" path) and the presence check. Every provider's own
// `<cli> login` produces one of these on the owner's machine; the owner pastes
// its contents in. The file is never opened or parsed.
var credentialFiles = map[string]string{
	"claude-code": ".claude/.credentials.json",
	"codex":       ".codex/auth.json",
	"opencode":    ".local/share/opencode/auth.json",
}

// CredentialFile returns the provider's credential file path (relative to HOME).
func CredentialFile(id string) (string, bool) {
	f, ok := credentialFiles[id]
	return f, ok
}

// apiKeyEnv maps a provider to the single environment variable its CLI reads an
// API key from. This is the ONE deliberate exception to runtimed's secret-env
// scrub: when an owner connects a provider by API key, runtimed injects just
// this var (from the stored key file) into the agent process — nothing else
// secret-shaped survives the scrub. opencode connects with an OpenCode (Zen)
// key against api.opencode.ai, so it maps to OPENCODE_API_KEY — NOT an Anthropic
// key (Zen is opencode's own gateway; the key is issued at opencode.ai).
var apiKeyEnv = map[string]string{
	"claude-code": "ANTHROPIC_API_KEY",
	"codex":       "OPENAI_API_KEY",
	"opencode":    "OPENCODE_API_KEY",
	// Credential-only providers (see registry above): no CLI reads these vars
	// directly (none run as a task agent), so the names below are advisory —
	// the real credential is read from the stored key file and injected by the
	// auth proxy on the wire (see authproxy/proxy.go).
	"minimax":               "MINIMAX_API_KEY",
	"openai":                "OPENAI_API_KEY",
	"deepseek":              "DEEPSEEK_API_KEY",
	"openrouter":            "OPENROUTER_API_KEY",
	"cerebras":              "CEREBRAS_API_KEY",
	"nvidia":                "NVIDIA_API_KEY",
	"xai":                   "XAI_API_KEY",
	"ollama":                "OLLAMA_API_KEY",
	"google":                "GOOGLE_GENERATIVE_AI_API_KEY",
	"amazon-bedrock":        "AWS_BEARER_TOKEN_BEDROCK",
	"azure":                 "AZURE_OPENAI_API_KEY",
	"github-copilot":        "GITHUB_COPILOT_TOKEN",
	"cloudflare-ai-gateway": "CLOUDFLARE_API_TOKEN",
	"vercel-ai-gateway":     "VERCEL_AI_GATEWAY_API_KEY",
	"huggingface":           "HF_TOKEN",
	"zai":                   "ZAI_API_KEY",
	"perplexity":            "PERPLEXITY_API_KEY",
	"mistral":               "MISTRAL_API_KEY",
}

// APIKeyEnv returns the env var name a provider's CLI reads its API key from.
func APIKeyEnv(id string) (string, bool) {
	e, ok := apiKeyEnv[id]
	return e, ok
}

// APIKeyFile is the HOME-relative file an API key is stored in (opaque, one line).
// Distinct from any credentialFile so the two auth methods never collide; each
// connect fully replaces the provider dir, so a provider holds exactly one method.
const APIKeyFile = ".sandboxd-apikey"
