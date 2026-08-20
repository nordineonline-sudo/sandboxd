// OpenCode Web — an always-on, internal-only process each sandbox runs so the
// console can embed OpenCode's own native chat UI (see docs/agent-auth.md →
// "OpenCode Web"). It is NOT a manifest process: it never appears in
// sandbox.yaml, in GET /status's process list, or the console's Processes
// tab — it exists purely for the control plane's proxy to reach.
package main

import (
	"log/slog"
	"os"
	"path/filepath"
)

// opencodeWebPort is bound on 0.0.0.0 INSIDE the container so the control-plane
// proxy (a different container) can reach it over the shared sandboxd network
// by this container's bridge IP. It is still never exposed publicly: Traefik's
// only route for opencode hosts points at sandboxd:9000 (never this container),
// and the control-plane proxy authenticates every request before forwarding.
// See internal/api/opencodeweb.go.
const opencodeWebPort = "4097"

// ensureXdgOpenStub makes opencode web's "open a browser" step a no-op.
// runtimed runs headless — there is no browser — and without this the
// missing xdg-open binary crashes opencode's Bun runtime with ENOENT before
// it ever binds its port. The sandbox filesystem is --read-only except the
// bind-mounted HOME, so the stub is written there (already on PATH via
// /etc/profile.d/sandbox-env.sh) instead of baked into the image.
func ensureXdgOpenStub(home string, log *slog.Logger) {
	dir := filepath.Join(home, ".local", "bin")
	path := filepath.Join(dir, "xdg-open")
	if _, err := os.Stat(path); err == nil {
		return // already there (a previous boot, or an image that ships one)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Warn("opencode-web: mkdir .local/bin", "err", err.Error())
		return
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		log.Warn("opencode-web: write xdg-open stub", "err", err.Error())
	}
}

// newOpencodeWebProcess builds (but does not start) the supervised opencode-web
// process. appDir is the SAME workspace dir the "web" process and coding tasks
// use, so OpenCode's file tools, LSP, and session operate on this sandbox's own
// project — never another one (each sandbox is its own container, so this is
// structural, not something this code has to enforce).
//
// The password is read from RUNTIMED_OPENCODE_WEB_PASSWORD (set by the control
// plane at container create — see internal/api/handlers.go) and forwarded to
// `opencode web` under the env var IT reads, OPENCODE_SERVER_PASSWORD, via the
// shell command rather than a Go-level env override (process.go's runOnce
// inherits the container's env as-is, which already has the RUNTIMED_ var).
// Empty password → nil, the caller skips starting it: no control-plane key
// configured means no way to reach it anyway, so don't run an open server.
func newOpencodeWebProcess(appDir, runtimeDir string, log *slog.Logger) *process {
	if os.Getenv("RUNTIMED_OPENCODE_WEB_PASSWORD") == "" {
		return nil
	}
	cmd := `export OPENCODE_SERVER_PASSWORD="$RUNTIMED_OPENCODE_WEB_PASSWORD" OPENCODE_SERVER_USERNAME="opencode"; ` +
		`exec opencode web --hostname 0.0.0.0 --port ` + opencodeWebPort
	return newProcess("opencode-web", "internal", appDir, cmd, filepath.Join(runtimeDir, "opencode-web.log"), log)
}
