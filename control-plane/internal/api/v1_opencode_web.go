// OpenCode Web proxy — the browser-facing edge of the embedded OpenCode UI.
//
// OpenCode's own web client resolves its API/server origin from
// `location.origin` with hardcoded absolute paths (/api/…, /event, /pty/…), so
// it CANNOT be served under a sub-path of the console (every absolute URL would
// escape the prefix). Instead each sandbox gets a dedicated host —
//   opencode-<id>.preview.<domain>
// routed by Traefik (file provider, see traefik/dynamic/opencode.yml) to this
// handler on sandboxd. The host discriminator means the browser hits the
// control plane at the ROOT of that host, exactly what the client expects.
//
// Auth: this dispatch point sits OUTSIDE the session-cookie auth middleware
// (the browser would not send the console cookie cross-origin, and it is gated
// by host in main.go's hostDispatch, mirroring the wake path). Instead it
// validates the per-sandbox `opencode web` password, carried one of two ways:
//   - the ?auth_token= query param the console put in the iframe src — the
//     exact token OpenCode's client natively understands (base64 "user:pass"),
//     so a page the client renders directly (initial load) is authorized, or
//   - an `Authorization: Basic` header (the client re-sends the decoded
//     password on every API/WS call after stripping auth_token from the URL).
// Either way the proxy stamps the canonical Basic header upstream and forwards;
// `opencode web` still validates it too (defense in depth).
//
// See opencodeweb.go for the trust-boundary trade-off this makes.
package api

import (
	"encoding/base64"
	"errors"
	"html"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/tastyeffectco/sandboxd/control-plane/internal/store"
)

// ServeOpencodeWebHost reverse-proxies any request whose Host is
// opencode-<id>.preview.<domain> to that sandbox's internal `opencode web`.
// It is reached via hostDispatch in main.go — BEFORE the auth middleware — so
// it must authenticate itself (the auth_token / Basic checks below).
func (s *Server) ServeOpencodeWebHost(w http.ResponseWriter, r *http.Request) {
	if len(s.OpencodeWebKey) == 0 {
		writeOpencodeWebPage(w, http.StatusServiceUnavailable, "opencode web is not enabled on this instance")
		return
	}
	id := parseOpencodeIDFromHost(r.Host, s.PreviewDomain)
	if id == "" {
		http.NotFound(w, r)
		return
	}
	sb, err := s.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeOpencodeWebPage(w, http.StatusNotFound, "no such sandbox")
		return
	}
	if err != nil {
		writeOpencodeWebPage(w, http.StatusInternalServerError, "internal error")
		return
	}
	if sb.Status != "running" {
		writeOpencodeWebPage(w, http.StatusConflict, "sandbox is "+sb.Status+" — start it first")
		return
	}
	ip := sb.ContainerIP.String
	if ip == "" {
		// The stored container_ip is only populated when the egress manager is
		// enabled; on an OSS build with no nftables the row stays NULL. Fall
		// back to asking Docker for the live bridge IP so the feature works
		// regardless of that config.
		if s.Docker != nil {
			if cj, err := s.Docker.Inspect(r.Context(), "s-"+id); err == nil {
				ip = cj.BridgeIP()
			}
		}
	}
	if ip == "" {
		writeOpencodeWebPage(w, http.StatusConflict, "sandbox has no container IP yet — try again shortly")
		return
	}

	// Authorize: the console embeds ?auth_token=<base64(opencode:<password>)>,
	// which the client then re-sends as a Basic header once it strips the query.
	// Accept either; both derive from the same per-sandbox password.
	//
	// Static assets are exempt: the SPA's <script>/<link> tags load them with NO
	// credential (the client only attaches auth_token to its fetch() calls), so
	// we pass them through unconditionally — they are the same public bundle
	// every sandbox ships, with no session data. The backend still needs the
	// canonical Basic header (it 401s bare asset requests), so the proxy stamps
	// it below regardless.
	if !opencodeWebStaticPath(r.URL.Path) && !opencodeWebAuthorized(r, opencodeWebPassword(s.OpencodeWebKey, id)) {
		writeOpencodeWebPage(w, http.StatusUnauthorized, "invalid opencode web token")
		return
	}

	target := &url.URL{Scheme: "http", Host: ip + ":" + strconv.Itoa(opencodeWebPort)}
	auth := opencodeWebBasicAuth(s.OpencodeWebKey, id)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			// Canonical credential upstream; the client's own header (if any)
			// is equivalent, but stamping ours keeps it correct even after a
			// master-key rotation mid-session.
			req.Header.Set("Authorization", auth)
		},
		// -1 disables the default periodic-flush buffering so SSE (the /event
		// and /api/event streams) reaches the browser immediately — the same
		// requirement the console's task-events SSE proxy has. WebSocket
		// upgrades (pty) are handled transparently by ReverseProxy.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			writeOpencodeWebPage(w, http.StatusBadGateway, "opencode web: "+err.Error())
		},
	}
	proxy.ServeHTTP(w, r)
}

