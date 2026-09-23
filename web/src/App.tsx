import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { Link, Navigate, NavLink, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import {
  BookOpen,
  BookMarked,
  Boxes,
  FlaskConical,
  Keyboard,
  KeyRound,
  Library,
  Menu,
  Network,
  PenLine,
  Search,
  Sparkles,
  type LucideIcon,
} from 'lucide-react'
import { api } from './api'
import type { Me } from './types'
import HelpDialog from './components/HelpDialog'
import JobDock from './components/JobDock'
import CommandPalette from './components/CommandPalette'
import Login from './pages/Login'
import Projects from './pages/Projects'
import ProjectHome from './pages/ProjectHome'
import Cards from './pages/Cards'
import Lab from './pages/Lab'
import Fuse from './pages/Fuse'
import Audit from './pages/Audit'
import Sample from './pages/Sample'
import Settings from './pages/Settings'
import Write from './pages/Write'
import ChapterPage from './pages/Chapter'
import Bible from './pages/Bible'
import Graph from './pages/Graph'
import NotFound from './pages/NotFound'
import { projectName } from './projectCache'

function isInputFocused() {
  const el = document.activeElement
  return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement
}

type AuthContextValue = {
  me: Me | null
  setMe: (user: Me | null) => void
}

const AuthContext = createContext<AuthContextValue>({
  me: null,
  setMe: () => undefined,
})

export function useAuth() {
  return useContext(AuthContext)
}

function SideLink({
  to,
  icon,
  end,
  children,
}: {
  to: string
  icon: LucideIcon
  end?: boolean
  children: string
}) {
  const Icon = icon
  return (
    <NavLink to={to} end={end} className={({ isActive }) => `side-link${isActive ? ' active' : ''}`}>
      <span className="side-ico">
        <Icon size={18} aria-hidden="true" />
      </span>
      <span>{children}</span>
    </NavLink>
  )
}

function Sidebar({
  me,
  onLogout,
  open,
  onOpenCmd,
}: {
  me: Me
  onLogout: () => void
  open: boolean
  onOpenCmd: () => void
}) {
  const location = useLocation()
  const id = location.pathname.match(/^\/p\/([^/]+)/)?.[1]
  const curProjectTitle = id ? projectName(id) : ''
  const avatarInitial = (me.email?.[0] || 'U').toUpperCase()

  return (
    <aside className={open ? 'sidebar open' : 'sidebar'}>
      <Link className="brand" to="/">
        <span className="brand-mark">风格工坊</span>
        <span className="brand-sub">STYLE LAB · STUDIO</span>
      </Link>

      <button
        type="button"
        className="btn secondary sm"
        style={{
          width: '100%',
          justifyContent: 'space-between',
          marginBottom: '0.8rem',
          fontSize: '0.8rem',
          padding: '0 0.75rem',
        }}
        onClick={onOpenCmd}
        title="按 Ctrl+K 搜索"
      >
        <span style={{ display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
          <Search size={14} color="var(--gold)" />
          <span>快速搜索…</span>
        </span>
        <span
          style={{
            fontSize: '0.68rem',
            padding: '0.1rem 0.35rem',
            borderRadius: '4px',
            background: 'rgba(255,255,255,0.08)',
            color: 'var(--ink-faint)',
            fontFamily: 'var(--mono)',
          }}
        >
          ⌘K
        </span>
      </button>

      {id && curProjectTitle ? (
        <div
          style={{
            padding: '0.45rem 0.8rem',
            marginBottom: '0.5rem',
            background: 'rgba(247,203,104,0.06)',
            border: '1px solid rgba(247,203,104,0.18)',
            borderRadius: 'var(--radius)',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}
        >
          <Sparkles size={14} color="var(--gold-hi)" />
          <span
            style={{
              fontSize: '0.82rem',
              fontWeight: 600,
              color: 'var(--gold-hi)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {curProjectTitle}
          </span>
        </div>
      ) : null}

      <nav className="side-nav">
        <span className="side-label">工坊导航</span>
        <SideLink to="/" icon={Library} end>
          作品集
        </SideLink>
        {id ? (
          <>
            <SideLink to={`/p/${id}`} icon={BookOpen} end>
              风格抽离
            </SideLink>
            <SideLink to={`/p/${id}/cards`} icon={Boxes}>
              牌库总览
            </SideLink>
            <SideLink to={`/p/${id}/fuse`} icon={FlaskConical}>
              风格融合
            </SideLink>
            <SideLink to={`/p/${id}/write`} icon={PenLine}>
              章节写作
            </SideLink>
            <SideLink to={`/p/${id}/bible`} icon={BookMarked}>
              设定集世界观
            </SideLink>
            <SideLink to={`/p/${id}/graph`} icon={Network}>
              关系图谱
            </SideLink>
          </>
        ) : null}
        <span className="side-label">配置与安全</span>
        <SideLink to="/settings" icon={KeyRound}>
          模型与密钥
        </SideLink>
      </nav>

      <div className="side-foot">
        <div className="side-user-chip">
          <div className="side-avatar">{avatarInitial}</div>
          <div className="side-user">{me.email}</div>
        </div>
        <button className="btn secondary sm" type="button" onClick={onLogout}>
          退出登录
        </button>
      </div>
    </aside>
  )
}

export default function App() {
  const location = useLocation()
  const navigate = useNavigate()
  const [me, setMe] = useState<Me | null>(null)
  const [ready, setReady] = useState(false)
  const [offline, setOffline] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const [helpOpen, setHelpOpen] = useState(false)
  const [cmdOpen, setCmdOpen] = useState(false)

  const currentProjectId = location.pathname.match(/^\/p\/([^/]+)/)?.[1]

  useEffect(() => {
    function onOnline() {
      setOffline(false)
    }
    function onOffline() {
      setOffline(true)
    }
    setOffline(!navigator.onLine)
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    api
      .me()
      .then((user) => {
        if (!cancelled) {
          setMe(user)
          setReady(true)
        }
      })
      .catch(() => {
        if (cancelled) return
        setMe(null)
        setReady(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  // 会话过期（任意请求返回 401）：软跳登录页，记住来路，登录后送回
  useEffect(() => {
    function onUnauth() {
      if (location.pathname === '/login' || location.pathname === '/register') return
      setMe(null)
      navigate('/login', { replace: true, state: { from: location.pathname + location.search } })
    }
    window.addEventListener('app:unauth', onUnauth)
    return () => window.removeEventListener('app:unauth', onUnauth)
  }, [navigate, location])

  // 路由切换后收起移动端抽屉
  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  const closeMenu = useCallback(() => setMenuOpen(false), [])

  // 全局快捷键处理 (Ctrl+K 打开指令台, ? 打开帮助, Escape 关闭)
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setCmdOpen((v) => !v)
      } else if (e.key === '?' && !isInputFocused()) {
        e.preventDefault()
        setHelpOpen((v) => !v)
      } else if (e.key === 'Escape') {
        if (cmdOpen) setCmdOpen(false)
        if (helpOpen) setHelpOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [cmdOpen, helpOpen])

  async function logout() {
    try {
      await api.logout()
    } finally {
      setMe(null)
      navigate('/login')
    }
  }

  return (
    <>
      <AuthContext.Provider value={{ me, setMe }}>
        {!ready ? (
          <div className="gate">
            <div className="gate-card">
              <div className="gate-brand">
                <span className="brand-mark">风格工坊</span>
                <span className="brand-sub">STYLE LAB · STUDIO</span>
              </div>
              <p className="muted">灵感凝聚，墨色铺开…</p>
            </div>
          </div>
        ) : (
          <>
            {offline ? <div className="offline-banner">已离线，正在等待网络恢复</div> : null}
            <div className={me ? 'app-shell authed' : 'app-shell'}>
              {me ? (
                <>
                  <header className="mobile-bar">
                    <button
                      className="menu-btn"
                      type="button"
                      aria-label="打开菜单"
                      onClick={() => setMenuOpen(true)}
                    >
                      <Menu size={19} aria-hidden="true" />
                    </button>
                    <Link className="mobile-brand" to="/">
                      风格工坊
                    </Link>
                    <button
                      className="menu-btn"
                      type="button"
                      aria-label="搜索"
                      onClick={() => setCmdOpen(true)}
                    >
                      <Search size={18} />
                    </button>
                  </header>
                  {menuOpen ? <div className="side-scrim" onClick={closeMenu} aria-hidden="true" /> : null}
                  <Sidebar
                    me={me}
                    onLogout={logout}
                    open={menuOpen}
                    onOpenCmd={() => setCmdOpen(true)}
                  />
                  <JobDock />
                  <CommandPalette
                    isOpen={cmdOpen}
                    onClose={() => setCmdOpen(false)}
                    currentProjectId={currentProjectId}
                  />
                  {helpOpen && <HelpDialog onClose={() => setHelpOpen(false)} />}
                  <button
                    className="help-fab"
                    type="button"
                    onClick={() => setHelpOpen((v) => !v)}
                    aria-label="显示快捷键帮助"
                    title="按 ? 显示帮助，Ctrl+K 快速搜索"
                  >
                    <Keyboard size={18} />
                  </button>
                </>
              ) : null}
              <div className={me ? 'table' : undefined}>
                <Routes>
                  <Route path="/login" element={me ? <Navigate to="/" replace /> : <Login mode="login" />} />
                  <Route path="/register" element={me ? <Navigate to="/" replace /> : <Login mode="register" />} />
                  <Route path="/" element={me ? <Projects /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id" element={me ? <ProjectHome /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/cards" element={me ? <Cards /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/lab/:cardId" element={me ? <Lab /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/fuse" element={me ? <Fuse /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/write" element={me ? <Write /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/bible" element={me ? <Bible /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/graph" element={me ? <Graph /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/chapter/:chapterId" element={me ? <ChapterPage /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/audit/:auditId" element={me ? <Audit /> : <Navigate to="/login" replace />} />
                  <Route path="/p/:id/sample/:sampleId" element={me ? <Sample /> : <Navigate to="/login" replace />} />
                  <Route path="/settings" element={me ? <Settings /> : <Navigate to="/login" replace />} />
                  <Route path="*" element={<NotFound />} />
                </Routes>
              </div>
            </div>
          </>
        )}
      </AuthContext.Provider>
    </>
  )
}
