import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import Crumb from '../components/Crumb'
import DimensionSliders from '../components/DimensionSliders'
import JobProgress from '../components/JobProgress'
import Skeleton from '../components/Skeleton'
import { confirm } from '../components/ConfirmDialog'
import { Tooltip } from '../components/Tooltip'
import { useDirtyGuard, usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import { projectName } from '../projectCache'
import {
  DIMENSION_KEYS,
  DIMENSION_LABELS,
  KIND_LABEL,
  type AuditEdit,
  type DimensionKey,
  type JobStatus,
  type StyleCard,
} from '../types'

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
  const location = useLocation()
  const navigate = useNavigate()

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
  const [auditJobId, setAuditJobId] = useState('')
  const [sampleJobId, setSampleJobId] = useState('')
  const [auditActive, setAuditActive] = useState(false)
  const [sampleActive, setSampleActive] = useState(false)
  const [showSampleForm, setShowSampleForm] = useState(false)
  const [focusDim, setFocusDim] = useState<DimensionKey>('sentence_rhythm')
  usePageTitle(card?.name ? `${card.name} · 实验室` : '实验室')

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

  const dirty = !!card && (name !== card.name || DIMENSION_KEYS.some((key) => (levels[key] ?? 0) !== (card.dimensions?.[key]?.level ?? 0)))
  useDirtyGuard(dirty)

  // Ctrl+S 保存快捷键
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        if (!dirty || busy) return
        const form = document.querySelector('.lab-workspace') as HTMLFormElement
        form?.requestSubmit()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [dirty, busy])

  // 从审计报告跳转而来：把建议水平写入滑条，等用户检查后手动保存
  const appliedRef = useRef(false)
  useEffect(() => {
    if (!card || appliedRef.current) return
    const edits = (location.state as { applyEdits?: AuditEdit[] } | null)?.applyEdits
    if (!Array.isArray(edits) || edits.length === 0) return
    appliedRef.current = true
    const keys = DIMENSION_KEYS as readonly string[]
    setLevels((prev) => {
      const next = { ...prev }
      for (const e of edits) {
        if (keys.includes(e.dimension)) next[e.dimension] = e.target_level
      }
      return next
    })
    notify('审计建议已载入，检查后保存为新版本', 'info')
    navigate(location.pathname, { replace: true, state: null })
  }, [card, location.state, location.pathname, navigate])

  async function onSelectVersion(n: number) {
    if (!idOfCard) return
    if (dirty) {
      const ok = await confirm({
        title: '切换版本会丢弃未保存的修改',
        body: '当前对名称或维度的调整还没有保存为版本。',
        confirmText: '切换',
        danger: true,
      })
      if (!ok) return
    }
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
      notify(`已保存为 v${saved.version}`, 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onExport() {
    if (!idOfCard) return
    try {
      await api.exportCard(idOfCard)
      notify('已导出 simulation_profile.json', 'success')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '导出失败', 'error')
    }
  }

  function onAuditStatus(status: JobStatus) {
    setAuditActive(status === 'queued' || status === 'running')
  }

  function onSampleStatus(status: JobStatus) {
    setSampleActive(status === 'queued' || status === 'running')
  }

  async function startAudit() {
    if (!idOfCard || auditActive) return
    setError('')
    setBusy(true)
    try {
      const res = await api.auditCard(idOfCard, model.trim())
      setAuditJobId(res.job_id)
      setAuditActive(true)
      registerJob({ jobId: res.job_id, projectId, kind: 'audit', label: `审计「${card?.name ?? ''}」`, startedAt: Date.now() })
      notify('审计任务已提交，后台生成中', 'info')
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
    if (!idOfCard || sampleActive) return
    setError('')
    setBusy(true)
    try {
      const res = await api.sampleCard(idOfCard, text, target, model.trim())
      setSampleJobId(res.job_id)
      setSampleActive(true)
      registerJob({ jobId: res.job_id, projectId, kind: 'sample', label: '试写样章', startedAt: Date.now() })
      notify('试写任务已提交，后台生成中', 'info')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '试写提交失败')
    } finally {
      setBusy(false)
    }
  }

  const premiseCount = [...premise].length
  const techniqueCount = Object.values(card?.dimensions ?? {}).reduce(
    (sum, dim) => sum + (dim?.techniques?.length ?? 0),
    0,
  )
  const prohibitionCount = card?.prohibitions?.length ?? 0

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current={card?.name ?? '实验室'} />
      <div className="page-head">
        <div>
          <p className="kicker">STUDIO</p>
          <h1>{card?.name ?? projectName(projectId)}</h1>
          <p className="sub">
            {card
              ? `正在调「${DIMENSION_LABELS[focusDim]}」· ${levels[focusDim] ?? 0} · 保存会生成新版本`
              : '九个维度可调，一次只改一维。'}
          </p>
        </div>
        <div className="status-grid">
          <div className="status-chip">
            <span>当前版本</span>
            <strong>{card ? `v${card.version}` : '—'}</strong>
          </div>
          <div className="status-chip">
            <span>技法条目</span>
            <strong>{techniqueCount}</strong>
          </div>
          <div className="status-chip">
            <span>约束</span>
            <strong>{prohibitionCount}</strong>
          </div>
        </div>
      </div>
      {error && !card ? (
        <div className="empty-state">
          <h2>卡片打不开</h2>
          <p className="error">{error}</p>
          <button className="btn secondary" type="button" onClick={() => void loadCurrent()}>重试</button>
        </div>
      ) : !card ? (
        <div className="lab-workspace">
          <Skeleton h={92} />
          <Skeleton h={360} />
          <Skeleton h={280} />
        </div>
      ) : (
        <form className="lab-workspace" onSubmit={onSave}>
          <section className="lab-identity work-zone">
            <div className="zone-heading">
              <span className="zone-index">ID</span>
              <div>
                <h2>卡片身份</h2>
                <p>{KIND_LABEL[card.kind] ?? card.kind} · 正在查看 v{card.version}</p>
              </div>
            </div>
            <div className="lab-identity-grid">
              <label>
                名称
                <input type="text" value={name} maxLength={80} onChange={(e) => setName(e.target.value)} />
              </label>
              <label>
                模型（可选）
                <input type="text" value={model} onChange={(e) => setModel(e.target.value)} />
              </label>
            </div>
          </section>

          <section className="lab-studio work-zone">
            <div className="zone-heading">
              <span className="zone-index">09</span>
              <div>
                <h2>雷达与当前维</h2>
                <p>点雷达或维名，只改当前这一维</p>
              </div>
            </div>
            <DimensionSliders
              levels={levels}
              dimensions={card.dimensions}
              onChange={setLevel}
              disabled={busy}
              focus={focusDim}
              onFocus={setFocusDim}
            />
          </section>

          <aside className="lab-side">
            <section className="work-zone">
              <div className="zone-heading">
                <span className="zone-index">V</span>
                <div>
                  <h2>版本</h2>
                  <p>切换前会确认未保存修改</p>
                </div>
              </div>
              <div className="version-chips" role="listbox" aria-label="卡片版本">
                {versions.map((n) => (
                  <button
                    key={n}
                    className={'version-chip' + (card.version === n ? ' on' : '')}
                    type="button"
                    role="option"
                    aria-selected={card.version === n}
                    onClick={() => void onSelectVersion(n)}
                  >
                    v{n}{n === maxVersion ? ' 当前' : ''}
                  </button>
                ))}
              </div>
            </section>
            <section className="work-zone">
              <div className="zone-heading">
                <span className="zone-index">!</span>
                <div>
                  <h2>约束</h2>
                  <p>抽离时记下的禁用倾向</p>
                </div>
              </div>
              {card.prohibitions?.length ? (
                <ul className="lab-constraints">
                  {card.prohibitions.map((p) => (
                    <li key={p}>{p}</li>
                  ))}
                </ul>
              ) : <p className="zone-empty">这张卡还没有约束条目。</p>}
            </section>
          </aside>

          {showSampleForm ? (
            <section className="lab-sample work-zone">
              <div className="zone-heading">
                <span className="zone-index">S</span>
                <div>
                  <h2>试写</h2>
                  <p>前提不超过 80 字，目标字数可调</p>
                </div>
              </div>
              <label>
                试写前提（{premiseCount}/80）
                <textarea
                  value={premise}
                  maxLength={80}
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
              <button className="btn" type="button" onClick={() => void startSample()} disabled={busy || sampleActive}>
                {sampleActive ? '试写中…' : '开始试写'}
              </button>
            </section>
          ) : null}

          <div className="lab-jobs">
            {auditJobId ? <JobProgress jobId={auditJobId} projectId={projectId} onStatus={onAuditStatus} /> : null}
            {sampleJobId ? <JobProgress jobId={sampleJobId} projectId={projectId} onStatus={onSampleStatus} /> : null}
            {error ? <p className="error">{error}</p> : null}
          </div>

          <div className="lab-sticky-bar">
            <label className="inline-toggle">
              <input
                type="checkbox"
                checked={rewrite}
                onChange={(e) => setRewrite(e.target.checked)}
              />
              保存时重写已改维度的摘要与技法
            </label>
            <div className="lab-sticky-actions">
              <Tooltip label="保存修改为新版本" shortcut="Ctrl+S">
                <button className="btn" type="submit" disabled={busy}>
                  {busy ? '处理中…' : dirty ? '保存未提交的修改' : '保存为新版本'}
                </button>
              </Tooltip>
              <Tooltip label="导出为 JSON">
                <button className="btn secondary" type="button" onClick={() => void onExport()}>
                  导出
                </button>
              </Tooltip>
              <Tooltip label="分析卡片并给出调整建议">
                <button
                  className="btn secondary"
                  type="button"
                  onClick={() => void startAudit()}
                  disabled={busy || auditActive}
                >
                  {auditActive ? '审计中…' : '审计'}
                </button>
              </Tooltip>
              <Tooltip label="用卡片风格试写一段文字">
                <button
                  className="btn secondary"
                  type="button"
                  onClick={() => setShowSampleForm((v) => !v)}
                  disabled={sampleActive}
                >
                  试写
                </button>
              </Tooltip>
              <Tooltip label="用风格卡写正文章节">
                <Link className="btn secondary" to={`/p/${projectId}/write?card=${idOfCard}`}>
                  写正章
                </Link>
              </Tooltip>
            </div>
          </div>
        </form>
      )}
    </div>
  )
}
