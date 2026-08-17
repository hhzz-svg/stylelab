import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { APIError, api } from '../api'
import JobProgress from '../components/JobProgress'
import type { Asset, CardSummary, Project } from '../types'

export default function ProjectHome() {
  const { id } = useParams()
  const projectId = id ?? ''
  const [project, setProject] = useState<Project | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [cards, setCards] = useState<CardSummary[]>([])
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [file, setFile] = useState<File | null>(null)
  const [cardName, setCardName] = useState('风格卡片')
  const [model, setModel] = useState('')
  const [jobId, setJobId] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    if (!projectId) return
    try {
      const [p, a, c] = await Promise.all([
        api.getProject(projectId),
        api.listAssets(projectId),
        api.listCards(projectId),
      ])
      setProject(p)
      setAssets(a.assets ?? [])
      setCards(c.cards ?? [])
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载失败')
    }
  }

  useEffect(() => {
    void load()
  }, [projectId])

  async function onUpload(e: FormEvent) {
    e.preventDefault()
    if (!file || !projectId) return
    setError('')
    setBusy(true)
    try {
      const created = await api.uploadAsset(projectId, file)
      setAssets((prev) => [created, ...prev])
      setSelected((prev) => ({ ...prev, [created.id]: true }))
      setFile(null)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '上传失败')
    } finally {
      setBusy(false)
    }
  }

  async function startExtract() {
    const assetIds = assets.filter((a) => selected[a.id]).map((a) => a.id)
    if (assetIds.length === 0) {
      setError('请先选择至少一个文本资产')
      return
    }
    setError('')
    setBusy(true)
    try {
      const res = await api.extract(projectId, assetIds, cardName.trim() || '风格卡片', model.trim())
      setJobId(res.job_id)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '抽离失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      <p>
        <Link to="/">← 全部项目</Link>
      </p>
      <div className="card">
        <h1>{project?.name ?? '项目'}</h1>
        <p className="helper">
          请确保你有权使用该文本。系统只抽取抽象技法，不会把大段原文写入风格卡片。
        </p>
        <form className="stack" onSubmit={onUpload}>
          <label>
            上传 .txt / .md
            <input
              type="file"
              accept=".txt,.md,text/plain,text/markdown"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </label>
          <button className="btn" type="submit" disabled={!file || busy}>
            {busy ? '处理中…' : '上传文本'}
          </button>
        </form>
      </div>

      <div className="card">
        <h2>文本资产</h2>
        {assets.length === 0 ? (
          <p className="muted">还没有资产。上传 txt 或 md 后即可抽离风格。</p>
        ) : (
          <ul className="list">
            {assets.map((a) => (
              <li key={a.id}>
                <label>
                  <input
                    type="checkbox"
                    checked={!!selected[a.id]}
                    onChange={(e) =>
                      setSelected((prev) => ({ ...prev, [a.id]: e.target.checked }))
                    }
                  />{' '}
                  {a.filename}
                </label>
                <span className="muted">
                  {a.rune_count} 字 / {a.chapter_count} 章
                </span>
              </li>
            ))}
          </ul>
        )}
        <div className="stack" style={{ marginTop: '1rem' }}>
          <label>
            卡片名称
            <input type="text" value={cardName} onChange={(e) => setCardName(e.target.value)} />
          </label>
          <label>
            模型（可选）
            <input type="text" value={model} onChange={(e) => setModel(e.target.value)} />
          </label>
          <button className="btn" type="button" onClick={startExtract} disabled={busy || !!jobId}>
            抽离风格
          </button>
        </div>
        {error ? <p className="error">{error}</p> : null}
        {jobId ? <JobProgress jobId={jobId} projectId={projectId} /> : null}
      </div>

      <div className="card">
        <div className="row" style={{ justifyContent: 'space-between' }}>
          <h2 style={{ marginBottom: 0 }}>风格卡片</h2>
          <Link to={`/p/${projectId}/fuse`}>风格融合</Link>
        </div>
        {cards.length === 0 ? (
          <p className="muted">还没有卡片。抽离风格成功后会在这里列出。</p>
        ) : (
          <ul className="list">
            {cards.map((card) => (
              <li key={card.id}>
                <Link to={`/p/${projectId}/lab/${card.id}`}>{card.name}</Link>
                <span className="muted">
                  {card.kind} · v{card.current_version}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