// opencodeWebStaticPath reports whether the path is a static asset the SPA's
// markup loads without any credential (script/link/icon/manifest tags). These
// carry no session data, so the proxy lets them through without client auth;
// everything else (the page itself, /api/*, /event, /pty/*, …) must be
// authenticated.
func opencodeWebStaticPath(p string) bool {
	if strings.HasPrefix(p, "/assets/") {
		return true
	}
	switch p {
	case "/favicon.ico",
		"/favicon-v3.ico",
		"/favicon-v3.svg",
		"/favicon-96x96-v3.png",
		"/apple-touch-icon-v3.png",
		"/site.webmanifest",
		"/social-share.png":
		return true
	}
	return false
}

// opencodeWebAuthorized reports whether the request carries a valid credential
// for the given password: either an `Authorization: Basic` header decoding to
// opencode:<password>, or an `auth_token` query param decoding to the same.
func opencodeWebAuthorized(r *http.Request, password string) bool {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Basic ") {
		if user, pass, ok := decodeBasicAuth(h); ok && user == opencodeWebUser && pass == password {
			return true
		}
	}
	if tok := r.URL.Query().Get("auth_token"); tok != "" {
		if user, pass, ok := decodeAuthToken(tok); ok && user == opencodeWebUser && pass == password {
			return true
		}
	}
	return false
}

// decodeAuthToken decodes OpenCode's native auth_token query format
// (base64 "user:pass", user defaulting to "opencode"). Mirrors the web
// client's y1e().
func decodeAuthToken(tok string) (user, pass string, ok bool) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tok))
	if err != nil {
		return "", "", false
	}
	raw := string(b)
	user, pass = opencodeWebUser, raw
	if i := strings.Index(raw, ":"); i >= 0 {
		user, pass = raw[:i], raw[i+1:]
	}
	if user == "" {
		user = opencodeWebUser
	}
	return user, pass, pass != ""
}

// decodeBasicAuth splits a Basic header into user/pass.
func decodeBasicAuth(h string) (user, pass string, ok bool) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(h, "Basic ")))
	if err != nil {
		return "", "", false
	}
	raw := string(b)
	i := strings.Index(raw, ":")
	if i < 0 {
		return "", "", false
	}
	return raw[:i], raw[i+1:], true
}

// writeOpencodeWebPage renders a small HTML page for the iframe (the client is
// a browser UI, not a JSON API — a JSON body would be unreadable there).
func writeOpencodeWebPage(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte("<!doctype html><html><head><meta charset=\"utf-8\"><title>" +
		html.EscapeString(msg) + "</title></head><body style=\"font-family:system-ui;padding:32px;color:#444\"><h1>" +
		html.EscapeString(msg) + "</h1></body></html>\n"))
}