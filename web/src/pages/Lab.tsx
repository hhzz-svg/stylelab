import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { APIError, api } from '../api'
import DimensionSliders from '../components/DimensionSliders'
import JobProgress from '../components/JobProgress'
import { DIMENSION_KEYS, type DimensionKey, type StyleCard } from '../types'

function levelsFromCard(card: StyleCard): Record<string, number> {
  const next: Record<string, number> = {}
  for (const key of DIMENSION_KEYS) {
    next[key] = card.dimensions?.[key]?.level ?? 0
  }
  return next
}

export default function Lab() {
  const { id, cardId } = useParams()
  const projectId = id ?? ''
  const idOfCard = cardId ?? ''

  const [card, setCard] = useState<StyleCard | null>(null)
  const [maxVersion, setMaxVersion] = useState(1)
  const [levels, setLevels] = useState<Record<string, number>>({})
  const [name, setName] = useState('')
  const [model, setModel] = useState('')
  const [rewrite, setRewrite] = useState(false)
  const [premise, setPremise] = useState('')
  const [target, setTarget] = useState(1200)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState('')
  const [showSampleForm, setShowSampleForm] = useState(false)

  async function loadCurrent() {
    if (!idOfCard) return
    setError('')
    try {
      const data = await api.getCard(idOfCard)
      setCard(data)
      setMaxVersion(data.version)
      setLevels(levelsFromCard(data))
      setName(data.name)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载卡片失败')
    }
  }

  useEffect(() => {
    void loadCurrent()
  }, [idOfCard])

  const versions = useMemo(() => {
    return Array.from({ length: maxVersion }, (_, i) => i + 1)
  }, [maxVersion])

  async function onSelectVersion(n: number) {
    if (!idOfCard) return
    setError('')
    try {
      const data = n === maxVersion ? await api.getCard(idOfCard) : await api.getCardVersion(idOfCard, n)
      setCard(data)
      setLevels(levelsFromCard(data))
      setName(data.name)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '读取版本失败')
    }
  }

  function setLevel(key: DimensionKey, value: number) {
    setLevels((prev) => ({ ...prev, [key]: value }))
  }

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!idOfCard) return
    setError('')
    setBusy(true)
    try {
      const saved = await api.saveCardVersion(idOfCard, {
        name: name.trim(),
        levels,
        rewrite_summaries: rewrite,
        model: model.trim(),
      })
      setCard(saved)
      setMaxVersion(saved.version)
      setLevels(levelsFromCard(saved))
      setName(saved.name)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onExport() {
    if (!idOfCard) return
    setError('')
    try {
      await api.exportCard(idOfCard)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '导出失败')
    }
  }

  async function startAudit() {
    if (!idOfCard) return
    setError('')
    setBusy(true)
    try {
      const res = await api.auditCard(idOfCard, model.trim())
      setJobId(res.job_id)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '审计提交失败')
    } finally {
      setBusy(false)
    }
  }

  async function startSample() {
    const text = premise.trim()
    if ([...text].length === 0) {
      setError('请填写试写前提（不超过 80 字）')
      return
    }
    if ([...text].length > 80) {
      setError('试写前提不能超过 80 字')
      return
    }
    if (!idOfCard) return
    setError('')
    setBusy(true)
    try {
      const res = await api.sampleCard(idOfCard, text, target, model.trim())
      setJobId(res.job_id)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '试写提交失败')
    } finally {
      setBusy(false)
    }
  }

  const premiseCount = [...premise].length

  return (
    <div className="page page-wide">
      <p>
        <Link to={`/p/${projectId}`}>← 返回项目</Link>
        {' · '}
        <Link to={`/p/${projectId}/fuse`}>风格融合</Link>
      </p>
      <div className="card">
        <div className="row" style={{ justifyContent: 'space-between' }}>
          <h1 style={{ marginBottom: 0 }}>{card?.name ?? '风格工坊'}</h1>
          <span className="muted">
            {card ? `${card.kind} · v${card.version}` : idOfCard}
          </span>
        </div>
        {card ? (
          <form className="stack" onSubmit={onSave}>
            <div className="row">
              <label style={{ flex: 1 }}>
                名称
                <input type="text" value={name} onChange={(e) => setName(e.target.value)} />
              </label>
              <label>
                版本
                <select
                  value={card.version}
                  onChange={(e) => void onSelectVersion(Number(e.target.value))}
                >
                  {versions.map((n) => (
                    <option key={n} value={n}>
                      v{n}
                      {n === maxVersion ? '（当前）' : ''}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                模型（可选）
                <input type="text" value={model} onChange={(e) => setModel(e.target.value)} />
              </label>
            </div>
            <DimensionSliders levels={levels} onChange={setLevel} disabled={busy} />
            {card.prohibitions?.length ? (
              <div>
                <h3>约束</h3>
                <ul>
                  {card.prohibitions.map((p) => (
                    <li key={p}>{p}</li>
                  ))}
                </ul>
              </div>
            ) : null}
            <label className="row" style={{ display: 'flex' }}>
              <input
                type="checkbox"
                checked={rewrite}
                onChange={(e) => setRewrite(e.target.checked)}
              />
              保存时重写已改维度的摘要与技法
            </label>
            <div className="row">
              <button className="btn" type="submit" disabled={busy}>
                {busy ? '处理中…' : '保存为新版本'}
              </button>
              <button className="btn secondary" type="button" onClick={() => void onExport()}>
                导出
              </button>
              <button className="btn secondary" type="button" onClick={() => void startAudit()} disabled={busy}>
                审计
              </button>
              <button
                className="btn secondary"
                type="button"
                onClick={() => setShowSampleForm((v) => !v)}
              >
                试写
              </button>
            </div>
          </form>
        ) : (
          <p className="muted">正在加载卡片…</p>
        )}
        {showSampleForm ? (
          <div className="stack" style={{ marginTop: '1rem' }}>
            <label>
              试写前提（{premiseCount}/80）
              <textarea
                value={premise}
                maxLength={240}
                rows={3}
                onChange={(e) => setPremise(e.target.value)}
              />
            </label>
            <label>
              目标字数 {target}
              <input
                type="range"
                min={800}
                max={2000}
                step={50}
                value={target}
                onChange={(e) => setTarget(Number(e.target.value))}
              />
            </label>
            <button className="btn" type="button" onClick={() => void startSample()} disabled={busy}>
              开始试写
            </button>
          </div>
        ) : null}
        {error ? <p className="error">{error}</p> : null}
        {jobId ? <JobProgress jobId={jobId} projectId={projectId} /> : null}
      </div>
    </div>
  )
}
