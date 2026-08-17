import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { APIError, api } from '../api'
import JobProgress from '../components/JobProgress'
import {
  DIMENSION_KEYS,
  DIMENSION_LABELS,
  type CardSummary,
  type DimensionKey,
  type ParentRef,
} from '../types'

type ParentDraft = {
  selected: boolean
  version: number
  dims: Record<DimensionKey, boolean>
  weights: Record<DimensionKey, number>
}

function emptyDraft(version: number): ParentDraft {
  const dims = {} as Record<DimensionKey, boolean>
  const weights = {} as Record<DimensionKey, number>
  for (const key of DIMENSION_KEYS) {
    dims[key] = true
    weights[key] = 0
  }
  return { selected: false, version, dims, weights }
}

function dimWeightSums(drafts: Record<string, ParentDraft>, selectedIds: string[]) {
  const sums: Record<string, number> = {}
  for (const key of DIMENSION_KEYS) {
    let sum = 0
    for (const id of selectedIds) {
      const draft = drafts[id]
      if (draft?.dims[key]) {
        sum += Number(draft.weights[key] || 0)
      }
    }
    sums[key] = sum
  }
  return sums
}

export default function Fuse() {
  const { id } = useParams()
  const projectId = id ?? ''
  const [cards, setCards] = useState<CardSummary[]>([])
  const [drafts, setDrafts] = useState<Record<string, ParentDraft>>({})
  const [name, setName] = useState('融合风格')
  const [model, setModel] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState('')

  useEffect(() => {
    if (!projectId) return
    api
      .listCards(projectId)
      .then((data) => {
        const list = data.cards ?? []
        setCards(list)
        setDrafts((prev) => {
          const next = { ...prev }
          for (const c of list) {
            if (!next[c.id]) next[c.id] = emptyDraft(c.current_version)
          }
          return next
        })
      })
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '加载卡片失败')
      })
  }, [projectId])

  const selectedIds = useMemo(
    () => cards.filter((c) => drafts[c.id]?.selected).map((c) => c.id),
    [cards, drafts],
  )
  const sums = useMemo(() => dimWeightSums(drafts, selectedIds), [drafts, selectedIds])
  const weightsOk = DIMENSION_KEYS.every((key) => sums[key] === 100)

  function patch(idOfCard: string, fn: (d: ParentDraft) => ParentDraft) {
    setDrafts((prev) => {
      const cur = prev[idOfCard] ?? emptyDraft(1)
      return { ...prev, [idOfCard]: fn(cur) }
    })
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (selectedIds.length < 2 || selectedIds.length > 4) {
      setError('请选择 2–4 张卡片')
      return
    }
    if (!weightsOk) {
      setError('每个维度被勾选卡片的权重之和必须为 100')
      return
    }
    const parents: ParentRef[] = selectedIds.map((cardId) => {
      const draft = drafts[cardId]
      const dims = DIMENSION_KEYS.filter((key) => draft.dims[key])
      const weights: Record<string, number> = {}
      for (const key of dims) {
        weights[key] = Number(draft.weights[key] || 0)
      }
      return {
        card_id: cardId,
        version: draft.version,
        dims,
        weights,
      }
    })
    setError('')
    setBusy(true)
    try {
      const res = await api.fuse(projectId, name.trim() || '融合风格', parents, model.trim())
      setJobId(res.job_id)
    } catch (err) {
      setError(err instanceof APIError ? err.message : '融合提交失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page page-wide">
      <p>
        <Link to={`/p/${projectId}`}>← 返回项目</Link>
      </p>
      <div className="card">
        <h1>风格融合</h1>
        <p className="muted">选择 2–4 张卡片。每个维度只统计已勾选卡片的权重，提交前各维之和须为 100。</p>
        <form className="stack" onSubmit={onSubmit}>
          <div className="row">
            <label style={{ flex: 1 }}>
              新卡片名称
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              模型（可选）
              <input type="text" value={model} onChange={(e) => setModel(e.target.value)} />
            </label>
          </div>
          {cards.length === 0 ? (
            <p className="muted">这个项目还没有风格卡片。先抽离或手工保存一张。</p>
          ) : (
            <div className="fuse-grid">
              {cards.map((c) => {
                const draft = drafts[c.id] ?? emptyDraft(c.current_version)
                return (
                  <section key={c.id} className="fuse-card">
                    <label className="row">
                      <input
                        type="checkbox"
                        checked={draft.selected}
                        disabled={!draft.selected && selectedIds.length >= 4}
                        onChange={(e) =>
                          patch(c.id, (d) => ({ ...d, selected: e.target.checked }))
                        }
                      />
                      <strong>{c.name}</strong>
                    </label>
                    <p className="muted">
                      {c.kind} · 当前 v{c.current_version}
                    </p>
                    <label>
                      使用版本
                      <input
                        type="number"
                        min={1}
                        max={c.current_version}
                        value={draft.version}
                        onChange={(e) =>
                          patch(c.id, (d) => ({
                            ...d,
                            version: Number(e.target.value) || c.current_version,
                          }))
                        }
                      />
                    </label>
                    {DIMENSION_KEYS.map((key) => (
                      <div key={key} className="fuse-dim">
                        <label>
                          <input
                            type="checkbox"
                            checked={draft.dims[key]}
                            onChange={(e) =>
                              patch(c.id, (d) => ({
                                ...d,
                                dims: { ...d.dims, [key]: e.target.checked },
                              }))
                            }
                          />
                          {DIMENSION_LABELS[key]}
                        </label>
                        <input
                          type="number"
                          min={0}
                          max={100}
                          disabled={!draft.dims[key] || !draft.selected}
                          value={draft.weights[key]}
                          onChange={(e) =>
                            patch(c.id, (d) => ({
                              ...d,
                              weights: { ...d.weights, [key]: Number(e.target.value) },
                            }))
                          }
                        />
                      </div>
                    ))}
                  </section>
                )
              })}
            </div>
          )}
          <div className="helper">
            <p>各维权重合计（须为 100）</p>
            <ul className="weight-sums">
              {DIMENSION_KEYS.map((key) => (
                <li key={key} className={sums[key] === 100 ? '' : 'error'}>
                  {DIMENSION_LABELS[key]}：{sums[key] ?? 0}
                </li>
              ))}
            </ul>
          </div>
          {error ? <p className="error">{error}</p> : null}
          <button
            className="btn"
            type="submit"
            disabled={busy || !!jobId || selectedIds.length < 2 || !weightsOk}
          >
            {busy ? '提交中…' : '开始融合'}
          </button>
        </form>
        {jobId ? <JobProgress jobId={jobId} projectId={projectId} /> : null}
      </div>
    </div>
  )
}
