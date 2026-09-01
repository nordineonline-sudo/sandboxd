import { useState } from 'react'
import { c, font, useIsMobile } from './design/kit'

export type NavItem = { key: string; label: string }

// Left-hand navigation, replacing the old horizontal top bar. Desktop: a
// fixed narrow rail (labels always visible — no hamburger needed, there's
// room). Mobile: a hamburger button opens a full-height drawer over the
// content; picking an item or tapping the scrim closes it.
export function Sidebar({
  nav, active, onNavigate, running, onOpenRunning, logout, authEnabled,
}: {
  nav: NavItem[]
  active: string
  onNavigate: (key: string) => void
  running?: { id: string; name: string } | null
  onOpenRunning: () => void
  logout: () => void
  authEnabled: boolean
}) {
  const isMobile = useIsMobile()
  const [open, setOpen] = useState(false)

  const brand = (
    <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
      <div style={{ width: 26, height: 26, borderRadius: 7, background: 'linear-gradient(135deg,#3f3f46,#18181b)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: font.mono, fontSize: 11, color: c.bg, flexShrink: 0 }}>&gt;_</div>
      <span style={{ fontFamily: font.display, fontWeight: 700, fontSize: 15, letterSpacing: '.2px', whiteSpace: 'nowrap' }}>sandboxd <span style={{ fontWeight: 500, color: c.muted }}>console</span></span>
    </div>
  )

  const navList = (closeOnClick: boolean) => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '10px 10px' }}>
      {nav.map((n) => (
        <div
          key={n.key}
          data-testid={`nav-${n.key}`}
          className="dc-hoverink"
          onClick={() => { onNavigate(n.key); if (closeOnClick) setOpen(false) }}
          style={{
            padding: isMobile ? '13px 14px' : '8px 12px', fontSize: isMobile ? 15 : 13, borderRadius: 8, cursor: 'pointer',
            color: active === n.key ? c.fg : c.muted, background: active === n.key ? c.panel2 : 'transparent', fontWeight: active === n.key ? 600 : 500,
          }}
        >
          {n.label}
        </div>
      ))}
    </div>
  )

  const footerLinks = (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '10px 10px', borderTop: `1px solid ${c.border}` }}>
      <a href="https://sandboxd.io" target="_blank" rel="noreferrer" className="dc-hoverink" style={{ color: c.muted, textDecoration: 'none', fontSize: isMobile ? 14 : 12, padding: isMobile ? '9px 14px' : '6px 12px' }}>Docs</a>
      <a href="https://github.com/tastyeffectco/sandboxd" target="_blank" rel="noreferrer" className="dc-hoverink" style={{ color: c.muted, textDecoration: 'none', fontSize: isMobile ? 14 : 12, padding: isMobile ? '9px 14px' : '6px 12px' }}>GitHub</a>
      <a href="https://github.com/tastyeffectco/sandboxd/discussions" target="_blank" rel="noreferrer" className="dc-hoverink" data-testid="nav-feedback" style={{ color: c.muted, textDecoration: 'none', fontSize: isMobile ? 14 : 12, padding: isMobile ? '9px 14px' : '6px 12px' }}>Feedback</a>
      {authEnabled && (
        <span data-testid="nav-logout" className="dc-hoverink" onClick={logout} style={{ color: c.muted, fontSize: isMobile ? 14 : 12, cursor: 'pointer', padding: isMobile ? '9px 14px' : '6px 12px' }}>Log out</span>
      )}
    </div>
  )

  if (isMobile) {
    return (
      <>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, height: 52, flexShrink: 0, padding: '0 12px', borderBottom: `1px solid ${c.border}`, background: c.panel }}>
          <button
            onClick={() => setOpen(true)}
            aria-label="Open menu"
            data-testid="mobile-menu-open"
            className="dc-hoverborder"
            style={{ width: 38, height: 38, border: `1px solid ${c.border}`, background: c.bg, borderRadius: 8, cursor: 'pointer', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 3, flexShrink: 0 }}
          >
            <span style={{ width: 16, height: 2, background: c.fg, borderRadius: 1 }} />
            <span style={{ width: 16, height: 2, background: c.fg, borderRadius: 1 }} />
            <span style={{ width: 16, height: 2, background: c.fg, borderRadius: 1 }} />
          </button>
          <div onClick={() => onNavigate('apps')} style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', flex: 1, minWidth: 0 }}>
            <div style={{ width: 24, height: 24, borderRadius: 6, background: 'linear-gradient(135deg,#3f3f46,#18181b)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: font.mono, fontSize: 10, color: c.bg, flexShrink: 0 }}>&gt;_</div>
            {running && (
              <span onClick={(e) => { e.stopPropagation(); onOpenRunning() }} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: c.muted, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: c.good, flexShrink: 0 }} />{running.name}
              </span>
            )}
          </div>
        </div>
        {open && (
          <div onClick={() => setOpen(false)} style={{ position: 'fixed', inset: 0, background: 'rgba(9,9,11,.4)', zIndex: 95 }}>
            <div onClick={(e) => e.stopPropagation()} style={{ position: 'absolute', top: 0, left: 0, bottom: 0, width: '82%', maxWidth: 320, background: c.panel, boxShadow: '4px 0 24px rgba(0,0,0,.18)', display: 'flex', flexDirection: 'column' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 14px', borderBottom: `1px solid ${c.border}` }}>
                {brand}
                <button onClick={() => setOpen(false)} aria-label="Close menu" data-testid="mobile-menu-close" style={{ background: 'none', border: 'none', fontSize: 20, color: c.muted2, cursor: 'pointer', padding: 4 }}>×</button>
              </div>
              <div style={{ flex: 1, overflowY: 'auto' }}>{navList(true)}</div>
              {footerLinks}
            </div>
          </div>
        )}
      </>
    )
  }

  // Desktop: fixed narrow rail on the left.
  return (
    <div style={{ display: 'flex', flexDirection: 'column', width: 208, flexShrink: 0, height: '100%', borderRight: `1px solid ${c.border}`, background: c.panel }}>
      <div onClick={() => onNavigate('apps')} style={{ padding: '14px 14px', cursor: 'pointer', borderBottom: `1px solid ${c.border}` }}>{brand}</div>
      <div style={{ flex: 1, overflowY: 'auto' }}>{navList(false)}</div>
      {running && (
        <div onClick={onOpenRunning} className="dc-hoverborder" style={{ display: 'flex', alignItems: 'center', gap: 7, margin: '0 10px 10px', border: `1px solid ${c.border}`, background: c.bg, borderRadius: 7, padding: '7px 10px', cursor: 'pointer' }}>
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: c.good, flexShrink: 0 }} />
          <span style={{ fontFamily: font.mono, fontSize: 11.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{running.name}</span>
        </div>
      )}
      {footerLinks}
    </div>
  )
}
