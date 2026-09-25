import { FormEvent, useEffect, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import {
  BookOpenText,
  BookPlus,
  ChevronDown,
  ChevronRight,
  Download,
  FileText,
  Pencil,
  Plus,
  ShieldAlert,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react'
import { APIError, api, errMessage, notify } from '../api'
import { confirm } from '../components/ConfirmDialog'
import ContinuityRadarModal from '../components/ContinuityRadarModal'
import Crumb from '../components/Crumb'
import JobProgress from '../components/JobProgress'
import OutlinePlannerModal from '../components/OutlinePlannerModal'
import Skeleton from '../components/Skeleton'
import VolumeDialog from '../components/VolumeDialog'
import { usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import { rememberProject } from '../projectCache'
import type { CardSummary, ChapterSummary, JobStatus, Structure, StructureChapter, Volume } from '../types'

// Scenes listed under a chapter row before "共 N 个场景".
const SCENES_SHOWN = 6

const STATUS_LABEL: Record<string, string> = {
  draft: '未写',
  writing: '生成中',
  written: '已成篇',
  failed: '生成失败',
}

export default function Write() {
  const { id } = useParams()
  const projectId = id ?? ''
  const [params] = useSearchParams()
  usePageTitle('章节目录')

  const [chapters, setChapters] = useState<ChapterSummary[] | null>(null)
  // The volume -> chapter -> scene tree; null falls back to a flat list.
  const [structure, setStructure] = useState<Structure | null>(null)
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [volumeDialog, setVolumeDialog] = useState<{ volume?: Volume; startSeq?: number } | null>(null)
  const [cards, setCards] = useState<CardSummary[]>([])
  const [cardId, setCardId] = useState(params.get('card') ?? '')
  const [title, setTitle] = useState('')
  const [brief, setBrief] = useState('')
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState('')
  const [jobActive, setJobActive] = useState(false)
  const [writingId, setWritingId] = useState('')
  const [outlineOpen, setOutlineOpen] = useState(false)
  const [radarOpen, setRadarOpen] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const exportRef = useRef<HTMLDivElement>(null)

  function onJobStatus(status: JobStatus) {
    setJobActive(status === 'queued' || status === 'running')
    if (status === 'succeeded' || status === 'failed' || status === 'canceled') {
      void load()
    }
  }

  async function load() {
    if (!projectId) return
    try {
      const [ch, c, p, st] = await Promise.all([
        api.listChapters(projectId),
        api.listCards(projectId),
        api.getProject(projectId).catch(() => null),
        api.projectStructure(projectId).catch(() => null),
      ])
      setChapters(ch.chapters ?? [])
      setStructure(st)
      setCards(c.cards ?? [])
      if (p) rememberProject(projectId, p.name)
      setError('')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载失败')
      setChapters([])
    }
  }

  useEffect(() => {
    if (!exportOpen) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setExportOpen(false)
    }
    function onPointerDown(e: PointerEvent) {
      if (exportRef.current && !exportRef.current.contains(e.target as Node)) {
        setExportOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointerDown)
    }
  }, [exportOpen])

  useEffect(() => {
    void load()
  }, [projectId])

  useEffect(() => {
    const preset = params.get('card')
    if (preset) setCardId(preset)
  }, [params])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setFormError('')
    setBusy(true)
    try {
      await api.createChapter(projectId, title.trim(), brief.trim(), cardId)
      setTitle('')
      setBrief('')
      await load()
      notify('已加入章程目录', 'success')
    } catch (err) {
      setFormError(err instanceof APIError ? err.message : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  async function onWrite(ch: ChapterSummary) {
    if (!ch.card_id && !cardId) {
      setError('请先选择一张风格卡绑定')
      return
    }
    if (ch.rune_count > 0) {
      const ok = await confirm({
        title: `重写第 ${ch.seq} 章「${ch.title}」？`,
        body: '现有正文会被新生成的覆盖，章节摘要也会重新生成。',
        confirmText: '确认重写',
        danger: true,
      })
      if (!ok) return
    }
    if (!ch.card_id && cardId) {
      try {
        await api.patchChapter(ch.id, { card_id: cardId })
      } catch (err) {
        notify(err instanceof APIError ? err.message : '未能绑定风格卡', 'error')
        return
      }
    }
    setError('')
    try {
      const res = await api.writeChapter(ch.id, '', ch.target_runes || 2500, '')
      setJobId(res.job_id)
      setWritingId(ch.id)
      setJobActive(true)
      registerJob({
        jobId: res.job_id,
        projectId,
        kind: 'write',
        label: `写第 ${ch.seq} 章「${ch.title}」`,
        startedAt: Date.now(),
      })
      notify('章节撰写任务已提交，后台生成中', 'info')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '提交失败', 'error')
    }
  }

  async function onDelete(ch: ChapterSummary) {
    const ok = await confirm({
      title: `删除第 ${ch.seq} 章「${ch.title}」？`,
      body: '删除后后续章节会自动前移序号。',
      confirmText: '删除此章',
      danger: true,
    })
    if (!ok) return
    try {
      await api.deleteChapter(ch.id)
      await load()
      notify('已删除章节', 'success')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '删除失败', 'error')
    }
  }

  const list = chapters ?? []
  const written = list.filter((c) => c.status === 'written').length
  const totalRunes = list.reduce((sum, c) => sum + (c.rune_count || 0), 0)

  const byId = new Map(list.map((c) => [c.id, c]))
  const groups = structure?.volumes ?? null
  const showVolumeHeads = !!groups && groups.some((g) => g.id)
  const volumeStarts = new Set((groups ?? []).filter((g) => g.id).map((g) => g.start_seq))

  function toggleVolume(key: string) {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  async function onDeleteVolume(v: Volume) {
    const ok = await confirm({
      title: `删除分卷「${v.title}」？`,
      body: '只删除分卷本身，章节和正文都会保留，并入上一卷。',
      confirmText: '删除分卷',
      danger: true,
    })
    if (!ok) return
    try {
      await api.deleteVolume(v.id)
      void load()
    } catch (err) {
      notify(errMessage(err, '删除分卷失败'), 'error')
    }
  }

  function renderRow(ch: ChapterSummary, node?: StructureChapter) {
    return (
      <article key={ch.id} className="chapter-row">
        <Link to={`/p/${projectId}/chapter/${ch.id}`} className="chapter-main">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
            <span className="chapter-seq-badge">第 {ch.seq} 章</span>
            <h3 className="folio-title" style={{ fontSize: '1.25rem' }}>{ch.title}</h3>
            <span
              style={{
                fontSize: '0.72rem',
                fontWeight: 600,
                padding: '0.15rem 0.55rem',
                borderRadius: '20px',
                background: ch.status === 'written' ? 'rgba(45,212,191,0.15)' : ch.status === 'writing' ? 'rgba(247,203,104,0.15)' : 'rgba(255,255,255,0.06)',
                color: ch.status === 'written' ? 'var(--jade-hi)' : ch.status === 'writing' ? 'var(--gold-hi)' : 'var(--ink-soft)',
                border: `1px solid ${ch.status === 'written' ? 'rgba(45,212,191,0.3)' : ch.status === 'writing' ? 'rgba(247,203,104,0.3)' : 'rgba(255,255,255,0.1)'}`,
              }}
            >
              {STATUS_LABEL[ch.status] ?? ch.status}
            </span>
          </div>
          <p className="muted" style={{ margin: '0.4rem 0 0.2rem', lineHeight: 1.5 }}>
            {ch.brief}
          </p>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', fontSize: '0.78rem', color: 'var(--ink-faint)', fontFamily: 'var(--mono)' }}>
            {ch.rune_count ? <span>{ch.rune_count.toLocaleString()} 字</span> : <span>待撰写</span>}
            {ch.has_summary && <span style={{ color: 'var(--gold-hi)' }}>· 已生成前情摘要</span>}
            <span>· 更新于 {ch.updated_at.slice(0, 10)}</span>
          </div>
          {node && node.scenes.length > 0 ? (
            <ol className="row-scenes" aria-label="本章场景">
              {node.scenes.slice(0, SCENES_SHOWN).map((sc) => (
                <li key={sc.id} title={sc.cue === 'transition' ? `${sc.cue_text} · ${sc.runes} 字` : `${sc.runes} 字`}>
                  <span className="row-scene-no">{sc.index + 1}</span>
                  {sc.title}
                </li>
              ))}
              {node.scenes.length > SCENES_SHOWN ? <li className="row-scenes-more">共 {node.scenes.length} 个场景</li> : null}
              {node.scenes_stale ? <li className="row-scenes-more">正文已改，场景待重新切分</li> : null}
            </ol>
          ) : null}
        </Link>
        <div className="chapter-actions">
          {!volumeStarts.has(ch.seq) ? (
            <button
              className="btn ghost sm"
              type="button"
              title="从这一章开始新的一卷"
              onClick={() => setVolumeDialog({ startSeq: ch.seq })}
            >
              <BookPlus size={15} />
            </button>
          ) : null}
          <Link className="btn secondary sm" to={`/p/${projectId}/chapter/${ch.id}`}>
            <FileText size={14} />
            进入工作台
          </Link>
          <button
            className="btn sm"
            type="button"
            disabled={jobActive && writingId === ch.id}
            onClick={() => void onWrite(ch)}
          >
            <Sparkles size={14} />
            {ch.rune_count > 0 ? '重写' : '写这一章'}
          </button>
          <button
            className="btn ghost sm"
            style={{ color: 'var(--cinnabar-hi)' }}
            type="button"
            onClick={() => void onDelete(ch)}
            title="删除章节"
          >
            <Trash2 size={15} />
          </button>
        </div>
      </article>
    )
  }

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current="章节写作" />
      <div className="page-head">
        <div>
          <p className="kicker">MANUSCRIPT STUDIO</p>
          <h1>小说章节目录</h1>
          <p className="sub">
            列定大纲章程，逐章驱动 AI 依照九维风格卡与设定集行文，前情摘要自动衔接。
          </p>
        </div>
        <div className="status-grid">
          <div className="status-chip">
            <span>总章节</span>
            <strong>{list.length}</strong>
          </div>
          <div className="status-chip">
            <span>已成篇</span>
            <strong>{written}</strong>
          </div>
          <div className="status-chip">
            <span>总字数</span>
            <strong>{totalRunes.toLocaleString()}</strong>
          </div>
          <button
            className="btn"
            type="button"
            onClick={() => setOutlineOpen(true)}
            title="输入故事梗概，AI 自动生成分卷与章节细纲"
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <Wand2 size={16} />
            智能大纲规划
          </button>
          <button
            className="btn secondary"
            type="button"
            onClick={() => setRadarOpen(true)}
            disabled={list.length === 0}
            title="扫描全书已写章节与设定集，检测战力崩塌与遗忘伏笔"
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <ShieldAlert size={16} color="#f87171" />
            伏笔逻辑雷达
          </button>
          <div ref={exportRef} style={{ position: 'relative', display: 'inline-block' }}>
            <button
              className="btn secondary"
              type="button"
              onClick={() => setExportOpen((v) => !v)}
              disabled={list.length === 0}
              title="导出整部小说文稿"
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}
            >
              <Download size={16} />
              导出全书 ▾
            </button>
            {exportOpen ? (
              <div
                style={{
                  position: 'absolute',
                  top: '100%',
                  right: 0,
                  marginTop: '6px',
                  background: 'rgba(15, 20, 32, 0.98)',
                  backdropFilter: 'blur(16px)',
                  border: '1px solid var(--line)',
                  borderRadius: '8px',
                  boxShadow: '0 12px 30px rgba(0, 0, 0, 0.6)',
                  zIndex: 90,
                  minWidth: '180px',
                  padding: '0.4rem',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '0.2rem',
                }}
              >
                <button
                  className="btn ghost sm"
                  style={{ justifyContent: 'flex-start', textAlign: 'left', color: '#fff' }}
                  type="button"
                  onClick={async () => {
                    setExportOpen(false)
                    try {
                      await api.downloadNovel(projectId, 'txt')
                      notify('已导出标准 TXT 网文格式文稿！', 'success')
                    } catch (err) {
                      notify(errMessage(err, '导出失败'), 'error')
                    }
                  }}
                >
                  📄 标准 TXT 格式 (网文排版)
                </button>
                <button
                  className="btn ghost sm"
                  style={{ justifyContent: 'flex-start', textAlign: 'left', color: '#fff' }}
                  type="button"
                  onClick={async () => {
                    setExportOpen(false)
                    try {
                      await api.downloadNovel(projectId, 'md')
                      notify('已导出 Markdown 格式全书文稿！', 'success')
                    } catch (err) {
                      notify(errMessage(err, '导出失败'), 'error')
                    }
                  }}
                >
                  📝 Markdown 文稿 (带目录)
                </button>
              </div>
            ) : null}
          </div>
        </div>
      </div>

      {error ? <p className="error" style={{ marginBottom: '1.2rem' }}>{error}</p> : null}

      <section className="panel" style={{ marginBottom: '2rem' }}>
        <div className="section-head">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <Plus size={18} color="var(--gold-hi)" />
            <h2>开辟新章程</h2>
          </div>
          <span className="muted">指定标题与情节梗概，生成时自动注入对应风格</span>
        </div>
        <form className="stack" onSubmit={onCreate}>
          <div className="row" style={{ flexWrap: 'wrap' }}>
            <label style={{ flex: '1 1 240px' }}>
              绑定风格卡
              <select value={cardId} onChange={(e) => setCardId(e.target.value)}>
                <option value="">（可选）先选一张风格卡</option>
                {cards.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} · v{c.current_version} ({c.kind})
                  </option>
                ))}
              </select>
            </label>
            <label style={{ flex: '2 1 300px' }}>
              本章标题
              <input
                type="text"
                value={title}
                maxLength={40}
                placeholder="例如：第一章：残镜与古灯"
                onChange={(e) => setTitle(e.target.value)}
                required
              />
            </label>
          </div>
          <label>
            本章核心梗概（{[...brief].length}/200）
            <textarea
              value={brief}
              maxLength={200}
              rows={3}
              placeholder="说明本章核心事件推进、出场人物、冲突爆发点与章末落点…"
              onChange={(e) => setBrief(e.target.value)}
              required
            />
          </label>
          {formError ? <p className="error">{formError}</p> : null}
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <button className="btn" type="submit" disabled={busy || !title.trim() || !brief.trim()}>
              <Plus size={16} />
              {busy ? '正在开辟…' : '加入章程'}
            </button>
          </div>
        </form>
      </section>

      {chapters === null ? (
        <div className="stack">
          <Skeleton h={84} />
          <Skeleton h={84} />
          <Skeleton h={84} />
        </div>
      ) : list.length === 0 ? (
        <section className="empty-state">
          <BookOpenText size={36} color="var(--gold)" style={{ opacity: 0.8, marginBottom: '0.8rem' }} />
          <h2>暂无章节</h2>
          <p className="muted" style={{ maxWidth: '440px', margin: '0 auto' }}>
            先在上方添加第一章标题与梗概，选定风格卡即可一键开始生成正文。
          </p>
        </section>
      ) : (
        <div className="chapter-list">
          {groups ? (
            groups.map((g) => {
              const key = g.id || 'lead'
              const open = !collapsed.has(key)
              const first = g.chapters[0]?.seq
              const last = g.chapters[g.chapters.length - 1]?.seq
              return (
                <section key={key} className="volume-group" aria-label={g.id ? g.title : '未分卷'}>
                  {showVolumeHeads ? (
                    <header className={'volume-head' + (g.id ? '' : ' lead')}>
                      <button
                        type="button"
                        className="volume-toggle"
                        aria-expanded={open}
                        onClick={() => toggleVolume(key)}
                      >
                        {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        <span className="volume-title">{g.id ? g.title : '未分卷'}</span>
                      </button>
                      <span className="volume-stats">
                        {g.chapters.length
                          ? `第 ${first}${last !== first ? `–${last}` : ''} 章 · ${g.chapters.length} 章 · 已写 ${g.written} · ${g.runes.toLocaleString()} 字`
                          : '暂无章节'}
                      </span>
                      {g.id ? (
                        <span className="volume-actions">
                          <button className="icon-btn sm" type="button" title="编辑分卷" onClick={() => setVolumeDialog({ volume: g })}>
                            <Pencil size={13} />
                          </button>
                          <button className="icon-btn sm" type="button" title="删除分卷（章节保留）" onClick={() => void onDeleteVolume(g)}>
                            <Trash2 size={13} />
                          </button>
                        </span>
                      ) : null}
                      {g.brief && open ? <p className="volume-brief">{g.brief}</p> : null}
                    </header>
                  ) : null}
                  {open
                    ? g.chapters.map((node) => {
                        const ch = byId.get(node.id)
                        return ch ? renderRow(ch, node) : null
                      })
                    : null}
                </section>
              )
            })
          ) : (
            list.map((ch) => renderRow(ch))
          )}
        </div>
      )}

      {jobId ? <JobProgress jobId={jobId} projectId={projectId} onStatus={onJobStatus} /> : null}

      {/* Mounted only while open: these modals keep their result in local
          state, so an always-mounted instance reopens showing the last run
          instead of a fresh form. */}
      {outlineOpen ? (
        <OutlinePlannerModal
          projectId={projectId}
          cards={cards}
          isOpen={outlineOpen}
          onClose={() => setOutlineOpen(false)}
          onImportSuccess={() => void load()}
        />
      ) : null}

      {volumeDialog ? (
        <VolumeDialog
          projectId={projectId}
          volume={volumeDialog.volume}
          startSeq={volumeDialog.startSeq}
          onClose={() => setVolumeDialog(null)}
          onSaved={() => void load()}
        />
      ) : null}

      {radarOpen ? (
        <ContinuityRadarModal
          projectId={projectId}
          isOpen={radarOpen}
          onClose={() => setRadarOpen(false)}
        />
      ) : null}
    </div>
  )
}
