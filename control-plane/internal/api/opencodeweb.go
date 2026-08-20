// OpenCode Web embed — sandboxd runs `opencode web` inside every sandbox
// (see cmd/runtimed/opencodeweb.go) so the console can embed OpenCode's own,
// native chat/session UI instead of maintaining a bespoke one. That gives every
// project the full OpenCode experience — its real provider/model catalog,
// whatever the owner has connected (GitHub Copilot, Anthropic, OpenRouter,
// local Ollama, …) — with ZERO sandboxd-side provider integration work.
//
// Isolation is structural, not a policy we have to enforce: each sandbox is
// its own container with its own filesystem, so one project's OpenCode
// instance/session/auth can never see or affect another's.
//
// Trade-off (explicit, by design — see docs/agent-auth.md): unlike every other
// agent integration in sandboxd, the provider credential here lives INSIDE the
// sandbox (OpenCode's own ~/.local/share/opencode/auth.json), not behind the
// control-plane's credential-injecting proxy. It never lands in the workspace
// tree, so it's never in a git commit or a snapshot — but it is not proxied,
// and reconnecting a provider is per-project (by design: no auth is ever
// copied between sandboxes).
package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// opencodeWebPort is the fixed port runtimed binds `opencode web` to INSIDE
// every sandbox container. It binds 0.0.0.0 (not loopback) so the control-plane
// proxy can reach it over the shared sandboxd network by the container's bridge
// IP — the proxy is the ONLY path to it: Traefik has no route to the sandbox for
// this host (the opencode.yml file router points at sandboxd:9000, never the
// container), and every peer that could reach it still needs the per-sandbox
// password it validates natively. The control plane never exposes it publicly.
const opencodeWebPort = 4097

// opencodeWebUser is the fixed HTTP Basic username `opencode web` accepts
// (OPENCODE_SERVER_USERNAME defaults to this too — set explicitly so a future
// opencode default change can't silently break the proxy).
const opencodeWebUser = "opencode"

// LoadOpencodeWebKey loads (or generates and persists) the 32-byte master key
// used to derive each sandbox's `opencode web` password. Mirrors internal/
// secrets' keyfile pattern (env override, else keyfile, else generate+0600)
// but doesn't need AEAD — HMAC only — so it lives here rather than growing
// that package's scope. Call once at startup; nil+error disables the feature
// (Server.OpencodeWebKey stays nil, callers treat that as "off").
func LoadOpencodeWebKey(envKey, keyfilePath string) ([]byte, error) {
	if envKey != "" {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(envKey))
		if err != nil {
			return nil, fmt.Errorf("SANDBOXD_OPENCODE_WEB_KEY is not valid base64: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("SANDBOXD_OPENCODE_WEB_KEY must decode to 32 bytes, got %d", len(key))
		}
		return key, nil
	}
	if b, err := os.ReadFile(keyfilePath); err == nil {
		key, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if derr != nil || len(key) != 32 {
			return nil, fmt.Errorf("opencode-web keyfile %s is corrupt (want base64 of 32 bytes)", keyfilePath)
		}
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyfilePath), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyfilePath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, fmt.Errorf("write opencode-web keyfile: %w", err)
	}
	return key, nil
}

// opencodeWebPassword derives sandbox id's `opencode web` HTTP Basic password:
// HMAC-SHA256(masterKey, sandboxID), hex, truncated to 32 chars. Deterministic
// so nothing new needs storing or migrating — the control plane recomputes it
// on every proxied request, and the exact same value is handed to runtimed as
// RUNTIMED_OPENCODE_WEB_PASSWORD at `docker run` time.
func opencodeWebPassword(masterKey []byte, sandboxID string) string {
	mac := hmac.New(sha256.New, masterKey)
	mac.Write([]byte(sandboxID))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// opencodeWebBasicAuth builds the "Basic base64(user:pass)" header value the
// proxy injects server-side before forwarding to the sandbox. The browser may
// send its own (decoded from the ?auth_token= the console embedded), but the
// proxy always stamps the canonical value so a missing/stale browser header
// can't break a valid session.
func opencodeWebBasicAuth(masterKey []byte, sandboxID string) string {
	raw := opencodeWebUser + ":" + opencodeWebPassword(masterKey, sandboxID)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// opencodeWebAuthToken is the value the console puts in the iframe URL as
// ?auth_token= — the token OpenCode's web client natively understands (it
// base64-decodes "opencode:<password>" and uses it as its HTTP Basic
// credential; see the bundle's y1e()/Am()). The control-plane handler validates
// the same token before proxying, so a stale or leaked query param grants
// nothing on its own.
func opencodeWebAuthToken(masterKey []byte, sandboxID string) string {
	raw := opencodeWebUser + ":" + opencodeWebPassword(masterKey, sandboxID)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// parseOpencodeIDFromHost extracts the sandbox id from an OpenCode web host
// (opencode-<id>.preview.<domain>[:port]). Returns "" for anything else. Mirrors
// wake.Handler's preview-host parse: same regex shape, different prefix.
func parseOpencodeIDFromHost(host, domain string) string {
	if domain == "" {
		return ""
	}
	if !strings.HasPrefix(host, "opencode-") {
		return ""
	}
	rest := strings.TrimPrefix(host, "opencode-")
	// Strip an optional :port.
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	want := "preview." + domain
	if !strings.HasSuffix(rest, "."+want) {
		return ""
	}
	id := strings.TrimSuffix(rest, "."+want)
	// Browsers lowercase hostnames before sending them, so the parsed ULID
	// may arrive all-lowercase. ULIDs are case-insensitive Crockford base32;
	// normalise to upper so store lookups and path derivation match the
	// canonical form.
	id = strings.ToUpper(id)
	if !isULID(id) {
		return ""
	}
	return id
}
