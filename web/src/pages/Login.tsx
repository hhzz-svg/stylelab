import { FormEvent, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Lock, LogIn, Mail, UserPlus } from 'lucide-react'
import { useAuth } from '../App'
import { APIError, api } from '../api'
import { usePageTitle } from '../hooks'

type Props = {
  mode: 'login' | 'register'
}

export default function Login({ mode }: Props) {
  const navigate = useNavigate()
  const location = useLocation()
  const { setMe } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const isRegister = mode === 'register'
  usePageTitle(isRegister ? '建立工坊' : '进入工坊')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (isRegister) {
        await api.register(email, password)
      } else {
        await api.login(email, password)
      }
      const user = await api.me()
      setMe(user)
      // 会话过期被送来登录的：回到来路页
      const from = (location.state as { from?: string } | null)?.from
      navigate(from && from.startsWith('/') ? from : '/', { replace: true })
    } catch (err) {
      setError(err instanceof APIError ? err.message : '请求失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="gate">
      <div className="gate-card">
        <div className="gate-brand">
          <span className="brand-mark" style={{ fontSize: '1.75rem' }}>风格工坊</span>
          <span className="brand-sub">STYLE LAB · STUDIO</span>
        </div>

        <h1 style={{ fontSize: '1.5rem', marginBottom: '0.4rem' }}>
          {isRegister ? '开创独立创作工坊' : '登入小说创作台'}
        </h1>
        <p className="muted" style={{ fontSize: '0.86rem', lineHeight: 1.6, marginBottom: '1.8rem' }}>
          抽离抽象技法 · 融合风格卡片 · 驱动 AI 小说长篇创作
        </p>

        <form className="stack" onSubmit={onSubmit}>
          <label style={{ textAlign: 'left' }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
              <Mail size={14} color="var(--gold)" />
              <span>电子邮箱</span>
            </span>
            <input
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="author@novel.ai"
              required
            />
          </label>

          <label style={{ textAlign: 'left' }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
              <Lock size={14} color="var(--gold)" />
              <span>登录密码</span>
            </span>
            <input
              type="password"
              autoComplete={isRegister ? 'new-password' : 'current-password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              placeholder="至少 8 位安全密码"
              required
            />
          </label>

          {error ? <p className="error">{error}</p> : null}

          <button className="btn" type="submit" disabled={busy} style={{ width: '100%', marginTop: '0.6rem' }}>
            {isRegister ? <UserPlus size={17} /> : <LogIn size={17} />}
            {busy ? '正在验证身份…' : isRegister ? '注册并即刻进入' : '进入创作工坊'}
          </button>
        </form>

        <div style={{ marginTop: '1.8rem', paddingTop: '1.2rem', borderTop: '1px solid var(--line-dim)', fontSize: '0.86rem' }}>
          <p className="muted" style={{ margin: 0 }}>
            {isRegister ? (
              <>
                已有工坊账号？<Link to="/login" style={{ fontWeight: 600, color: 'var(--gold-hi)' }}>直接登录</Link>
              </>
            ) : (
              <>
                初次造访工坊？<Link to="/register" style={{ fontWeight: 600, color: 'var(--gold-hi)' }}>立即建立账号</Link>
              </>
            )}
          </p>
        </div>
      </div>
    </div>
  )
}
