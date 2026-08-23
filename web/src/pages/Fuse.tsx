import { type DragEvent, FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowRight, Check, Library, Plus, X } from 'lucide-react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import CardDeckDrawer from '../components/CardDeckDrawer'
import Crumb from '../components/Crumb'
import JobProgress from '../components/JobProgress'
import Skeleton from '../components/Skeleton'
import StyleCardTile from '../components/StyleCardTile'
import { usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import {
  DIMENSION_KEYS,
  DIMENSION_LABELS,
  type BlendMode,
  type CardSummary,
  type DimensionKey,
  type JobResult,
  type JobStatus,
  type ParentRef,
  type StyleCard,
} from '../types'

const CARD_DRAG_TYPE = 'application/x-stylelab-card'

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

function splitEven(total: number, count: number): number[] {
  if (count <= 0) return []
  const base = Math.floor(total / count)
  const rem = total - base * count
  return Array.from({ length: count }, (_, index) => base + (index < rem ? 1 : 0))
}

function cloneDrafts(source: Record<string, ParentDraft>) {
  const next: Record<string, ParentDraft> = {}
  for (const [id, draft] of Object.entries(source)) {
    next[id] = { ...draft, dims: { ...draft.dims }, weights: { ...draft.weights } }
  }
  return next
}

function balanceDimension(next: Record<string, ParentDraft>, ids: string[], key: DimensionKey) {
  const active = ids.filter((id) => next[id]?.dims[key])
  const shares = splitEven(100, active.length)
  ids.forEach((id) => {
    const index = active.indexOf(id)
    if (next[id]) next[id].weights[key] = index >= 0 ? shares[index] : 0
  })
}

function balanceAll(next: Record<string, ParentDraft>, ids: string[]) {
  DIMENSION_KEYS.forEach((key) => balanceDimension(next, ids, key))
}

function applyFavor(next: Record<string, ParentDraft>, ids: string[], cardId: string) {
  DIMENSION_KEYS.forEach((key) => {
    const activeOthers = ids.filter((idValue) => idValue !== cardId && next[idValue]?.dims[key])
    const targetOn = !!next[cardId]?.dims[key]
    if (!targetOn) {
      balanceDimension(next, ids, key)
      return
    }
    next[cardId].weights[key] = activeOthers.length ? 70 : 100
    splitEven(activeOthers.length ? 30 : 0, activeOthers.length).forEach((share, index) => {
      next[activeOthers[index]].weights[key] = share
    })
  })
}

function applyPreset(
  next: Record<string, ParentDraft>,
  ids: string[],
  mode: BlendMode,
  dominantCardId?: string,
) {
  if (mode === 'dominant' && dominantCardId && ids.includes(dominantCardId)) {
    applyFavor(next, ids, dominantCardId)
    return
  }
  balanceAll(next, ids)
}

function protectLastSources(next: Record<string, ParentDraft>, remaining: string[]) {
  DIMENSION_KEYS.forEach((key) => {
    const still = remaining.filter((id) => next[id]?.dims[key])
    if (still.length === 0 && remaining[0] && next[remaining[0]]) {
      next[remaining[0]].dims[key] = true
    }
  })
}

function sumsFor(drafts: Record<string, ParentDraft>, ids: string[]) {
  const sums = {} as Record<DimensionKey, number>
  DIMENSION_KEYS.forEach((key) => {
    sums[key] = ids.reduce((total, id) => {
      const draft = drafts[id]
      return total + (draft?.dims[key] ? Number(draft.weights[key] || 0) : 0)
    }, 0)
  })
  return sums
}

function detailKey(cardId: string, version: number) {
  return cardId + ':' + version
}

function previewLevels(
  ids: string[],
  drafts: Record<string, ParentDraft>,
  details: Record<string, StyleCard>,
): Record<DimensionKey, number> | null {
  const out = {} as Record<DimensionKey, number>
  for (const key of DIMENSION_KEYS) {
    const active = ids.filter((id) => drafts[id]?.dims[key])
    if (!active.length) {
      out[key] = 0
      continue
    }
    let weighted = 0
    for (const id of active) {
      const draft = drafts[id]
      const card = details[detailKey(id, draft.version)]
      if (!card) return null
      weighted += (card.dimensions?.[key]?.level ?? 0) * Number(draft.weights[key] || 0) / 100
    }
    out[key] = Math.round(weighted)
  }
  return out
}

function readDroppedCardId(event: DragEvent) {
  return event.dataTransfer.getData(CARD_DRAG_TYPE) || event.dataTransfer.getData('text/plain')
}

export default function Fuse() {
  const { id } = useParams()
  const projectId = id ?? ''
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  usePageTitle('风格融合')

  const [cards, setCards] = useState<CardSummary[]>([])
  const [drafts, setDrafts] = useState<Record<string, ParentDraft>>({})
  const [details, setDetails] = useState<Record<string, StyleCard>>({})
  const [loaded, setLoaded] = useState(false)
  const [deckOpen, setDeckOpen] = useState(false)
  const [name, setName] = useState('融合风格')
  const [model, setModel] = useState('')
  const [mode, setMode] = useState<BlendMode>('balanced')
  const [dominantCardId, setDominantCardId] = useState('')
  const [focusDim, setFocusDim] = useState<DimensionKey>('sentence_rhythm')
  const [resultCard, setResultCard] = useState<StyleCard | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState('')
  const [jobActive, setJobActive] = useState(false)
  const [dropSlot, setDropSlot] = useState<number | null>(null)
  const detailsRef = useRef(details)
  detailsRef.current = details

  useEffect(() => {
    if (!projectId) return
    api.listCards(projectId)
      .then((data) => {
        const list = data.cards ?? []
        setCards(list)
        setDrafts((previous) => {
          const next = { ...previous }
          list.forEach((card) => {
            if (!next[card.id]) next[card.id] = emptyDraft(card.current_version)
          })
          const requested = (searchParams.get('cards') ?? '').split(',').map((value) => value.trim()).filter(Boolean)
          requested.slice(0, 4).forEach((cardId) => {
            const draft = next[cardId]
            if (draft && !draft.selected) {
              next[cardId] = { ...draft, selected: true, version: list.find((card) => card.id === cardId)?.current_version ?? draft.version }
            }
          })
          const selectedIds = list.filter((card) => next[card.id]?.selected).map((card) => card.id).slice(0, 4)
          selectedIds.forEach((cardId) => { next[cardId].selected = true })
          applyPreset(next, selectedIds, 'balanced')
          return next
        })
        setLoaded(true)
      })
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '卡片加载失败')
        setLoaded(true)
      })
  }, [projectId, searchParams])

  const selectedIds = useMemo(
    () => cards.filter((card) => drafts[card.id]?.selected).map((card) => card.id),
    [cards, drafts],
  )
  const selectedCards = cards.filter((card) => drafts[card.id]?.selected)
  const sums = useMemo(() => sumsFor(drafts, selectedIds), [drafts, selectedIds])
  const weightsOk = selectedIds.length >= 2 && DIMENSION_KEYS.every((key) => sums[key] === 100)
  const focusActiveIds = selectedIds.filter((cardId) => drafts[cardId]?.dims[focusDim])
  const selectionKey = selectedIds.map((cardId) => detailKey(cardId, drafts[cardId]?.version ?? 0)).join(',')
  const preview = useMemo(
    () => (selectedIds.length >= 2 ? previewLevels(selectedIds, drafts, details) : null),
    [selectedIds, drafts, details],
  )

  useEffect(() => {
    if (!selectionKey) return
    let cancelled = false
    async function loadSelected() {
      for (const cardId of selectedIds) {
        const version = drafts[cardId]?.version
        if (!version) continue
        const key = detailKey(cardId, version)
        if (detailsRef.current[key]) continue
        try {
          const summary = cards.find((card) => card.id === cardId)
          const card = summary && summary.current_version === version
            ? await api.getCard(cardId)
            : await api.getCardVersion(cardId, version)
          if (cancelled) return
          setDetails((previous) => previous[key] ? previous : { ...previous, [key]: card })
        } catch (err) {
          if (cancelled) return
          setError(err instanceof APIError ? err.message : '来源卡读取失败')
        }
      }
    }
    void loadSelected()
    return () => {
      cancelled = true
    }
  }, [selectionKey, selectedIds, drafts, cards])

  function onJobStatus(status: JobStatus) {
    setJobActive(status === 'queued' || status === 'running')
  }

  function currentVersionOf(cardId: string, fallback: number) {
    return cards.find((card) => card.id === cardId)?.current_version ?? fallback
  }

  function rewriteDrafts(
    mutator: (next: Record<string, ParentDraft>) => BlendMode | void,
    nextMode = mode,
    nextDominant = dominantCardId,
  ) {
    setDrafts((previous) => {
      const next = cloneDrafts(previous)
      const resolvedMode = mutator(next) ?? nextMode
      const ids = cards.filter((card) => next[card.id]?.selected).map((card) => card.id)
      if (resolvedMode !== 'custom') {
        applyPreset(next, ids, resolvedMode, nextDominant)
      }
      return next
    })
  }

  function toggleCard(cardId: string) {
    const turningOn = !drafts[cardId]?.selected
    if (turningOn && selectedIds.length >= 4) {
      setError('融合槽最多放入 4 张卡片')
      return
    }
    setError('')
    const nextMode: BlendMode = mode === 'custom' ? 'balanced' : mode
    const nextDominant = turningOn
      ? (dominantCardId || cardId)
      : (dominantCardId === cardId ? (selectedIds.find((idValue) => idValue !== cardId) ?? '') : dominantCardId)
    if (nextMode !== mode) setMode(nextMode)
    if (nextDominant !== dominantCardId) setDominantCardId(nextDominant)
    rewriteDrafts((next) => {
      const current = next[cardId] ?? emptyDraft(currentVersionOf(cardId, 1))
      next[cardId] = {
        ...current,
        selected: turningOn,
        version: turningOn ? currentVersionOf(cardId, current.version) : current.version,
        dims: { ...current.dims },
        weights: { ...current.weights },
      }
      if (!turningOn) {
        protectLastSources(next, cards.filter((card) => card.id !== cardId && next[card.id]?.selected).map((card) => card.id))
      }
      return nextMode
    }, nextMode, nextDominant)
  }

  function evenAll() {
    setMode('balanced')
    rewriteDrafts(() => 'balanced', 'balanced')
  }

  function favorCard(cardId: string) {
    setMode('dominant')
    setDominantCardId(cardId)
    setDrafts((previous) => {
      const next = cloneDrafts(previous)
      applyFavor(next, selectedIds, cardId)
      return next
    })
  }

  function setWeight(cardId: string, key: DimensionKey, raw: number) {
    setMode('custom')
    setDrafts((previous) => {
      const next = cloneDrafts(previous)
      const current = next[cardId]
      if (!current?.dims[key]) return previous
      const others = selectedIds.filter((idValue) => idValue !== cardId && next[idValue]?.dims[key])
      const value = others.length ? Math.max(0, Math.min(100, Math.round(raw))) : 100
      current.weights[key] = value
      splitEven(100 - value, others.length).forEach((share, index) => {
        next[others[index]].weights[key] = share
      })
      return next
    })
  }

  function toggleDim(cardId: string, key: DimensionKey, enabled: boolean) {
    const activeForKey = selectedIds.filter((idValue) => drafts[idValue]?.dims[key])
    if (!enabled && activeForKey.length <= 1) {
      setError('每个维度至少保留一张来源卡片')
      return
    }
    setError('')
    setDrafts((previous) => {
      const next = cloneDrafts(previous)
      if (!next[cardId]) return previous
      next[cardId].dims[key] = enabled
      if (mode === 'dominant' && dominantCardId) applyFavor(next, selectedIds, dominantCardId)
      else balanceDimension(next, selectedIds, key)
      return next
    })
  }

  function onSlotDragOver(event: DragEvent<HTMLElement>, index: number) {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'copy'
    setDropSlot(index)
  }

  function onSlotDrop(event: DragEvent<HTMLElement>) {
    event.preventDefault()
    setDropSlot(null)
    const cardId = readDroppedCardId(event)
    if (!cardId || !cards.some((card) => card.id === cardId)) return
    if (drafts[cardId]?.selected) return
    toggleCard(cardId)
  }

  async function onFuseSucceeded(result: JobResult) {
    if (!result.card_id) return
    try {
      const [card, list] = await Promise.all([api.getCard(result.card_id), api.listCards(projectId)])
      setResultCard(card)
      setCards(list.cards ?? [])
      setDrafts((previous) => {
        const next = { ...previous }
        for (const summary of list.cards ?? []) {
          if (!next[summary.id]) next[summary.id] = emptyDraft(summary.current_version)
        }
        return next
      })
      notify('融合卡片已加入牌库，并在当前页面显示', 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '融合结果读取失败')
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (selectedIds.length < 2 || selectedIds.length > 4) {
      setError('请选择 2–4 张卡片')
      return
    }
    if (!weightsOk) {
      setError('每个维度的权重必须合计 100')
      return
    }
    const parents: ParentRef[] = selectedIds.map((cardId) => {
      const draft = drafts[cardId]
      const dims = DIMENSION_KEYS.filter((key) => draft.dims[key])
      const weights: Record<string, number> = {}
      dims.forEach((key) => { weights[key] = Number(draft.weights[key] || 0) })
      return { card_id: cardId, version: draft.version, dims, weights }
    })
    setError('')
    setResultCard(null)
    setBusy(true)
    try {
      const outputName = name.trim() || '融合风格'
      const result = await api.fuse(projectId, outputName, parents, model.trim())
      setJobId(result.job_id)
      setJobActive(true)
      registerJob({ jobId: result.job_id, projectId, kind: 'fuse', label: `融合「${outputName}」`, startedAt: Date.now() })
      notify('融合任务已提交，结果会原位显现', 'info')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '融合提交失败')
    } finally {
      setBusy(false)
    }
  }

  const hint = selectedIds.length === 0
    ? '先放入 2–4 张卡片，再为九个维度分配来源。'
    : selectedIds.length < 2
      ? '再放入一张卡片即可开始配比。'
      : weightsOk ? '每个维度已合计 100，可以生成融合卡。' : '请完成当前维度的权重配比。'

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current="融合" />
      <div className="page-head">
        <div>
          <p className="kicker">FUSION BENCH</p>
          <h1>融合工作台</h1>
          <p className="sub">{hint}</p>
        </div>
        <div className="page-actions">
          <div className="status-chip"><span>融合槽</span><strong>{selectedIds.length}/4</strong></div>
          <button className="btn secondary" type="button" onClick={() => setDeckOpen(true)}>
            <Library size={17} />打开牌库
          </button>
        </div>
      </div>

      {!loaded ? (
        <div className="fuse-layout">
          <Skeleton h={310} />
          <Skeleton h={510} />
        </div>
      ) : cards.length === 0 ? (
        <div className="empty-state">
          <h2>牌库还是空的</h2>
          <p className="muted">先去抽离工作台生成第一张风格卡。</p>
          <div style={{ display: 'flex', gap: '0.6rem', flexWrap: 'wrap', justifyContent: 'center', marginTop: '0.8rem' }}>
            <Link className="btn" to={`/p/${projectId}`}>
              去抽离工作台
            </Link>
            <Link className="btn secondary" to={`/p/${projectId}/cards`}>
              查看牌库
            </Link>
          </div>
        </div>
      ) : (
        <form className="stack" onSubmit={onSubmit}>
          <section className="fuse-slot-panel">
            <div className="section-head">
              <div><span className="eyebrow">01 · SOURCES</span><h2>融合槽</h2></div>
              <span className="muted">点击或拖入卡片；牌库抽屉支持键盘选择</span>
            </div>
            <div className="fuse-slots">
              {Array.from({ length: 4 }, (_, index) => {
                const card = selectedCards[index]
                return card ? (
                  <div
                    className={'fuse-slot filled' + (dropSlot === index ? ' drop-over' : '')}
                    key={card.id}
                    onDragOver={(event) => onSlotDragOver(event, index)}
                    onDragLeave={() => setDropSlot((current) => current === index ? null : current)}
                    onDrop={onSlotDrop}
                  >
                    <StyleCardTile card={card} selected onClick={() => toggleCard(card.id)} actionLabel="点击移出" />
                    <span className="fuse-slot-index">0{index + 1} · v{drafts[card.id]?.version ?? card.current_version}</span>
                    <button className="fuse-slot-remove" type="button" onClick={() => toggleCard(card.id)} aria-label={`移出${card.name}`}><X size={15} /></button>
                  </div>
                ) : (
                  <button
                    className={'fuse-slot empty' + (dropSlot === index ? ' drop-over' : '')}
                    type="button"
                    onClick={() => setDeckOpen(true)}
                    onDragOver={(event) => onSlotDragOver(event, index)}
                    onDragLeave={() => setDropSlot((current) => current === index ? null : current)}
                    onDrop={onSlotDrop}
                    aria-label={`打开第 ${index + 1} 个融合槽`}
                  >
                    <Plus size={22} /><span>放入来源卡</span><small>槽位 0{index + 1}</small>
                  </button>
                )
              })}
            </div>
          </section>

          <div className="fuse-layout">
            <section className="fuse-control-panel">
              <div className="section-head">
                <div><span className="eyebrow">02 · OUTPUT</span><h2>结果卡设定</h2></div>
                <span className="muted">生成后仍停留在这里</span>
              </div>
              <div className="stack">
                <label>新卡名称<input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} /></label>
                <label>模型（可选）<input value={model} onChange={(event) => setModel(event.target.value)} /></label>
              </div>
              {selectedIds.length >= 2 ? (
                <div className="fuse-preview">
                  <span className="eyebrow">LOCAL LEVEL PREVIEW</span>
                  {preview ? (
                    <div className="dimension-bars">
                      {DIMENSION_KEYS.map((key) => (
                        <div className="dimension-bar" key={key}>
                          <span>{DIMENSION_LABELS[key]}</span>
                          <div className="dimension-track" aria-hidden="true">
                            <i style={{ width: String(preview[key]) + '%' }} />
                          </div>
                          <strong>{preview[key]}</strong>
                        </div>
                      ))}
                    </div>
                  ) : <p className="muted">正在读取来源卡等级…</p>}
                </div>
              ) : (
                <div className="fusion-empty">完成至少两个来源卡片的配比后，这里会显示本地等级预览。</div>
              )}
              {resultCard ? (
                <div className="fusion-result">
                  <span className="eyebrow"><Check size={14} />RESULT IN DECK</span>
                  <StyleCardTile
                    card={{ id: resultCard.id, name: resultCard.name, kind: resultCard.kind, current_version: resultCard.version, updated_at: new Date().toISOString() }}
                    active
                    onClick={() => navigate('/p/' + projectId + '/lab/' + resultCard.id)}
                    actionLabel="刚刚生成"
                  />
                  <div className="result-actions">
                    <Link className="btn" to={`/p/${projectId}/lab/${resultCard.id}`}>进入实验室<ArrowRight size={16} /></Link>
                    <Link className="btn secondary" to={`/p/${projectId}/cards`}>查看牌库</Link>
                  </div>
                </div>
              ) : null}
            </section>

            <section className="fuse-control-panel dimension-panel">
              <div className="section-head">
                <div><span className="eyebrow">03 · BLEND</span><h2>九维配比</h2></div>
                <div className="blend-presets" role="group" aria-label="配比预设">
                  <button className={'btn secondary sm' + (mode === 'balanced' ? ' on' : '')} type="button" onClick={evenAll} disabled={selectedIds.length < 2}>均分</button>
                  {selectedCards.map((card) => (
                    <button
                      className={'btn secondary sm' + (mode === 'dominant' && dominantCardId === card.id ? ' on' : '')}
                      type="button"
                      key={card.id}
                      onClick={() => favorCard(card.id)}
                      disabled={selectedIds.length < 2}
                    >
                      「{card.name}」70%
                    </button>
                  ))}
                  <span className={'preset-tag' + (mode === 'custom' ? ' on' : '')}>自定义</span>
                </div>
              </div>
              <div className="dim-pills" aria-label="选择要调节的维度">
                {DIMENSION_KEYS.map((key) => <button key={key} type="button" className={`dim-pill${focusDim === key ? ' on' : ''}`} onClick={() => setFocusDim(key)}>{DIMENSION_LABELS[key]}<em>{sums[key] ?? 0}</em></button>)}
              </div>
              <div className="blend-focus">
                <div className="blend-focus-head"><div><span className="eyebrow">CURRENT DIMENSION</span><h3>{DIMENSION_LABELS[focusDim]}</h3></div><strong className={sums[focusDim] === 100 ? 'ok' : 'warn'}>{sums[focusDim] ?? 0}<small>/100</small></strong></div>
                <div className="contribution-list">
                  {selectedCards.length ? selectedCards.map((card) => {
                    const draft = drafts[card.id] ?? emptyDraft(card.current_version)
                    const enabled = !!draft.dims[focusDim]
                    const value = draft.weights[focusDim] ?? 0
                    return <div className={`contribution-row${enabled ? '' : ' muted-row'}`} key={card.id}>
                      <div className="contribution-card"><span className={`mini-glyph kind-${card.kind}`}>{card.kind === 'fused' ? 'F' : card.kind === 'manual' ? 'M' : 'E'}</span><strong>{card.name}</strong></div>
                      <label className="inline-toggle"><input type="checkbox" checked={enabled} onChange={(event) => toggleDim(card.id, focusDim, event.target.checked)} />参与</label>
                      <input className="contribution-range" type="range" min={0} max={100} disabled={!enabled || focusActiveIds.length <= 1} value={enabled ? value : 0} onChange={(event) => setWeight(card.id, focusDim, Number(event.target.value))} />
                      <output>{enabled ? value : 0}%</output>
                    </div>
                  }) : <p className="muted">先从上方融合槽放入来源卡片。</p>}
                </div>
              </div>
            </section>
          </div>

          <div className="fuse-command-bar">
            <div>{error ? <p className="error">{error}</p> : <span className="muted">{selectedIds.length < 2 ? '至少选择 2 张卡片' : weightsOk ? '配比完整，可生成结果' : '权重尚未合计 100'}</span>}</div>
            <button className="btn" type="submit" disabled={busy || jobActive || selectedIds.length < 2 || !weightsOk}>{busy ? '提交中…' : jobActive ? '融合进行中…' : '生成融合卡'}</button>
          </div>
          {jobId ? <JobProgress jobId={jobId} projectId={projectId} autoNavigate={false} onStatus={onJobStatus} onSucceeded={(result) => void onFuseSucceeded(result)} /> : null}
        </form>
      )}
      <CardDeckDrawer
        open={deckOpen}
        projectId={projectId}
        cards={cards}
        selectedIds={selectedIds}
        onToggle={toggleCard}
        onClose={() => setDeckOpen(false)}
      />
    </div>
  )
}
