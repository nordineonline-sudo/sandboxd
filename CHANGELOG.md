# Changelog

All notable changes to sandboxd are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and the project follows
[Semantic Versioning](https://semver.org/) (pre-1.0: **rolling releases bump the
patch** — each lands the meaningful changes merged since the last one — and a
**minor bump marks a milestone** release).

## [0.4.1-nordineonline.10] — 2026-09-02

* feat(providers): **Ollama Cloud** wired end-to-end — added to the agentauth
  registry + credential proxy upstream (`https://ollama.com/v1`),
  `creditOnlyProviders`/`additionalProviderUpstreams`, the dynamic model
  catalog (`GET /v1/agents/ollama/models`, 19 public models, no key needed to
  list), and the console provider pickers. Usable from the chat as
  `ollama/<model-id>` once a key is connected in Settings, by @nordineonline-sudo (fork)
* feat(console): **GitHub Copilot** added to the chat's provider list; listed
  for visibility, but its OAuth-device auth isn't proxyable so it stays
  usable in the OpenCode (native) tab, by @nordineonline-sudo (fork)
* docs: README (release-history + section 8 table), CHANGELOG,
  by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.9] — 2026-09-02

* feat(console): **sidebar overhaul** — removed the non-functional **Brain** and
  **App Store** nav entries and the **Feedback** footer link; the **GitHub**
  link now points to the fork repo (`nordineonline-sudo/sandboxd`); below
  Settings a separator then the **app list with live status** (running apps
  first); clicking an app opens its dashboard and reveals a **submenu** with
  Start/Stop (real sandbox actions), Open ↗, and every app tab (agent,
  OpenCode, overview, brain, readme, files, git, config, terminal, snapshots,
  activity) so the in-app tab bar is no longer needed, by @nordineonline-sudo (fork)
* feat(console): removed the **top-right search / ⌘K palette**, the **in-app
  horizontal tab bar**, and the **Deploy** button (not functional yet),
  by @nordineonline-sudo (fork)
* feat(console): the app menu's "Copy preview URL" is now **"QRCode URL"** —
  opens a popup with the preview URL and a scannable QR code (embedded
  `qrcode` lib) + a copy button, by @nordineonline-sudo (fork)
* deps(console): added `qrcode` (+ `@types/qrcode`), by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.8] — 2026-09-01

* chore(console): removed the floating "?" helper button ("Ask sandboxd /
  Platform-wide help", component `Helper` in `console/src/App.tsx`) that
  blocked part of the interface, by @nordineonline-sudo (fork)
* docs: added `chat-context.md` — hand-off guide for AI agents / humans to
  resume work on this fork (state, history, rules, versioning, commit &
  deployment runbooks), by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.7] — 2026-09-01

* feat(providers): **6 more model providers wired end-to-end** — Mistral,
  Vercel AI Gateway, Hugging Face, Z.AI (standard OpenAI-compatible bearer),
  and Google/Gemini (its own header `x-goog-api-key` + response shape,
  handled by the new `v1GoogleModels`) and Perplexity (wired for task
  execution; no public `/models` endpoint, so no discovery — type the model
  id manually). 12 providers now fully usable from the chat; 4 remain
  connectable-but-not-routed (Amazon Bedrock, Azure, GitHub Copilot,
  Cloudflare AI Gateway — genuinely different auth schemes: AWS SigV4, an
  Azure resource/deployment field, GitHub's OAuth device flow, or a
  per-account URL segment), by @nordineonline-sudo (fork)
* feat(console): removed the redundant "agent" selector (OpenCode vs Claude
  Code) — the chat always runs on `opencode`, the only agent wired to the
  gateway providers; the provider picker now lists every wired gateway
  (connected or not, with a "(not connected)" hint) instead of only the
  currently-connected ones; the last provider/model choice is now persisted
  per sandbox in `localStorage` (previously reset on every reopen)
