// Package authproxy is the credential-injecting reverse proxy that lets a
// sandbox's coding agent reach its model provider WITHOUT the credential ever
// entering the sandbox. Every agent (claude-code, opencode, …) is pointed at
// this proxy per provider with a DUMMY key; the proxy — running control-plane
// side, holding the real credentials — strips the dummy auth and injects the
// real one on the wire, then forwards to the provider.
//
// Sandbox base URLs take the form `<proxy>/<agent>/<upstream>/…`, e.g.
// `<proxy>/opencode/zen/v1/chat/completions`. The proxy parses <agent> and
// <upstream>, resolves that agent's stored credential, injects it for the
// upstream, and forwards the remaining path. No credential — API key or OAuth
// token — is ever mounted or env-injected into the sandbox.
//
// Why: mounting/injecting the raw credential exposed it to the untrusted
// workspace AND let a CLI mutate/erase the shared file on a failed refresh.
// Keeping every credential here fixes both: the sandbox can neither read,
// exfiltrate, nor clobber it.
package authproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tastyeffectco/sandboxd/control-plane/internal/agentauth"
)

// oauthBeta is the anthropic-beta flag Claude Code sends in subscription (OAuth)
// mode — required for the OAuth bearer to be accepted on /v1/messages.
const oauthBeta = "oauth-2025-04-20"

// upstreams the proxy forwards to, keyed by the <upstream> segment of the
// sandbox base URL. A base path (e.g. /zen/v1) is preserved: the incoming path
// after /<agent>/<upstream> is appended to it.
var upstreams = map[string]string{
	"anthropic": "https://api.anthropic.com",
	"openai":    "https://api.openai.com/v1",
	"zen":       "https://opencode.ai/zen/v1",    // opencode's hosted gateway (pay-as-you-go)
	"zengo":     "https://opencode.ai/zen/go/v1", // opencode Zen "go" subscription
	// MiniMax direct endpoints — global (api.minimax.io) and China
	// (api.minimaxi.com), each in an OpenAI-compatible (/v1) and an
	// Anthropic-compatible (/anthropic) flavor. Their credential is the
	// connected MiniMax API key (see credFor), not the carrying agent's own.
	"minimax":              "https://api.minimax.io/v1",
	"minimax-cn":           "https://api.minimaxi.com/v1",
	"minimax-anthropic":    "https://api.minimax.io/anthropic",
	"minimax-anthropic-cn": "https://api.minimaxi.com/anthropic",
	// Additional credential-only providers (Settings → AI Agents), all
	// OpenAI-compatible bearer-token endpoints — same shape as MiniMax above,
	// generalized via creditOnlyProviders instead of one-off switch cases.
	// ("openai" reuses the pre-existing api.openai.com/v1 entry above.)
	"deepseek":   "https://api.deepseek.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"cerebras":   "https://api.cerebras.ai/v1",
	"nvidia":     "https://integrate.api.nvidia.com/v1",
	"xai":        "https://api.x.ai/v1",
	"ollama":     "https://ollama.com/v1",
	"mistral":            "https://api.mistral.ai/v1",
	"vercel-ai-gateway":  "https://ai-gateway.vercel.sh/v1",
	"huggingface":        "https://router.huggingface.co/v1",
	"zai":                "https://api.z.ai/api/paas/v4",
	// Perplexity has no /models discovery endpoint (see v1_agent_models.go's
	// modelCatalogUpstreams — it's deliberately absent there), but chat
	// completions work at the bare host with a standard Bearer key.
	"perplexity": "https://api.perplexity.ai",
	// Google (Gemini API / AI Studio) is OpenAI-INCOMPATIBLE: it needs an
	// "x-goog-api-key" header instead of "Authorization: Bearer" (see the
	// isGoogleUpstream special case in credFor below) and its /models
	// response has its own shape (handled in v1_agent_models.go).
	"google": "https://generativelanguage.googleapis.com/v1beta",
}

// creditOnlyProviders maps an <upstream> segment to the agentauth provider ID
// whose stored credential the proxy injects, REGARDLESS of which coding agent
// (opencode, claude-code, …) carries the request. These providers have no
// task-agent CLI of their own (Runnable=false in the registry) — they're
// reached through another agent (typically "opencode", via a
// provider-prefixed model id like "openai/gpt-4o-mini"), same as MiniMax.
var creditOnlyProviders = map[string]string{
	"minimax":              "minimax",
	"minimax-cn":           "minimax",
	"minimax-anthropic":    "minimax",
	"minimax-anthropic-cn": "minimax",
	"openai":               "openai",
	"deepseek":             "deepseek",
	"openrouter":           "openrouter",
	"cerebras":             "cerebras",
	"nvidia":               "nvidia",
	"xai":                  "xai",
	"ollama":               "ollama",
	"mistral":              "mistral",
	"vercel-ai-gateway":    "vercel-ai-gateway",
	"huggingface":          "huggingface",
	"zai":                  "zai",
	"perplexity":           "perplexity",
	"google":               "google",
}

