import { createContext, useContext, useEffect, useState } from 'react'
import { Link, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { APIError, api } from './api'
import type { Me } from './types'
import Login from './pages/Login'
import Projects from './pages/Projects'
import ProjectHome from './pages/ProjectHome'
import Lab from './pages/Lab'
import Fuse from './pages/Fuse'
import Audit from './pages/Audit'
import Sample from './pages/Sample'
import Settings from './pages/Settings'

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

export default function App() {
  const location = useLocation()
  const navigate = useNavigate()
  const [me, setMe] = useState<Me | null>(null)
  const [ready, setReady] = useState(false)
  const publicAuth = location.pathname === '/login' || location.pathname === '/register'

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
      .catch((err: unknown) => {
        if (cancelled) return
        setMe(null)
        setReady(true)
        if (!publicAuth && err instanceof APIError && err.status === 401) {
          navigate('/login', { replace: true })
        }
      })
    return () => {
      cancelled = true
    }
  }, [navigate, publicAuth])

  async function logout() {
    try {
      await api.logout()
    } finally {
      setMe(null)
      navigate('/login')
    }
  }

  if (!ready) {
    return <div className="page">加载中…</div>
  }

  return (
    <AuthContext.Provider value={{ me, setMe }}>
      <div className="app-shell">
        <header className="topbar">
          <Link className="brand" to="/">
            Style Lab
          </Link>
          {me ? (
            <nav className="nav-links">
              <Link to="/">项目</Link>
              <Link to="/settings">设置</Link>
              <span className="nav-user">
                <span>{me.email}</span>
                <button className="btn secondary" type="button" onClick={logout}>
                  退出
                </button>
              </span>
            </nav>
          ) : (
            <nav className="nav-links">
              <Link to="/login">登录</Link>
              <Link to="/register">注册</Link>
            </nav>
          )}
        </header>
        <Routes>
          <Route path="/login" element={me ? <Navigate to="/" replace /> : <Login mode="login" />} />
          <Route path="/register" element={me ? <Navigate to="/" replace /> : <Login mode="register" />} />
          <Route path="/" element={me ? <Projects /> : <Navigate to="/login" replace />} />
          <Route path="/p/:id" element={me ? <ProjectHome /> : <Navigate to="/login" replace />} />
          <Route path="/p/:id/lab/:cardId" element={me ? <Lab /> : <Navigate to="/login" replace />} />
          <Route path="/p/:id/fuse" element={me ? <Fuse /> : <Navigate to="/login" replace />} />
          <Route path="/p/:id/audit/:auditId" element={me ? <Audit /> : <Navigate to="/login" replace />} />
          <Route path="/p/:id/sample/:sampleId" element={me ? <Sample /> : <Navigate to="/login" replace />} />
          <Route path="/settings" element={me ? <Settings /> : <Navigate to="/login" replace />} />
        </Routes>
      </div>
    </AuthContext.Provider>
  )
}