* feat(console): the send button turns into a **Stop** button while a task
  is running (`POST /v1/sandboxes/{id}/tasks/{id}/cancel` — the endpoint
  already existed, just wasn't wired to the UI)
* feat(console): the "Advanced" tab is renamed **"OpenCode"** and enabled on
  mobile too — it's the only place with real interactive permission prompts /
  multiple-choice questions, since the headless chat's `opencode run` always
  passes `--dangerously-skip-permissions`
* docs: README section 8 rewritten with a per-version table, `docs/agent-auth.md`
  provider directory updated, by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.6] — 2026-09-01

* fix(runtimed): a gateway model routed through the OpenCode agent (e.g.
  `openrouter/deepseek/deepseek-v4-flash`) was silently rewritten to
  OpenCode Zen instead of reaching its own connected provider, because the
  bugfix in `.4`/`.5` lives in `runtimed` — which is compiled into
  `sandboxd-base` (the SANDBOX image), not `sandboxd-control-plane`. Rebuilding
  only the control plane never picks it up; `image/build.sh` (or a full
  `docker compose build`) must run, and existing sandbox CONTAINERS must be
  recreated (stop/start alone reuses the old container) — documented in
  `docs/agent-auth.md`. No code change beyond re-confirming the `.4` fix
  actually reaches deployed sandboxes, by @nordineonline-sudo (fork)
* feat(console): the chat's single model field is now **two pickers** —
  a **provider** select (which connected gateway to route through) and a
  **searchable model combobox** (native `<input list>` + `<datalist>`, since
  a catalog like OpenRouter's is 400+ entries) fed from that provider's own
  catalog only (lighter than fetching every connected gateway up front),
  by @nordineonline-sudo (fork)
* docs: README section 8 + release-history table, by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.5] — 2026-09-01

* feat(api): new **GET /v1/agents/{provider}/models** — best-effort, read-only
  model catalog for a connected "model gateway" provider. Calls the standard
  OpenAI-compatible `GET <base>/models` directly from the control plane (never
  through a sandbox), injecting the stored API key as a Bearer header;
  OpenCode Zen, OpenRouter and NVIDIA also answer without one. Returns
  `"<provider>/<model-id>"`, passable straight back as a task's `model`,
  by @nordineonline-sudo (fork)
* feat(console): the chat's model field becomes a live **dropdown** populated
  from every connected wired gateway (aggregated in parallel), with a "type
  id…" escape hatch back to free text; falls back automatically when the list
  is empty or the provider isn't wired yet, by @nordineonline-sudo (fork)
* docs: openapi.yaml, README section 8, `docs/agent-auth.md`,
  by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.4] — 2026-09-01

* feat(console): **sidebar navigation** — the top bar becomes a fixed left
  rail on desktop and a hamburger-opened full-height drawer on mobile,
  touch-sized (`Sidebar.tsx`), by @nordineonline-sudo (fork)
* feat(console): **headless agent chat**, Telegram/WhatsApp-style, on PC and
  mobile — chat bubbles, auto-expanding textarea, Enter-safe on mobile
  keyboards, round send button; drives the existing
  `/v1/sandboxes/{id}/tasks` + SSE API (no new backend surface). OpenCode's
  own native web session moves to a desktop-only **Advanced** tab,
  by @nordineonline-sudo (fork)
* feat(control-plane): generalized the MiniMax-style credential-only gateway
  (`authproxy.creditOnlyProviders`) to **6 more providers wired end-to-end** —
  OpenAI, DeepSeek, OpenRouter, Cerebras, NVIDIA, xAI — connectable in
  Settings → AI Agents and usable from the OpenCode agent as
  `<provider>/<model-id>`, by @nordineonline-sudo (fork)
* feat(agentauth): **10 further providers** connectable (key stored securely)
  — Google, Amazon Bedrock, Azure OpenAI, GitHub Copilot, Cloudflare/Vercel AI
  Gateway, Hugging Face, Z.AI, Perplexity, Mistral — not yet routed by the
  proxy (need a non-bearer auth scheme); see `docs/agent-auth.md`,
  by @nordineonline-sudo (fork)
