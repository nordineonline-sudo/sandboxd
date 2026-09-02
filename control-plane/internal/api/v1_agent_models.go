// v1_agent_models.go — GET /v1/agents/{provider}/models: a best-effort,
// read-only model catalog for the credential-only "model gateway" providers
// (Settings → AI Agents; see docs/agent-auth.md). Most expose the same
// standard OpenAI-compatible discovery endpoint, "GET <base>/models" ->
// {"data":[{"id":"..."}]} — this handler calls it directly from the control
// plane (never through a sandbox), injecting the connected API key as a
// Bearer header when one is stored. Several upstreams (OpenCode Zen,
// OpenRouter, NVIDIA, Hugging Face) also answer without one (a public/
// free-tier catalog). Google (Gemini API) is handled as a special case: a
// different header (x-goog-api-key) and a different response shape.
//
// This performs a plain read-only discovery GET with the SAME key material
// agentauth already holds server-side — never returned to the browser, never
// mounted in a sandbox. Kept in sync with authproxy.upstreams; a provider not
// listed here (Amazon Bedrock, Azure, GitHub Copilot, Cloudflare AI Gateway —
// they need request signing, extra connection fields, or an OAuth device
// flow the generic bearer path doesn't cover) returns 400 — the console falls
// back to its free-text model field for those. Perplexity is wired for task
// execution (authproxy) but has no public /models endpoint, so it's
// deliberately absent here too.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tastyeffectco/sandboxd/control-plane/internal/agentauth"
)

// modelCatalogUpstreams maps a provider id to its OpenAI-compatible base URL.
var modelCatalogUpstreams = map[string]string{
	"opencode":          "https://opencode.ai/zen/v1",
	"minimax":           "https://api.minimax.io/v1",
	"openai":            "https://api.openai.com/v1",
	"deepseek":          "https://api.deepseek.com/v1",
	"openrouter":        "https://openrouter.ai/api/v1",
	"cerebras":          "https://api.cerebras.ai/v1",
	"nvidia":            "https://integrate.api.nvidia.com/v1",
	"xai":               "https://api.x.ai/v1",
	"ollama":           "https://ollama.com/v1",
	"mistral":           "https://api.mistral.ai/v1",
	"vercel-ai-gateway": "https://ai-gateway.vercel.sh/v1",
	"huggingface":       "https://router.huggingface.co/v1",
	"zai":               "https://api.z.ai/api/paas/v4",
}

type v1AgentModel struct {
	ID string `json:"id"` // "<provider>/<model-id>" — pass straight back as a task's model field
}

// v1AgentModels — GET /v1/agents/{provider}/models.
func (s *Server) v1AgentModels(w http.ResponseWriter, r *http.Request) {
	p, ok := s.agentProvider(w, r)
	if !ok {
		return
	}
	if p.ID == "google" {
		s.v1GoogleModels(w, r, p)
		return
	}
	base, ok := modelCatalogUpstreams[p.ID]
	if !ok {
		writeErr(w, http.StatusBadRequest, "model listing isn't wired for this provider yet — enter the model id directly")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not build the discovery request")
		return
	}
	if key := s.storedAPIKey(p.ID); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not reach "+p.Label+": "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // OpenRouter's catalog alone is ~700 KiB
	if resp.StatusCode >= 400 {
		writeErr(w, http.StatusBadGateway, p.Label+" rejected the request (status "+resp.Status+") — check the connected key in Settings")
		return
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		writeErr(w, http.StatusBadGateway, p.Label+" returned an unexpected response")
		return
	}
	out := make([]v1AgentModel, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID == "" {
			continue
		}
		// Every upstream (including OpenCode Zen) returns bare ids ("gpt-4o",
		// "claude-fable-5", …) — prefix with the connect-time provider id so
		// the console/task API gets "<provider>/<model-id>" straight back.
		out = append(out, v1AgentModel{ID: p.ID + "/" + m.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": out})
}

// v1GoogleModels — Gemini API special case: "x-goog-api-key" header (never
// Authorization) and a "{"models":[{"name":"models/gemini-…"}]}" response
// shape, both unlike every other (OpenAI-compatible) gateway above.
func (s *Server) v1GoogleModels(w http.ResponseWriter, r *http.Request, p agentauth.Provider) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://generativelanguage.googleapis.com/v1beta/models", nil)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not build the discovery request")
		return
	}
	if key := s.storedAPIKey(p.ID); key != "" {
		req.Header.Set("x-goog-api-key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not reach Google: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		writeErr(w, http.StatusBadGateway, "Google rejected the request (status "+resp.Status+") — check the connected key in Settings")
		return
	}
	var parsed struct {
		Models []struct {
			Name string `json:"name"` // "models/gemini-2.5-flash"
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		writeErr(w, http.StatusBadGateway, "Google returned an unexpected response")
		return
	}
	out := make([]v1AgentModel, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		id := strings.TrimPrefix(m.Name, "models/")
		if id == "" {
			continue
		}
		out = append(out, v1AgentModel{ID: "google/" + id})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": out})
}

// storedAPIKey reads the provider's stored API key (opaque, one line), or ""
// if none is connected — mirrors authproxy's own credFor lookup, without
// pulling in that package's HTTP-proxy machinery for a single read.
func (s *Server) storedAPIKey(providerID string) string {
	if s.AgentAuth == nil || s.AgentAuth.Method(providerID) != "api_key" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(s.AgentAuth.Dir(providerID), agentauth.APIKeyFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