// isMiniMaxUpstream reports whether <upstream> is one of the MiniMax direct
// endpoints — kept as a narrow helper for the one path (opencode's
// zero-friction free-tier fallback in the router below) that must never be
// rewritten to Zen even for a MiniMax request.
func isMiniMaxUpstream(up string) bool {
	switch up {
	case "minimax", "minimax-cn", "minimax-anthropic", "minimax-anthropic-cn":
		return true
	}
	return false
}

// Proxy injects the real provider credential into forwarded requests.
type Proxy struct {
	store *agentauth.Store
	log   *slog.Logger
}

// New builds the proxy over the agent-auth store (which holds every provider's
// credential). Returns nil if store is nil (proxy disabled).
func New(store *agentauth.Store, log *slog.Logger) *Proxy {
	if store == nil {
		return nil
	}
	return &Proxy{store: store, log: log}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	// /<agent>/<upstream>/<rest...>
	segs := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 3)
	if len(segs) < 2 {
		http.Error(w, "bad proxy path (want /<agent>/<upstream>/...)", http.StatusBadRequest)
		return
	}
	agent, up := segs[0], segs[1]
	// Credential-only upstreams (MiniMax + the generic bearer providers) are
	// never rewritten to the opencode free-tier path — each needs its OWN
	// connected key, never Zen's keyless free models.
	if _, ok := creditOnlyProviders[up]; !ok && agent == "opencode" && p.store.Method("opencode") == "" {
		up = "zen"
	}
	rest := "/"
	if len(segs) == 3 {
		rest = "/" + segs[2]
	}
	base, ok := upstreams[up]
	if !ok {
		http.Error(w, "unknown upstream: "+up, http.StatusBadRequest)
		return
	}
	inject, ok := p.credFor(agent, up)
	if !ok {
		// No usable/proxyable credential — a clear 401 so the task reads
		// "reconnect this agent" rather than a cryptic upstream error.
		http.Error(w, agent+" is not connected (or not proxyable) — connect it in Settings → AI Agents", http.StatusUnauthorized)
		return
	}
	target, _ := url.Parse(base)
	// Preserve the upstream's base path (e.g. /zen/v1) + the request suffix.
	r.URL.Path = strings.TrimRight(target.Path, "/") + rest
	r.URL.RawPath = ""
	inject(r.Header)
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		FlushInterval: -1, // stream SSE token-by-token
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if p.log != nil {
				p.log.Warn("authproxy: upstream error", "upstream", up, "err", err.Error())
			}
			http.Error(w, "upstream error", http.StatusBadGateway)
		},
		// Fail fast on permanent provider errors. Agent CLIs retry 401/429
		// for a long time, so an exhausted quota surfaced as a task that
		// "did not finish within the timeout" minutes later — a misleading
		// message for a problem the provider reported immediately. Rewriting
		// these as a 400 (which no agent retries) makes the task fail in
		// seconds with the provider's own words. Transient rate limits are
		// passed through untouched so normal backoff still works.
		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode < 400 {
				return nil // success and streaming responses are never touched
			}
			raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
			_ = resp.Body.Close()
			if err != nil {
				resp.Body = io.NopCloser(bytes.NewReader(nil))
				return nil
			}
			reason, terminal := terminalProviderError(resp.StatusCode, string(raw))
			if !terminal {
				resp.Body = io.NopCloser(bytes.NewReader(raw)) // unchanged
				resp.ContentLength = int64(len(raw))
				return nil
			}
			msg := up + ": " + reason
			if detail := providerMessage(raw); detail != "" {
				msg += " — " + detail
			}
			if p.log != nil {
				p.log.Warn("authproxy: permanent provider error (failing the task fast)",
					"upstream", up, "agent", agent, "status", resp.StatusCode, "detail", msg)
			}
			body := []byte(`{"error":{"type":"provider_error","message":` + strconv.Quote(msg) + `}}`)
			resp.StatusCode = http.StatusBadRequest
			resp.Status = "400 Bad Request"
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header = resp.Header.Clone()
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
			resp.Header.Del("Retry-After") // nothing to wait for
			return nil
		},
	}
	rp.ServeHTTP(w, r)
}