* docs: README section 8, release-history table, `docs/agent-auth.md`
  provider directory, by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.3] — 2026-09-01

* feat(settings): custom global **Agent instructions** — saved from
  Settings → Agent instructions (custom), persisted per instance, appended
  after the embedded platform briefing (delimiter + rendered with the same
  per-sandbox placeholders) at every task submit, by @nordineonline-sudo (fork)
* feat(api): GET /v1/settings exposes `agents.custom_system_prompt`;
  PATCH /v1/settings accepts `agents.system_prompt` (8 KiB cap, set/clear,
  audited), applied to the NEXT tasks only, by @nordineonline-sudo (fork)
* feat(console): new editable card in Settings with textarea + Save +
  Revert to default, by @nordineonline-sudo (fork)

## [0.4.1-nordineonline.2] — 2026-09-01

* feat(api): the Files tab is now a real file manager — new host-side
  endpoints: single-file **download** (binary-safe, no size cap), **directory
  zip** archive, multi-file **upload** (multipart; part filenames may carry
  relative paths so dropped folders land intact; 25 MiB total cap),
  **mkdir**, **delete** (file or recursive) and in-place **rename**, by
  @nordineonline-sudo (fork)
* feat(console): Files tab rebuilt — per-node download/rename/delete actions,
  create file/folder, multi-file + drag & drop upload (files AND folders),
  inline image preview (png/jpg/gif/webp/svg) and a download CTA for
  binary/oversized files, by @nordineonline-sudo (fork)
* security: all new paths are lexically confined to the app dir AND checked
  component-by-component for symlinks (CWE-59); uploads are atomic, never
  overwrite a symlink and are chown'd to the workspace owner; mutations are
  audited, by @nordineonline-sudo (fork)
* fix(api): `/files/content` serves image extensions with their real media
  type plus a sandboxed CSP, so previews render without relaxing headers, by
  @nordineonline-sudo (fork)

## [0.4.1-nordineonline.1] — 2026-08-20

* feat(console): reorganize the app tabs — a dedicated **agent** tab (first,
  default) now hosts the embedded OpenCode web UI **full page width** and fills
  the exact remaining viewport height, replacing the chat panel on the Overview
  tab, by @nordineonline-sudo (fork)
* feat(console): new **README** tab (brain-style view/edit with a
  "Create README.md" empty state) for the app's root README.md — a normal
  workspace file committed to git, unlike the git-excluded brain, by @nordineonline-sudo (fork)
* feat(console): agent tab is full-bleed on mobile too, by @nordineonline-sudo (fork)
* chore(console): `Card`/`Btn` forward `data-testid` so E2E selectors are
  reliable, by @nordineonline-sudo (fork)

## [0.4.0-nordineonline.2] — 2026-08-20

* release(prod): production release of the fork — pinned image tags
  (`sandboxd-base`, `sandboxd-control-plane`, `sandboxd-console`) for the
  `nordineonline-sudo/sandboxd` deployment, by @nordineonline-sudo (fork)
* docs: rewrite the top-level README for this fork (repo links, install from
  this git, changelog) and document every evolution vs upstream — embedded
  OpenCode web console, cross-device sessions, mobile fixes — by @nordineonline-sudo (fork)
* fix(control-plane): hand `RUNTIMED_OPENCODE_WEB_PASSWORD` to sandboxes
  recreated on wake/self-heal (the rebuild used sandboxspec.Build, which never
  received the key) — without it a recreated container silently ran without the
  embedded `opencode web` and the console's OpenCode tab 502'd, by @nordineonline-sudo (fork)

## [0.4.0-nordineonline.1] — 2026-08-20

* feat(console): embed the native **OpenCode web** UI per app — the Overview tab
  now renders OpenCode's own session UI in an iframe on a dedicated
  `opencode-<id>.preview.<domain>` host (per-sandbox auth token minted by the
  control plane) instead of the old bespoke chat, by @nordineonline-sudo with Claude (Anthropic) (fork)
