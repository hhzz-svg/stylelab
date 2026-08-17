import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../App'
import { APIError, api } from '../api'

type Props = {
  mode: 'login' | 'register'
}

export default function Login({ mode }: Props) {
  const navigate = useNavigate()
  const { setMe } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const isRegister = mode === 'register'

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
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof APIError ? err.message : '请求失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      <div className="card">
        <h1>{isRegister ? '注册' : '登录'}</h1>
        <p className="muted">用邮箱创建账号，保存项目、文本资产和风格卡片。</p>
        <form className="stack" onSubmit={onSubmit}>
          <label>
            邮箱
            <input
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </label>
          <label>
            密码
            <input
              type="password"
              autoComplete={isRegister ? 'new-password' : 'current-password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              required
            />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <button className="btn" type="submit" disabled={busy}>
            {busy ? '提交中…' : isRegister ? '注册并进入' : '登录'}
          </button>
        </form>
        <p className="muted">
          {isRegister ? (
            <>
              已有账号？<Link to="/login">去登录</Link>
            </>
          ) : (
            <>
              没有账号？<Link to="/register">去注册</Link>
            </>
          )}
        </p>
      </div>
    </div>
  )
}