// credFor returns a header-injector for (agent, upstream) using the agent's
// stored credential, or ok=false when there's no usable/proxyable credential.
func (p *Proxy) credFor(agent, up string) (func(http.Header), bool) {
	// Credential-only providers (MiniMax + the generic bearer-token providers
	// added alongside it): regardless of which coding agent carries the
	// request, the proxy injects THAT provider's own connected API key. The
	// carrying agent's own credential is irrelevant here; none of these have
	// a task-agent CLI of their own.
	if providerID, ok := creditOnlyProviders[up]; ok {
		key := readTrim(filepath.Join(p.store.Dir(providerID), agentauth.APIKeyFile))
		if key == "" {
			return nil, false
		}
		if up == "google" {
			// Gemini API is OpenAI-incompatible: it reads its key from a
			// custom header, never Authorization.
			return func(h http.Header) {
				h.Del("Authorization")
				h.Set("x-goog-api-key", key)
			}, true
		}
		return func(h http.Header) {
			h.Del("X-Api-Key")
			h.Set("Authorization", "Bearer "+key)
		}, true
	}
	switch p.store.Method(agent) {
	case "api_key":
		key := readTrim(filepath.Join(p.store.Dir(agent), agentauth.APIKeyFile))
		if key == "" {
			return nil, false
		}
		return func(h http.Header) {
			h.Del("Authorization")
			h.Del("X-Api-Key")
			if up == "anthropic" {
				h.Set("X-Api-Key", key) // Anthropic API-key header
			} else {
				h.Set("Authorization", "Bearer "+key) // OpenAI / Zen (OpenAI-compatible)
			}
		}, true
	case "oauth":
		// Only claude-code's Anthropic OAuth is proxyable today (opencode/codex
		// OAuth/subscription formats are not — they connect by API key instead).
		if agent == "claude-code" && up == "anthropic" {
			tok := claudeOAuthToken(p.store)
			if tok == "" {
				return nil, false
			}
			return func(h http.Header) {
				h.Del("X-Api-Key")
				h.Del("Authorization")
				h.Set("Authorization", "Bearer "+tok)
				h.Set("anthropic-beta", mergeBeta(h.Get("anthropic-beta")))
			}, true
		}
	}
	// OpenCode zero-friction free tier: with no connected credential, opencode
	// still works out of the box on Zen's keyless free models. We forward with NO
	// auth (dropping the sandbox's dummy key) — the free models need none. This is
	// opencode-only; every other agent still requires a connected credential, so
	// they keep returning ok=false (a clear "connect it" 401). A connected key or
	// subscription is handled by the api_key/oauth cases above and takes over the
	// full paid catalog — this branch is reached only when nothing is connected.
	if agent == "opencode" {
		return func(h http.Header) {
			h.Del("Authorization")
			h.Del("X-Api-Key")
		}, true
	}
	return nil, false
}

// claudeOAuthToken reads the current subscription access token from the claude
// credential file. Opaque read; empty when absent/unparseable.
func claudeOAuthToken(store *agentauth.Store) string {
	b, err := os.ReadFile(filepath.Join(store.Dir("claude-code"), ".claude/.credentials.json"))
	if err != nil {
		return ""
	}
	var d struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if json.Unmarshal(b, &d) != nil {
		return ""
	}
	return d.ClaudeAiOauth.AccessToken
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// mergeBeta ensures the OAuth beta flag is present without dropping any the CLI
// already set (comma-joined, deduped).
func mergeBeta(existing string) string {
	if existing == "" {
		return oauthBeta
	}
	for _, f := range strings.Split(existing, ",") {
		if strings.TrimSpace(f) == oauthBeta {
			return existing
		}
	}
	return existing + "," + oauthBeta
}

// terminalProviderError classifies an upstream failure as permanent (the agent
// must not retry) and returns a plain-language reason. Credential and billing
// failures are always terminal. A 429 is terminal ONLY when the body says the
// quota/plan/credits are exhausted — a plain rate limit stays retryable so
// normal backoff keeps working.
func terminalProviderError(status int, body string) (string, bool) {
	switch status {
	case http.StatusUnauthorized:
		return "credentials rejected — reconnect this provider in Settings → AI Agents", true
	case http.StatusPaymentRequired:
		return "payment required — this provider account has no usable balance", true
	case http.StatusForbidden:
		return "access denied — the provider rejected this key (wrong plan, model, or region)", true
	case http.StatusTooManyRequests:
		low := strings.ToLower(body)
		for _, k := range []string{"usage limit", "quota", "credit", "billing", "insufficient", "balance", "upgrade your"} {
			if strings.Contains(low, k) {
				return "usage limit reached — add credits or upgrade the plan for this provider", true
			}
		}
		return "", false // transient rate limit: let the agent back off and retry
	}
	return "", false
}

// providerMessage digs the human-readable message out of a provider's error
// body. Providers differ ({"error":{"message":…}}, {"message":…}, {"error":…}),
// so try the common shapes and fall back to a trimmed snippet of the raw body.
func providerMessage(raw []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		if m := strings.TrimSpace(envelope.Error.Message); m != "" {
			return m
		}
		if m := strings.TrimSpace(envelope.Message); m != "" {
			return m
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
