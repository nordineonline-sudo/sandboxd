<h1 align="center">sandboxd <small>· fork <code>nordineonline-sudo</code></small></h1>

<p align="center">
  <b>Open-source, self-hosted AI app builder.</b><br/>
  Prompt it and a coding agent builds real apps in isolated sandboxes on <b>your</b>
  server — each live at a preview URL. You own the infra, code, and data. MIT.
</p>

<p align="center">
  <a href="https://github.com/nordineonline-sudo/sandboxd/stargazers"><img alt="Star sandboxd" src="https://img.shields.io/github/stars/nordineonline-sudo/sandboxd?style=for-the-badge&label=%E2%98%85%20Star&color=333"></a>
  &nbsp;
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-green.svg"></a>
  <img alt="Runs on Docker" src="https://img.shields.io/badge/runs%20on-Docker-2496ED.svg">
  <a href="https://github.com/nordineonline-sudo/sandboxd/commits/main"><img alt="Version" src="https://img.shields.io/badge/version-0.4.1--nordineonline.3-00ADD8"></a>
</p>

---

> **This is the `nordineonline-sudo/sandboxd` fork** of
> [`tastyeffectco/sandboxd`](https://github.com/tastyeffectco/sandboxd). It ships
> everything from upstream **plus the changes documented in
> [Changes in this fork](#changes-in-this-fork)**. The engine, the API, and the
> self-hosted model are unchanged: one Go binary driving Docker, Traefik for URLs,
> SQLite for state.

## What is sandboxd?

The apps where you type *"build me a todo app"* and a working site appears at its
own link — Lovable, Bolt, v0, Replit. **sandboxd is the open-source engine that
makes that work, on your own server.** One HTTP request and it:

1. spins up a **private, isolated container** (its own filesystem + limits),
2. runs an **AI coding agent inside it** against your prompt, and
3. hands the app a **live preview URL**.

Idle sandboxes **sleep and wake on demand**, so one ordinary box holds many apps
instead of a VM each. Under the hood it's deliberately small: **one Go program
driving Docker**, Traefik for URLs, SQLite for state — no Kubernetes, no separate
database, no queue.

## Two ways to use it

- **API-first** — everything is a `/v1` call, so you can build sandboxd into your
  own product.
- **Optional web console** — the fastest, **no-code** way to use it hands-on:
  create an app, chat to a coding agent, watch the live preview, edit files, and
  review a git diff & push — all in a browser. It's a **pure `/v1` client**; the
  engine runs perfectly headless without it.

> **New here? Start with the console. Building a product? Drive the API.**

## What can you run? (a lot — from one console, no code)

The console isn't only for coders. Open it and, in **one click or one prompt**, you can:

- **🚀 Run a ready-made open-source app** — a **Ghost** blog, **n8n** automations, a **Gitea** git host, **Grafana** / **Metabase** dashboards, **Uptime Kuma**, **Jupyter**, **Keycloak**… **80+ curated apps**, installed and live at their own URL.
- **🧩 Start from a starter** — a React/Vite, Next.js, or FastAPI scaffold that boots to a live preview; then just *chat* to shape it.
- **📥 Bring your own repo** — import any **public** Git repo (no credential needed) and let a coding agent work on it.
- **✨ Build from scratch** — describe an app and watch the agent build it in the live preview, then commit &amp; push.
- **🤖 Chat with the real OpenCode** — a dedicated **agent** tab (the app page's
  first, default tab) embeds OpenCode's native web UI **full page width**
  (per-app sandbox, full session/terminal/file editing), replacing the old
  bespoke chat. See [Changes in this fork](#changes-in-this-fork) for how this
  fork wires OpenCode's web UI in and makes your conversations follow you across
  devices.

<p align="center">
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/ghost.webp" height="38" alt="Ghost" title="Ghost" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/n8n.svg" height="38" alt="n8n" title="n8n" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/directus.svg" height="38" alt="Directus" title="Directus" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/gitea.svg" height="38" alt="Gitea" title="Gitea" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/grafana.svg" height="38" alt="Grafana" title="Grafana" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/metabase.svg" height="38" alt="Metabase" title="Metabase" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/code-server.webp" height="38" alt="code-server" title="code-server" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/jupyter.svg" height="38" alt="Jupyter" title="Jupyter" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/keycloak.svg" height="38" alt="Keycloak" title="Keycloak" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/pocketbase.svg" height="38" alt="PocketBase" title="PocketBase" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/uptime-kuma.svg" height="38" alt="Uptime Kuma" title="Uptime Kuma" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/vikunja.svg" height="38" alt="Vikunja" title="Vikunja" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/wikijs.svg" height="38" alt="Wiki.js" title="Wiki.js" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/memos.webp" height="38" alt="Memos" title="Memos" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/meilisearch.svg" height="38" alt="Meilisearch" title="Meilisearch" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/qdrant.svg" height="38" alt="Qdrant" title="Qdrant" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/open-webui.svg" height="38" alt="Open WebUI" title="Open WebUI" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/navidrome.svg" height="38" alt="Navidrome" title="Navidrome" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/audiobookshelf.svg" height="38" alt="Audiobookshelf" title="Audiobookshelf" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/actualbudget.svg" height="38" alt="Actual Budget" title="Actual Budget" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/prefect.svg" height="38" alt="Prefect" title="Prefect" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/marimo.svg" height="38" alt="marimo" title="marimo" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/ntfy.svg" height="38" alt="ntfy" title="ntfy" hspace="7" />
  <img src="https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/console/public/app-icons/gotify.svg" height="38" alt="Gotify" title="Gotify" hspace="7" />
</p>

<p align="center">
  <i><b>This is just a taste — if it's open-source, you can almost certainly run it.</b><br/>
  Any Node, Python, or static-binary app boots as-is; anything else, bring your own base image or preset.</i>
</p>

## Changes in this fork

This fork tracks upstream `main` and layers the following changes on top. Each is
also recorded in the [`CHANGELOG`](CHANGELOG.md).

### 1. Embedded OpenCode web console (per-app)
*Commits `aeab961`, plus the control-plane proxy pieces.*

The app page now has a dedicated **agent** tab (its first, default tab) that
renders **OpenCode's native web UI** in a full-page-width iframe — the old
bespoke chat is gone:

- The agent tab replaces the chat panel that used to sit on the **Overview**
  tab (which is now preview + processes + runtime, full width).
- Each sandbox gets a dedicated **`opencode-<id>.preview.<domain>`** host, served
  through the same Traefik edge as the app previews.
- The control plane reverse-proxies that host into the sandbox's internal
  `opencode web` (`control-plane/internal/api/v1_opencode_web.go`): it validates
  a **per-sandbox auth token** (`?auth_token=` or Basic auth), passes static
  assets through, and streams SSE / websocket (pty) traffic.
- The token is minted by the control plane and surfaced to the console via a new
  endpoint, **`GET /v1/sandboxes/{id}/opencode-url`**.
- The proxy is **CSP-aware**: injected scripts are allow-listed in the page's
  Content-Security-Policy by **sha256 hash**, so the embedded UI works without
  relaxing the sandbox's headers.
- `docs/openapi.yaml` documents the new endpoint; the `runtimed` supervisor
  launches `opencode web` inside the sandbox.

### 2. Conversations follow you across devices
*Commit `3712096`.*

OpenCode's web client keeps its **project registry in the browser's localStorage**
— so a fresh device used to show *"Nothing here yet"* even though the app already
had real sessions. This fork fixes that end-to-end:

- On first load, the control-plane proxy **pre-seeds the app workspace into the
  client's project store** (`opencode.global.dat:server`, scope `local`) and sets
  `home.selection`, so the sandbox's workspace appears in the sidebar.
- Seeding only happens **if the store is empty** — a user's own projects are never
  overwritten.
- Result: open a sandbox from **any device** and you immediately see the same
  workspace and conversation history.

### 3. Mobile responsiveness
*Commits `942626a`, `28745f0`.*

- The **Overview / Files / Git layouts and the top bar** adapt to small screens
  (single-column grid, touch-friendly).
- Fixed a **mobile-only overflow bug** where wide content was silently clipped in
  the single-column layout.

### 4. Better chat input on mobile
*Commits `0d15b67`, `223e302`.*

- The agent chat textarea is now **multiline and auto-growing**.
- On mobile, **Enter inserts a newline instead of sending** — touch keyboards
  have no Shift key, so the old behavior made multi-line messages impossible.
  Desktop behaviour is unchanged (Enter sends, Shift+Enter inserts a newline).

### 5. Dedicated agent + README tabs
*Release `0.4.1-nordineonline.1`.*

- **Agent is its own tab, first and default.** The embedded OpenCode UI moved out
  of the Overview into a dedicated **agent** tab that is **full page width** and
  sized to the exact viewport height left below the header (dynamically
  measured, so no page scroll). It is full-bleed on mobile too.
- **New README tab.** A brain-style **view/edit** tab for the app's root
  `README.md` — a **normal workspace file, committed to git** like any other
  source (the brain's `BRAIN.md` is git-excluded by design). Empty projects get a
  **"Create README.md"** button that seeds a template.
- **E2E-friendly markup.** `Card`/`Btn` now forward `data-testid`, so browser
  tests can target the agent and README tabs reliably.

### 6. Real file manager in the Files tab
*Release `0.4.1-nordineonline.2`.*

The Files tab is a full workspace file manager now (it used to be tree +
editor only):

- **Upload** one or many files — via the button **or drag & drop**, folders
  included (dropped folders are walked and uploaded with their tree). New
  folders created by an upload expand automatically.
- **Download** any file individually, any **folder as a zip**, or the whole
  workspace (the pre-existing export).
- **Create** files and folders, **rename** (in place), **delete** (files, or
  folders recursively) — with confirmations where it hurts.
- **Preview images inline** (png / jpg / gif / webp / svg); text files keep
  the syntax-highlighted editor; binary or oversized files offer a download.
- Server side: six new host-side endpoints (download / archive / upload /
  mkdir / delete / rename) documented in [`docs/openapi.yaml`](docs/openapi.yaml).
  Every path is confined to the app dir and symlink-checked component by
  component (CWE-59); uploads are atomic, size-capped and audited.

### 7. Custom agent instructions (global)
*Release `0.4.1-nordineonline.3`.*

Customize how the coding agents behave for the whole instance, from
**Settings → Agent instructions (custom)** — no rebuild, no redeploy:

- A textarea saving an optional prompt suffix, persisted per instance
  (`PATCH /v1/settings` accepts `agents.system_prompt`; 8&nbsp;KiB cap).
- It is appended **after** the embedded platform briefing with a delimiter —
  the built-in guardrails stay intact — and rendered with the same per-sandbox
  placeholders (`{{APP_DIR}}`, `{{PORT}}`, `{{HEALTH_PATH}}`,
  `{{LOCAL_URL}}`).
- Applies to the **next** tasks on every sandbox (live settings, read at task
  submit by the control plane and passed to runtimed).

### Where the fork differs operationally

- **Versioning** is pinned for the deployment: images are tagged
  `sandboxd-base` / `sandboxd-control-plane` / `sandboxd-console` at
  `0.4.1-nordineonline.<n>` (no floating `latest`). Bump the tag in
  [`docker-compose.yml`](docker-compose.yml), `console/package.json` and
  [`image/README.md`](image/README.md) together when you release.
- The install/deploy scripts below install **this** repo, not upstream.

## Quick start

Needs **Docker + the Compose plugin** and **git** on Linux (macOS via Docker
Desktop is best-effort). Runs natively on **amd64 and arm64**. Install in one line:

```bash
curl -fsSL https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/install.sh | bash
```

It builds the images, starts the stack **with the web console**, and prints your
**console URL + a generated login** — no password step. Open it, connect an agent
under **Settings**, create an app, and build. No code needed.

- **Console:** `http://console.localhost` — the installer prints your login; lost
  it? run **`./console-login.sh`** to see it again anytime
- **API:** `http://127.0.0.1:9090` (`curl http://127.0.0.1:9090/healthz` → `ok`)
- **Headless (no console):** run with `SANDBOXD_CONSOLE=0` (or `--no-console`)
- **Upgrade later:** run **`./upgrade.sh`** — it backs up your database first,
  health-checks the new version, and rolls back automatically if it fails
  ([Upgrading](docs/upgrading.md)). `./upgrade.sh --check` shows your version.

Prefer the API? Connect an agent once, create a sandbox, hand it a prompt:
```bash
API=http://127.0.0.1:9090
curl -s -XPOST $API/v1/agents/claude-code/api-key -d '{"api_key":"sk-ant-..."}'
ID=$(curl -s -XPOST $API/sandbox -d '{"ports":[3000]}' | sed -E 's/.*"id":"([^"]+)".*/\1/')
curl -s -XPOST $API/v1/sandboxes/$ID/tasks -d '{"prompt":"build a todo app on port 3000","agent":"opencode"}'
# open the result at  http://s-$ID-3000.preview.localhost
```

## 🚀 Deploy to a VPS in one click

sandboxd needs one Linux server with Docker — nothing else. Paste the repo's
[cloud-init file](deploy/cloud-init.yaml) at creation, or run the bootstrap on the
fresh server:

```bash
curl -fsSL https://raw.githubusercontent.com/nordineonline-sudo/sandboxd/main/deploy/bootstrap.sh | sudo bash
```

Full per-provider walkthrough: [deploy/DEPLOY.md](deploy/DEPLOY.md).

## What you get

- **Isolated sandboxes** — a hardened container per app with a workspace + live
  preview URL; sleep/wake so idle apps cost nothing.
- **Built-in agents** — OpenCode & Claude Code. **No credential ever enters a
  sandbox** (a proxy injects it on the wire), and every task is **checkpointed &
  revertible**.
- **Runtime presets** — React/Vite, Next.js, Node/Express, FastAPI, Worker; boot
  to a preview and reload after agent edits.
- **Files, Git & secrets** — in-browser editor + diffs, commit & push, per-app
  config/secrets (encrypted, write-only).
- **Snapshots / fork / restore**, an activity timeline, per-process logs, and live
  lifecycle tuning.

## Who's it for?

**✅ Use it** if you run **many sandboxes for other people** — an AI app-builder,
an agent platform, a coding playground, per-user or per-branch preview
environments, or team multi-app hosting.

**❌ Skip it** if you just need one or two containers for yourself — a shell
script or `docker run` is simpler.

## Documentation

- [`AGENTS.md`](AGENTS.md) — copy-pasteable runbook for building on sandboxd from your own agent
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — how the control plane, edge and sandboxes fit together
- [`docs/`](docs/) — configuration, upgrading, production/TLS
- Upstream docs: [sandboxd.io](https://sandboxd.io) (covers the engine; this fork's changes are listed above)

## Changelog

All releases are tracked in [`CHANGELOG.md`](CHANGELOG.md). The fork releases as
`0.4.1-nordineonline.<n>`.

| Version | Highlights |
| --- | --- |
| `0.4.1-nordineonline.3` | **Custom agent instructions** (Settings → Agent instructions (custom)) — global prompt suffix, persisted, appended to every next task. |
| `0.4.1-nordineonline.2` | **Real file manager** in the Files tab — upload (multi + drag & drop folders), download file/folder zip, mkdir/rename/delete, image preview. |
| `0.4.1-nordineonline.1` | **Dedicated agent tab** (first, default, full width) + **README tab** (view/edit). |
| `0.4.0-nordineonline.2` | Production release: pinned tags, README rewrite, OpenCode-web fix on wake/self-heal. |
| `0.4.0-nordineonline.1` | Embedded **OpenCode web** console, mobile responsiveness, cross-device sessions. |

## License

[MIT](LICENSE). Use it, ship it, sell what you build on it.