* feat(console): make the Overview/Files/Git layouts and top bar responsive on
  mobile by @nordineonline-sudo (fork)
* feat(control-plane): reverse-proxy `opencode-<id>.preview.<domain>` to each
  sandbox's internal `opencode web` — validates the per-sandbox password
  (auth_token query or Basic), passes static assets through, and streams SSE/pty
  — plus `GET /v1/sandboxes/{id}/opencode-url`, by @nordineonline-sudo (fork)
* feat(control-plane): seed the sandbox's app workspace into the OpenCode web
  client's per-browser project store (localStorage) on first load, so
  conversations created on one device are visible from any other — the project
  registry is otherwise browser-local and a fresh device shows "Nothing here
  yet"; injects the seed script into the SPA shell and allow-lists it in the
  page's Content-Security-Policy by sha256 hash, by @nordineonline-sudo (fork)

## [0.3.6] — 2026-08-01

* fix(console): stop double-prefixing write paths (console saves went to a phantom dir) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/99
* feat(detect): detect the project's package manager (yarn/npm/bun), not just pnpm by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/100


**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.5...v0.3.6

## [0.3.5] — 2026-07-30

* fix: self-healing start — recreate sandboxes whose container is stale or missing by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/97
* fix(authproxy): fail fast with the provider's real message instead of a timeout by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/98


**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.4...v0.3.5

## [0.3.4] — 2026-07-30

