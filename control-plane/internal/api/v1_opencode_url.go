// GET /v1/sandboxes/{id}/opencode-url — the embeddable OpenCode web URL.
//
// The console renders its OpenCode iframe from this URL (a dedicated host, not
// a sub-path — see v1_opencode_web.go for why). The URL carries the per-sandbox
// ?auth_token= that OpenCode's web client natively understands. Only a caller
// who is already authenticated to this /v1 API (console session or API key) can
// mint one, so the token only ever reaches authorized browsers.
package api

import (
	"net/http"
)

// opencodeWebURL builds the iframe URL for a sandbox:
//   <scheme>://opencode-<id>.preview.<domain>[:port]/?auth_token=<base64>
// Scheme/port mirror previewURL() (plain http + host-facing port unless TLS),
// so the embed works on any PREVIEW_DOMAIN / published-port deployment.
func (s *Server) opencodeWebURL(id string) string {
	scheme := "http"
	defaultPort := "80"
	if s.PreviewTLS {
		scheme = "https"
		defaultPort = "443"
	}
	host := "opencode-" + id + ".preview." + s.PreviewDomain
	if p := s.PublicHTTPPort; p != "" && p != defaultPort {
		host += ":" + p
	}
	return scheme + "://" + host + "/?auth_token=" + opencodeWebAuthToken(s.OpencodeWebKey, id)
}

// v1OpencodeURL serves GET /v1/sandboxes/{id}/opencode-url.
func (s *Server) v1OpencodeURL(w http.ResponseWriter, r *http.Request) {
	if len(s.OpencodeWebKey) == 0 {
		writeV1Err(w, http.StatusServiceUnavailable, "unavailable", "opencode web is not enabled on this instance")
		return
	}
	id := r.PathValue("id")
	if !isULID(id) {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such sandbox")
		return
	}
	sb, err := s.Store.Get(r.Context(), id)
	if err != nil {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such sandbox")
		return
	}
	if sb.Status != "running" {
		writeV1Err(w, http.StatusConflict, "conflict", "sandbox is "+sb.Status+" — start it first")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": s.opencodeWebURL(id)})
}