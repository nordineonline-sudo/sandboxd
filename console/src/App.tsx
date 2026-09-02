import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, setOnUnauthorized, App as TApp, Preset, GitCredential, Agent } from './api'
import { c, font, mono, Card, Btn, StatusPill, Input, useIsMobile } from './design/kit'
import { Sidebar } from './Sidebar'
import { PRESET_ICONS } from './design/presetIcons'
import { STARTERS, STARTER_ICONS } from './design/starters'
import { AppView } from './AppView'
import { SettingsView } from './SettingsView'
import { Login, CreatePassword } from './AuthGate'

type Route = { name: 'apps' } | { name: 'settings' } | { name: 'app'; id: string; tab?: string; task?: string }

export default function App() {
  const isMobile = useIsMobile()
  const [route, setRoute] = useState<Route>({ name: 'apps' })
  const [toasts, setToasts] = useState<{ id: number; msg: string }[]>([])
  const [apps, setApps] = useState<TApp[]>([])
  const [sbInfo, setSbInfo] = useState<Record<string, { status: string; url?: string }>>({})
  const [auth, setAuth] = useState<{ enabled: boolean; authenticated: boolean; password_set: boolean } | null>(null)
  // Update notification: the control plane checks GitHub releases (cached ~6h)
  // and reports update_available in /v1/settings. Dismissal is remembered per
  // version, so each new release notifies exactly once.
  const [upd, setUpd] = useState<{ latest: string; url?: string } | null>(null)

  const toast = useCallback((msg: string) => {
    const id = Date.now() + Math.floor(performance.now())
    setToasts((t) => [...t, { id, msg }])
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3200)
  }, [])
  const onError = useCallback((m: string) => toast(m), [toast])

  const loadApps = useCallback(() => api.listApps().then(setApps).catch(() => {}), [])
  const refreshAuth = useCallback(
    () => api.authStatus().then(setAuth).catch(() => setAuth({ enabled: true, authenticated: false, password_set: false })),
    [],
  )

  // On mount: resolve auth state, and register the 401 hook so an expired session
  // bounces back to the gate. Only load apps once we know we're allowed through.
  useEffect(() => {
    refreshAuth()
    setOnUnauthorized(() => setAuth((a) => (a ? { ...a, authenticated: false } : a)))
  }, [refreshAuth])
  useEffect(() => {
    if (auth && (auth.authenticated || auth.enabled === false)) loadApps()
  }, [auth, loadApps])
  useEffect(() => {
    if (!(auth && (auth.authenticated || auth.enabled === false))) return
    api.getSettings().then((s) => {
      if (s.update_available && s.latest_version && localStorage.getItem('sandboxd-update-dismissed') !== s.latest_version) {
        setUpd({ latest: s.latest_version, url: s.changelog_url })
      }
    }).catch(() => {})
  }, [auth])

  const onAuthed = useCallback(() => { refreshAuth().then(loadApps) }, [refreshAuth, loadApps])
  const logout = useCallback(() => { api.logout().finally(() => setAuth((a) => (a ? { ...a, authenticated: false } : a))) }, [])

  // Sandbox status/url per app, so the sidebar can show each app's live state
  // and the Start/Stop/Open actions. Refetched whenever the app list changes
  // and on a light interval while anything is running.
  const loadSbInfo = useCallback(() => {
    Promise.all(apps.filter((a) => a.current_sandbox_id).map(async (a) => {
      try { const s = await api.getSandbox(a.current_sandbox_id as string); return [a.id, { status: s.status, url: s.preview?.url }] as const }
      catch { return [a.id, { status: 'unknown' }] as const }
    })).then((p) => setSbInfo(Object.fromEntries(p))).catch(() => {})
  }, [apps])
  useEffect(() => { loadSbInfo() }, [loadSbInfo])
  useEffect(() => {
    const t = setInterval(() => { if (apps.some((a) => sbInfo[a.id]?.status === 'running')) loadSbInfo() }, 5000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apps, loadSbInfo, sbInfo])

  const goApp = (id: string, tab?: string) => setRoute({ name: 'app', id, tab })
  const running = apps.find((a) => a.current_sandbox_id)

  // Start or stop an app's sandbox from the sidebar submenu.
  const toggleSandbox = useCallback(async (appId: string) => {
    const sbId = apps.find((a) => a.id === appId)?.current_sandbox_id
    if (!sbId) { onError('No sandbox for this app yet — open it and create one'); return }
    try {
      const cur = await api.getSandbox(sbId)
      if (cur.status === 'running') await api.stopSandbox(sbId)
      else await api.startSandbox(sbId)
      loadSbInfo()
    } catch (e) { onError((e as Error).message) }
  }, [apps, loadSbInfo, onError])

  const nav = [
    { key: 'apps', label: 'Apps' },
    { key: 'settings', label: 'Settings' },
  ]

  // Auth gate — render before the app chrome. null = still resolving status.
  if (auth === null) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', background: c.bg, color: c.muted2, fontFamily: font.sans }}>
        Loading…
      </div>
    )
  }
  if (auth.enabled && !auth.authenticated) {
    return auth.password_set ? <Login onDone={onAuthed} /> : <CreatePassword onDone={onAuthed} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', background: c.bg, color: c.fg, fontFamily: font.sans, overflow: 'hidden' }}>
      {/* UPDATE NOTIFICATION — one slim, dismissible strip per new release. */}
      {upd && (
        <div data-testid="update-banner" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, flexShrink: 0, padding: '6px 40px 6px 16px', fontSize: 12.5, background: c.panel2, borderBottom: `1px solid ${c.border}`, position: 'relative' }}>
          <span style={{ width: 7, height: 7, borderRadius: '50%', background: c.good, flexShrink: 0 }} />
          <span>
            <b>Update available: {upd.latest}</b> — run <span style={{ ...mono, fontSize: 11.5, background: c.bg, border: `1px solid ${c.border}`, borderRadius: 4, padding: '1px 6px' }}>./upgrade.sh</span> on your server.
          </span>
          {upd.url && <a href={upd.url} target="_blank" rel="noreferrer" style={{ color: c.link, textDecoration: 'none' }}>Release notes ↗</a>}
          <span
            data-testid="update-dismiss"
            className="dc-hoverink"
            onClick={() => { localStorage.setItem('sandboxd-update-dismissed', upd.latest); setUpd(null) }}
            style={{ position: 'absolute', right: 14, color: c.muted, cursor: 'pointer', fontSize: 13 }}>
            ✕
          </span>
        </div>
      )}
      {/* SIDEBAR (desktop: fixed rail) + MAIN (mobile: Sidebar renders its own top bar with hamburger) */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {!isMobile && (
          <Sidebar
            nav={nav}
            active={route.name === 'app' ? 'app' : route.name}
            onNavigate={(key) => setRoute({ name: key } as Route)}
            apps={apps}
            sbInfo={sbInfo}
            currentApp={route.name === 'app' ? { id: route.id, tab: route.tab } : undefined}
            onOpenApp={(id, tab) => goApp(id, tab)}
            onToggleSandbox={toggleSandbox}
            running={running ? { id: running.id, name: running.name } : null}
            onOpenRunning={() => goApp(running!.id)}
            logout={logout}
            authEnabled={!!auth?.enabled}
          />
        )}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, overflow: 'hidden' }}>
          {isMobile && (
            <Sidebar
              nav={nav}
              active={route.name === 'app' ? 'app' : route.name}
              onNavigate={(key) => setRoute({ name: key } as Route)}
              apps={apps}
              sbInfo={sbInfo}
              currentApp={route.name === 'app' ? { id: route.id, tab: route.tab } : undefined}
              onOpenApp={(id, tab) => goApp(id, tab)}
              onToggleSandbox={toggleSandbox}
              running={running ? { id: running.id, name: running.name } : null}
              onOpenRunning={() => goApp(running!.id)}
              logout={logout}
              authEnabled={!!auth?.enabled}
            />
          )}

          {/* MAIN */}
          <div style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden' }}>
            {route.name === 'apps' && <AppsScreen apps={apps} reload={loadApps} sbInfo={sbInfo} onOpen={(id) => goApp(id)} onError={onError} />}
            {route.name === 'settings' && <SettingsView onError={onError} toast={toast} />}
            {route.name === 'app' && (
              <AppView
                appId={route.id}
                tab={route.tab}
                onTabChange={(t) => setRoute({ name: 'app', id: route.id, tab: t })}
                onError={onError}
                toast={toast}
                goApps={() => { setRoute({ name: 'apps' }); loadApps() }}
                goSettings={() => setRoute({ name: 'settings' })}
                apps={apps}
                onOpenApp={(id, tab) => goApp(id, tab)}
              />
            )}
          </div>
        </div>
      </div>

      {/* TOASTS */}
      {toasts.length > 0 && (
        <div style={{ position: 'fixed', bottom: 80, right: 20, zIndex: 99, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {toasts.map((t) => (
            <div key={t.id} style={{ background: c.ink, color: '#fff', borderRadius: 8, padding: '10px 16px', fontSize: 12.5, boxShadow: '0 8px 24px rgba(0,0,0,.18)', display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ color: '#4ade80' }}>✓</span>{t.msg}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// Short label + tagline per preset (cleaner than the API's full sentences).
const PRESET_META: Record<string, { short: string; tag: string }> = {
  'react-vite': { short: 'React', tag: 'Vite SPA · hot reload' },
  nextjs: { short: 'Next.js', tag: 'App Router · SSR' },
  'node-express': { short: 'Express', tag: 'Node REST API' },
  fastapi: { short: 'FastAPI', tag: 'Python REST API' },
  worker: { short: 'Worker', tag: 'Background · no preview' },
}

function AppsScreen({ apps, reload, sbInfo, onOpen, onError }: { apps: TApp[]; reload: () => void; sbInfo: Record<string, { status: string; url?: string }>; onOpen: (id: string) => void; onError: (m: string) => void }) {
  const [name, setName] = useState('')
  const [starter, setStarter] = useState('')
  const [preset, setPreset] = useState('')
  const [presets, setPresets] = useState<Preset[]>([])
  const [repo, setRepo] = useState('')
  const [branch, setBranch] = useState('main')
  const [credId, setCredId] = useState('')
  const [creds, setCreds] = useState<GitCredential[]>([])
  const [busy, setBusy] = useState(false)
  const [agents, setAgents] = useState<Agent[]>([])
  // Progressive disclosure: the simple path (name → create) shows first; the
  // power options (stack / git / starter) reveal only when asked for.
  const [showStack, setShowStack] = useState(false)
  const [showGit, setShowGit] = useState(false)
  const [showStarter, setShowStarter] = useState(false)
  const [creating, setCreating] = useState(false) // returning users open the form via "+ New app"

  useEffect(() => {
    api.listPresets().then(setPresets).catch(() => {})
    api.listGitCredentials().then(setCreds).catch(() => {})
    api.getAgents().then(setAgents).catch(() => {})
  }, [])

  const create = async () => {
    const useGit = showGit && !!repo.trim()
    const pick = starter ? STARTERS.find((s) => s.id === starter) : undefined
    // No forced naming: derive a sensible default (from the repo/starter, else a
    // short slug) when the field is blank. Renameable anytime from the app header.
    const slug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '').slice(0, 40)
    const derived = useGit ? slug(repo.trim().replace(/\.git$/, '').split('/').pop() || '')
      : pick ? slug(pick.id) : ''
    const finalName = name.trim() || derived || `app-${Math.random().toString(36).slice(2, 6)}`
    setBusy(true)
    try {
      const a = await api.createApp({
        name: finalName,
        runtime_preset: !useGit && !pick && preset ? preset : undefined, // no stack chosen → auto-detected
        git: useGit ? { repo_url: repo.trim(), branch: branch.trim() || 'main', ...(credId ? { credential_id: credId } : {}) } // no credId → public tokenless clone
          : pick ? { repo_url: `https://github.com/${pick.repo}`, branch: pick.branch } // public → tokenless
          : undefined,
      })
      // Zero-friction: boot the sandbox right away so the app is ready to use.
      // If it fails, the app view still offers a Create-sandbox retry.
      try { await api.createAppSandbox(a.id, {}) } catch { /* app exists; retry from the app view */ }
      setName(''); setRepo(''); setCreating(false); reload(); onOpen(a.id)
    } catch (e) { onError((e as Error).message) } finally { setBusy(false) }
  }

  const secBtn = (label: string, active: boolean, onClick: () => void) => (
    <div onClick={onClick} className="dc-hoverborder" style={{ padding: '7px 13px', fontSize: 12.5, borderRadius: 7, cursor: 'pointer', border: `1px solid ${active ? c.faint : c.border}`, color: active ? c.fg : c.muted, background: active ? c.panel2 : 'transparent' }}>{label}</div>
  )

  // Live overview — all real (no fabricated time-series/graphs). "asleep" = apps
  // not currently serving (stopped/sleeping/no sandbox yet); they wake on request.
  const runningApps = apps.filter((a) => sbInfo[a.id]?.status === 'running')
  const otherApps = apps.filter((a) => sbInfo[a.id]?.status !== 'running')
  const live = runningApps.length
  const connected = agents.filter((a) => a.status === 'connected').length
  const tiles: { n: string | number; label: string; tone: string; dot?: string }[] = [
    { n: apps.length, label: 'apps', tone: c.fg },
    { n: live, label: 'live', tone: c.good, dot: live ? c.good : undefined },
    { n: Math.max(0, apps.length - live), label: 'asleep', tone: c.muted },
    { n: `${connected}/${agents.length}`, label: 'agents', tone: connected ? c.fg : c.warn },
  ]
  const showForm = creating || apps.length === 0

  return (
    <div className="dc-page" style={{ maxWidth: 920, margin: '0 auto', padding: '36px 40px 80px' }}>
      <h1 style={{ fontFamily: font.display, fontSize: 24, fontWeight: 700, margin: '0 0 6px' }}>Apps</h1>
      <p style={{ color: c.muted, margin: '0 0 18px', maxWidth: 580 }}>Each app runs isolated in its own sandbox with a live preview URL — an AI agent builds it, you own it. Idle apps sleep and wake on request.</p>

      {/* At-a-glance overview: real, live counts pulled from the API. */}
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 26 }} data-testid="overview-stats">
        {tiles.map((t) => (
          <Card key={t.label} style={{ padding: '12px 16px', minWidth: 92, display: 'flex', flexDirection: 'column', gap: 3 }}>
            <span style={{ fontFamily: font.display, fontSize: 26, fontWeight: 700, lineHeight: 1, color: t.tone }}>{t.n}</span>
            <span style={{ ...mono, fontSize: 11, letterSpacing: '.04em', color: c.muted, textTransform: 'uppercase', display: 'flex', alignItems: 'center', gap: 5 }}>
              {t.dot && <span style={{ width: 6, height: 6, borderRadius: '50%', background: t.dot }} />}
              {t.label}
            </span>
          </Card>
        ))}
      </div>

      {/* RUNNING NOW — the live app is the hero, not a tiny pill in the top bar. */}
      {runningApps.length > 0 && (
        <div style={{ marginBottom: 28 }} data-testid="running-now">
          <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginBottom: 10 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: c.good }} />
            <span style={{ fontFamily: font.display, fontWeight: 700, fontSize: 14 }}>Running now</span>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(240px,1fr))', gap: 12 }}>
            {runningApps.map((a) => {
              const url = sbInfo[a.id]?.url
              return (
                <Card key={a.id} style={{ padding: 14, borderColor: c.good }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                    <span style={{ width: 7, height: 7, borderRadius: '50%', background: c.good, flexShrink: 0 }} />
                    <span style={{ ...mono, fontWeight: 500, fontSize: 14, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{a.name}</span>
                    <span style={{ ...mono, fontSize: 10, color: c.good, letterSpacing: '.05em' }}>LIVE</span>
                  </div>
                  {url && <div style={{ ...mono, fontSize: 11, color: c.muted2, marginBottom: 10, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{url.replace(/^https?:\/\//, '')}</div>}
                  <div style={{ display: 'flex', gap: 8 }}>
                    <Btn variant="primary" onClick={() => url ? window.open(url, '_blank') : onOpen(a.id)} style={{ padding: '7px 14px', fontSize: 12.5 }}>Open ↗</Btn>
                    <Btn onClick={() => onOpen(a.id)} style={{ padding: '7px 12px', fontSize: 12.5 }}>Manage</Btn>
                  </div>
                </Card>
              )
            })}
          </div>
        </div>
      )}

      {/* CREATE — simple by default (name → create, stack auto-detected); the
          power paths (stack / git / starter) reveal only on demand. */}
      {showForm && (
        <Card style={{ padding: 16, marginBottom: 28 }}>
          {apps.length === 0 && <div style={{ fontFamily: font.display, fontWeight: 700, fontSize: 16, marginBottom: 10 }}>Build an app you own</div>}
          <div style={{ display: 'flex', gap: 10, marginBottom: 12 }}>
            <Input mono value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && create()} placeholder="Describe or name your app…" style={{ flex: 1, fontSize: 13 }} data-testid="app-name" />
            <Btn variant="primary" disabled={busy} onClick={create} style={{ padding: '9px 18px', fontSize: 13 }}>Create app</Btn>
          </div>

          {/* optional stack picker — hidden by default */}
          <div onClick={() => setShowStack((v) => !v)} className="dc-hoverink" data-testid="stack-toggle" style={{ ...mono, fontSize: 12, color: c.muted, cursor: 'pointer', userSelect: 'none' }}>
            {showStack ? '▾' : '▸'} Choose a stack <span style={{ color: c.muted2 }}>(optional — auto-detected by default)</span>
          </div>
          {showStack && (
            <div data-testid="app-preset" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(158px,1fr))', gap: 10, marginTop: 12 }}>
              {presets.map((p) => {
                const meta = PRESET_META[p.id] || { short: p.label, tag: p.description }
                const active = preset === p.id
                return (
                  <div key={p.id} data-testid={`preset-${p.id}`} onClick={() => setPreset(active ? '' : p.id)} className="dc-hoverborder"
                    style={{ position: 'relative', display: 'flex', flexDirection: 'column', gap: 8, padding: '13px 13px 12px', borderRadius: 10, cursor: 'pointer', border: `1px solid ${active ? c.ink : c.border}`, background: active ? c.panel2 : c.panel, boxShadow: active ? `inset 0 0 0 1px ${c.ink}` : 'none' }}>
                    {active && <span style={{ position: 'absolute', top: 9, right: 9, width: 16, height: 16, borderRadius: '50%', background: c.ink, color: '#fff', fontSize: 10, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>✓</span>}
                    <span className="prov-ico" style={{ width: 26, height: 26 }} dangerouslySetInnerHTML={{ __html: PRESET_ICONS[p.id] || '' }} />
                    <div>
                      <div style={{ fontFamily: font.display, fontWeight: 600, fontSize: 13.5 }}>{meta.short}</div>
                      <div style={{ color: c.muted2, fontSize: 11.5, lineHeight: 1.35, marginTop: 1 }}>{meta.tag}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}

          {/* secondary paths — off to the side, not shoved up front */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginTop: 16, paddingTop: 14, borderTop: `1px solid ${c.border}` }}>
            <span style={{ color: c.muted2, fontSize: 12, marginRight: 2 }}>or</span>
            {secBtn('Import from Git', showGit, () => { setShowGit((v) => !v); setShowStarter(false) })}
            {secBtn('Start from a starter', showStarter, () => { setShowStarter((v) => !v); setShowGit(false) })}
          </div>

          {showGit && (
            <>
              <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
                <Input mono value={repo} onChange={(e) => setRepo(e.target.value)} placeholder="https://github.com/user/repo.git" style={{ flex: 1, minWidth: 220, fontSize: 12.5 }} data-testid="git-repo-url" />
                <Input mono value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="branch" style={{ width: 120, fontSize: 12.5 }} data-testid="git-branch" />
                <select value={credId} onChange={(e) => setCredId(e.target.value)} data-testid="git-cred" style={{ background: c.bg, border: `1px solid ${c.border2}`, borderRadius: 7, padding: '8px 10px', color: c.fg, fontSize: 12.5, fontFamily: font.sans }}>
                  <option value="">Public repo — no credential</option>
                  {creds.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
                </select>
              </div>
              {creds.length === 0 && (
                <div style={{ marginTop: 8, fontSize: 12, color: c.muted }} data-testid="git-no-creds"><b>Public repos import with no credential.</b> For a <b>private</b> repo, add a personal access token in <b>Settings → Git credentials</b>.</div>
              )}
            </>
          )}

          {showStarter && (
            <div data-testid="starter-grid" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(212px,1fr))', gap: 10, marginTop: 12 }}>
              {STARTERS.map((s) => {
                const active = starter === s.id
                return (
                  <div key={s.id} data-testid={`starter-${s.id}`} onClick={() => setStarter(active ? '' : s.id)} className="dc-hoverborder"
                    style={{ position: 'relative', display: 'flex', gap: 10, padding: '12px 13px', borderRadius: 10, cursor: 'pointer', border: `1px solid ${active ? c.ink : c.border}`, background: active ? c.panel2 : c.panel, boxShadow: active ? `inset 0 0 0 1px ${c.ink}` : 'none' }}>
                    {active && <span style={{ position: 'absolute', top: 8, right: 8, width: 15, height: 15, borderRadius: '50%', background: c.ink, color: '#fff', fontSize: 9, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>✓</span>}
                    <span className="prov-ico" style={{ width: 24, height: 24, flexShrink: 0, marginTop: 1 }} dangerouslySetInnerHTML={{ __html: STARTER_ICONS[s.tech] || '' }} />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontFamily: font.display, fontWeight: 600, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
                        {s.stars && <span style={{ ...mono, fontSize: 10, color: c.muted2, flexShrink: 0 }}>★{s.stars}</span>}
                      </div>
                      <div style={{ color: c.muted2, fontSize: 11, lineHeight: 1.35, marginTop: 2 }}>{s.blurb}</div>
                      <div style={{ ...mono, fontSize: 10, color: c.faint, marginTop: 3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.repo}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </Card>
      )}

      {/* YOUR APPS — the asleep/idle ones; running apps live in "Running now". */}
      {apps.length > 0 && (
        <div>
          <div style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
            <span style={{ fontFamily: font.display, fontWeight: 700, fontSize: 14 }}>Your apps</span>
            <div style={{ flex: 1 }} />
            <Btn variant="primary" onClick={() => setCreating((v) => !v)} style={{ padding: '7px 14px', fontSize: 12.5 }} data-testid="new-app-toggle">{creating ? 'Close' : '+ New app'}</Btn>
          </div>
          {otherApps.length === 0 ? (
            <p style={{ color: c.muted2, fontSize: 13 }}>Every app is running — see “Running now” above.</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(270px,1fr))', gap: 14 }} data-testid="app-list">
              {otherApps.map((a) => (
                <Card key={a.id} style={{ padding: 16, cursor: 'pointer' }}>
                  <div className="dc-hoverborder" onClick={() => onOpen(a.id)} data-testid="app-card">
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                      <span style={{ ...mono, fontWeight: 500, fontSize: 14 }}>{a.name}</span>
                      <StatusPill status={a.current_sandbox_id ? sbInfo[a.id]?.status : undefined} />
                    </div>
                    <div style={{ color: c.muted, fontSize: 12.5, marginBottom: 12 }}>{a.description || a.id}</div>
                    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                      {(a.tags || []).slice(0, 3).map((t) => (
                        <span key={t} style={{ ...mono, fontSize: 10.5, color: c.muted, background: c.panel2, border: `1px solid ${c.border}`, borderRadius: 5, padding: '2px 7px' }}>{t}</span>
                      ))}
                      <span style={{ marginLeft: 'auto', color: c.link, fontSize: 12 }}>Open →</span>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