* feat(brain): spoke notes (brain/*.md) + shared-concept radar by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/96


**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.3...v0.3.4

## [0.3.3] — 2026-07-30

* feat(brain): [[wikilinks]] between brains + knowledge graph view by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/95


**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.2...v0.3.3

## [0.3.2] — 2026-07-30

* feat: Project Brain — persistent per-app memory (BRAIN.md) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/94


**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.1...v0.3.2

## [0.3.1] — 2026-07-30

* docs: remove dev-process/phase artifacts; make md match the code by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/76
* install: BSD-safe mktemp on macOS by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/74
* docs(git): document token rotation (delete + recreate) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/72
* Opt-in gVisor (runsc) isolation for sandboxes — verified end-to-end by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/69
* fix(console): terminal WebSocket died with 400 through the console nginx by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/77
* fix(console): terminal origin check failed on non-default ports ($host strips the port) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/78
* console: terminal connects on explicit click (not on tab open) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/79
* readme: CTAs for release news + Cloud waitlist by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/80
* console(demo): make visitors notice the live preview is a real running app by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/81
* readme: affiliate disclosure above the deploy links by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/83
* console(home): explainer + live overview stats on the Apps home by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/82
* readme: compact 2-column screenshot grid by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/85
* feat(agent-auth): add direct MiniMax credentials and upstreams by @octo-patch in https://github.com/tastyeffectco/sandboxd/pull/89
* upgrade: rebuild the sandbox base image when runtimed changes by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/90
* console: update-available notification (+ checker false-positive fix) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/91
* release tooling: ./release.sh (rolling patch releases + generated changelog) by @tastyeffectco in https://github.com/tastyeffectco/sandboxd/pull/92

### New Contributors
* @octo-patch made their first contribution in https://github.com/tastyeffectco/sandboxd/pull/89

**Full Changelog**: https://github.com/tastyeffectco/sandboxd/compare/v0.3.0...v0.3.1

## [0.3.0] — 2026-07-07

The major platform release: a **web console**, one-step **runtime presets**, live
preview URLs, agent tasks, app config &amp; secrets, snapshots / fork / restore,
and git import / commit / push — with one headline change: **every coding agent
now reaches its model provider through a credential-injecting proxy, so no API key
or OAuth token ever enters a sandbox.**

### Added
- **Credential-injecting auth proxy for all agents.** claude-code and opencode
  route through a control-plane proxy (`internal/authproxy`) that holds the real
  credential and injects it on the wire; the sandbox gets only a base URL + a
  dummy key, and nothing secret is mounted or env-injected into the workspace.
  `SANDBOXD_OPENCODE_ZEN_PATH` selects the OpenCode Zen endpoint (`zen`
  pay-as-you-go or `zengo` subscription).
- **OpenCode is the default agent, and `--continue` is the default** for
  follow-up tasks — tri-state (`continue` omitted → continue when a prior session
  exists, gated so the first task in a sandbox starts fresh; `true`/`false` force
  it).

### Platform
This release adds the full self-hosted platform: a **web console**;
one-step **runtime presets** (React/Vite, Next.js, Node/Express, FastAPI,
Worker); **live preview URLs**; **agent tasks**; **app config &amp; secrets**
(write-only secrets); **snapshots / fork / restore**; managed **agent auth**
(API-key / import / guided OAuth); **git import, commit &amp; push**; **runtime
detection &amp; manifest**; an **activity / events** timeline; **per-process
logs**; and a **settings** view with editable idle / keepalive lifecycle
controls.

## [0.2.0] — 2026-06-22

Reliability fixes across the core, and durable "apps" as first-class entities
above sandboxes.

### Added
- **Durable app model.** Apps are now first-class entities above sandboxes. An
  app owns the user-facing concept (name, description, tags) and outlives the
  sandbox that is its current running instance. New tenant-scoped `/v1/apps` API
  (`POST` / `GET` / `GET {id}` / `PATCH {id}` / `POST {id}/sandbox`) with optional
  `external_*` integration tags; sandboxes gain a nullable `app_id`. Additive and
  backwards-compatible — the existing sandbox API is unchanged. (#31)
- **Selectable app templates.** A working Vite + React + TypeScript
  `react-standard` scaffold ships in the image at `/opt/templates/<name>` and is
  seeded into a new workspace on first boot (default `react-standard`;
  `template: "blank"` for an empty workspace). The agent now edits a known-good
  app with a passing build and a live preview instead of scaffolding from an
  empty directory. (#29)
- **Per-task timeout.** `timeout_s` on `POST /v1/sandboxes/{id}/tasks` (0 or
  omitted → 10m default, max 24h). The control-plane task watcher now derives its
  streaming window from the task timeout instead of a fixed 15 minutes, so long
  tasks are no longer marked failed prematurely. (#25)
- **Per-sandbox idle policy.** `idle_policy: sleep | always_on`. (#14)
- **End-to-end + image-smoke CI.** A job that builds the base image and drives
  the real create → seed → install → serve → wake lifecycle on a Docker daemon,
  and asserts the agent CLIs and the default template are present on the image.
  Adds `go vet` to the Go job. (#30)

### Fixed
- **Snapshot capture** targeted the old loopback `.img` model and returned 500 on
  the default directory-storage workspaces; it now copies the workspace tree
  crash-consistently and round-trips through `from_snapshot`. (#24)
- **`POST /v1/sandboxes`** returned `400` on a clean install because it forced an
  unseeded `react-standard` template; a no-template create is now provisioned
  cleanly. (#28)
- Four confirmed correctness items from the security/code audit. (#21)

### Changed
- The image installs Claude Code via the official native installer, alongside
  OpenCode. (#18)

### Removed
- The dormant single-token auto-git-push path (undocumented, unused). (#23)

## [0.1.1] — 2026-06-07

- Renamed the project to **sandboxd** and standardized the `SANDBOXD_` env prefix.
- Docs: production-safety checklist; Rancher Desktop / k3s port-80 preview note.

## [0.1.0] — 2026-06-06

- Initial release.

[0.3.0]: https://github.com/tastyeffectco/sandboxd/releases/tag/v0.3.0
[0.2.0]: https://github.com/tastyeffectco/sandboxd/releases/tag/v0.2.0
[0.1.1]: https://github.com/tastyeffectco/sandboxd/releases/tag/v0.1.1
[0.1.0]: https://github.com/tastyeffectco/sandboxd/releases/tag/v0.1.0
