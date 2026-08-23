import { FormEvent, useEffect, useState } from 'react'
import { Check, KeyRound, Lock, ShieldCheck, Trash2 } from 'lucide-react'
import { APIError, api, notify } from '../api'
import { confirm } from '../components/ConfirmDialog'
import { usePageTitle } from '../hooks'
import type { LLMKey } from '../types'

const providers = [
  {
    id: 'chat',
    label: 'Chat 接口 (/v1/chat/completions 通用格式 · DeepSeek, Qwen, SiliconFlow, OpenAI 等)',
    placeholderURL: '留空默认 https://api.openai.com，可填如 https://api.deepseek.com/v1',
  },
  {
    id: 'response',
    label: 'Response 接口 (/v1/responses 格式)',
    placeholderURL: '留空默认 https://api.openai.com',
  },
  {
    id: 'anthropic',
    label: 'Anthropic 接口 (/v1/messages 格式 · Claude 3.5 / 3.7 Sonnet 等)',
    placeholderURL: '留空默认 https://api.anthropic.com，支持中转反代地址',
  },
] as const

type ProviderId = (typeof providers)[number]['id']

export default function Settings() {
  usePageTitle('模型与密钥配置')
  const [keys, setKeys] = useState<LLMKey[]>([])
  const [provider, setProvider] = useState<ProviderId>('chat')
  const [apiKey, setApiKey] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  // 后四位直接从已存列表派生：切换提供方时自动跟对，不会残留上一家的
  const currentLast4 = keys.find((k) => k.provider === provider || (provider === 'chat' && k.provider === 'openai'))?.last4 ?? ''
  const currentProviderDef = providers.find((p) => p.id === provider) ?? providers[0]

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
      await api.putLLMKey(provider, apiKey, baseURL)
      setApiKey('')
      await load()
      notify(`已加密保存 ${provider.toUpperCase()} 接口密钥`, 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(p: string) {
    const ok = await confirm({
      title: `删除 ${p.toUpperCase()} 的接口密钥？`,
      body: '删除后使用该接口的抽离、融合、写作与审计任务将无法调用，需要重新配置。',
      confirmText: '确认删除',
      danger: true,
    })
    if (!ok) return
    setError('')
    try {
      await api.deleteLLMKey(p)
      await load()
      notify(`已安全移除 ${p.toUpperCase()} 接口密钥`, 'success')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '删除失败', 'error')
    }
  }

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <p className="kicker">KEY VAULT</p>
          <h1>模型接口与密钥配置 (BYOK)</h1>
          <p className="sub">
            支持 <strong>Chat 接口</strong>、<strong>Response 接口</strong>、<strong>Anthropic 接口</strong> 三大标准大模型协议。API Key 在服务端使用 AES-256-GCM 加密落盘，永不回传明文。
          </p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: '2rem' }}>
        <section className="panel">
          <div className="section-head">
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <Lock size={18} color="var(--gold-hi)" />
              <h2>录入或更新接口密钥</h2>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', color: 'var(--jade-hi)', fontSize: '0.78rem' }}>
              <ShieldCheck size={15} />
              <span>AES-256 GCM 加密保护</span>
            </div>
          </div>

          <form className="stack" onSubmit={onSave}>
            <label>
              模型接口类型
              <select value={provider} onChange={(e) => setProvider(e.target.value as ProviderId)}>
                {providers.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.label}
                  </option>
                ))}
              </select>
            </label>

            <label>
              Base URL 基础请求地址（可选）
              <input
                type="text"
                value={baseURL}
                onChange={(e) => setBaseURL(e.target.value)}
                placeholder={currentProviderDef.placeholderURL}
              />
            </label>

            <label>
              API Key 密钥明文
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                autoComplete="off"
                placeholder={currentLast4 ? `当前已配置 (尾号 ${currentLast4})，输入新值可覆盖` : 'sk-...'}
                required
              />
            </label>

            {error ? <p className="error">{error}</p> : null}

            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button className="btn" type="submit" disabled={busy || !apiKey.trim()}>
                <KeyRound size={16} />
                {busy ? '正在加密保存…' : '保存此接口密钥'}
              </button>
            </div>
          </form>
        </section>

        <section className="panel">
          <div className="section-head">
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <KeyRound size={18} color="var(--gold-hi)" />
              <h2>已就绪接口密钥库</h2>
            </div>
            <span className="muted">已保存 {keys.length} 组接口配置</span>
          </div>

          {keys.length === 0 ? (
            <div className="zone-empty">
              尚未录入任何模型接口密钥。请在上方输入 API Key 后保存。
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem' }}>
              {keys.map((k) => (
                <div
                  key={k.provider}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '1rem 1.4rem',
                    background: 'rgba(255,255,255,0.03)',
                    border: '1px solid var(--line-glass)',
                    borderRadius: 'var(--radius)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
                    <div
                      style={{
                        width: '32px',
                        height: '32px',
                        borderRadius: '50%',
                        background: 'rgba(45,212,191,0.12)',
                        color: 'var(--jade-hi)',
                        display: 'grid',
                        placeItems: 'center',
                        border: '1px solid rgba(45,212,191,0.3)',
                      }}
                    >
                      <Check size={16} />
                    </div>
                    <div>
                      <div style={{ fontWeight: 700, color: 'var(--ink)' }}>
                        {k.provider === 'chat' ? 'CHAT 接口 (/v1/chat/completions)' : k.provider === 'response' ? 'RESPONSE 接口 (/v1/responses)' : k.provider === 'anthropic' ? 'ANTHROPIC 接口 (/v1/messages)' : `${k.provider.toUpperCase()} 接口`}
                      </div>
                      <div style={{ fontSize: '0.78rem', color: 'var(--ink-faint)', fontFamily: 'var(--mono)' }}>
                        {k.base_url ? `${k.base_url} · ` : ''}尾号 ****{k.last4}
                      </div>
                    </div>
                  </div>
                  <button
                    className="btn ghost sm"
                    style={{ color: 'var(--cinnabar-hi)' }}
                    type="button"
                    onClick={() => void onDelete(k.provider)}
                    title="删除密钥"
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
