import { FormEvent, useEffect, useState } from 'react'
import { APIError, api } from '../api'
import type { LLMKey } from '../types'

const providers = ['openai', 'anthropic', 'compatible'] as const

export default function Settings() {
  const [keys, setKeys] = useState<LLMKey[]>([])
  const [provider, setProvider] = useState<(typeof providers)[number]>('openai')
  const [apiKey, setApiKey] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [savedLast4, setSavedLast4] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      const data = await api.listLLMKeys()
      setKeys(data.keys ?? [])
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function onSave(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const saved = await api.putLLMKey(provider, apiKey, baseURL)
      setSavedLast4(saved.last4)
      setApiKey('')
      await load()
    } catch (err) {
      setError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(p: string) {
    setError('')
    try {
      await api.deleteLLMKey(p)
      if (p === provider) setSavedLast4('')
      await load()
    } catch (err) {
      setError(err instanceof APIError ? err.message : '删除失败')
    }
  }

  return (
    <div className="page">
      <div className="card">
        <h1>模型密钥</h1>
        <p className="muted">密钥只保存在本服务，接口不会回传明文，保存后只显示后四位。</p>
        <form className="stack" onSubmit={onSave}>
          <label>
            提供方
            <select value={provider} onChange={(e) => setProvider(e.target.value as typeof provider)}>
              {providers.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </label>
          <label>
            Base URL（兼容接口可选）
            <input type="text" value={baseURL} onChange={(e) => setBaseURL(e.target.value)} />
          </label>
          <label>
            API Key
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              autoComplete="off"
              required
            />
          </label>
          {savedLast4 ? <p className="muted">已保存，后四位：{savedLast4}</p> : null}
          {error ? <p className="error">{error}</p> : null}
          <button className="btn" type="submit" disabled={busy}>
            {busy ? '保存中…' : '保存密钥'}
          </button>
        </form>
      </div>
      <div className="card">
        <h2>已保存的密钥</h2>
        {keys.length === 0 ? (
          <p className="muted">还没有密钥。抽离风格前需要先保存一个。</p>
        ) : (
          <ul className="list">
            {keys.map((k) => (
              <li key={k.provider}>
                <span>
                  {k.provider}
                  {k.base_url ? ` · ${k.base_url}` : ''} · ****{k.last4}
                </span>
                <button className="btn danger" type="button" onClick={() => onDelete(k.provider)}>
                  删除
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
