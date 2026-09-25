import { useEffect, useState } from 'react'
import { ArrowDownToLine, Loader2, Sparkles, Wand2, X } from 'lucide-react'
import { api, errMessage, notify } from '../api'
import JobProgress from './JobProgress'
import { registerJob, resultData, timeAgo } from '../jobs'
import type { CardSummary, Job, OutlineResponse, OutlineChapterItem } from '../types'

interface OutlinePlannerModalProps {
  projectId: string
  cards: CardSummary[]
  isOpen: boolean
  onClose: () => void
  onImportSuccess: () => void
}

const GENRES = [
  '玄幻修真 · 凡人流/杀伐果断',
  '都市异能 · 隐秘组织/灵气复苏',
  '末世科幻 · 智械危机/废土求生',
  '奇幻史诗 · 领主争霸/群像权谋',
  '悬疑惊悚 · 诡异复苏/规则怪谈',
  '轻小说/穿越 · 系统养成/反套路',
]

export default function OutlinePlannerModal({
  projectId,
  cards,
  isOpen,
  onClose,
  onImportSuccess,
}: OutlinePlannerModalProps) {
  const [premise, setPremise] = useState('')
  const [genre, setGenre] = useState(GENRES[0])
  const [targetChapters, setTargetChapters] = useState(15)
  const [volumeCount, setVolumeCount] = useState(2)
  const [cardId, setCardId] = useState('')
  const [loading, setLoading] = useState(false)
  const [jobId, setJobId] = useState('')
  // The last generated outline, offered -- not auto-loaded -- so reopening the
  // planner neither loses a paid generation nor clobbers a premise being typed.
  const [saved, setSaved] = useState<{ result: OutlineResponse; at: string } | null>(null)

  useEffect(() => {
    if (!isOpen) return
    let live = true
    api
      .projectStudioLatest<OutlineResponse>(projectId, 'outline_generate')
      .then(({ latest }) => {
        if (live && latest) setSaved({ result: latest.result, at: latest.finished_at })
      })
      .catch(() => {
        // Nothing to offer; the form works as before.
      })
    return () => {
      live = false
    }
  }, [isOpen, projectId])
  const [importing, setImporting] = useState(false)
  const [replaceExisting, setReplaceExisting] = useState(false)
  const [outline, setOutline] = useState<OutlineResponse | null>(null)
  const [error, setError] = useState('')

  if (!isOpen) return null

  async function handleGenerate() {
    if (!premise.trim()) {
      setError('请输入小说核心故事梗概')
      return
    }
    setError('')
    setLoading(true)
    try {
      const { job_id } = await api.generateOutline(projectId, {
        premise: premise.trim(),
        genre,
        target_chapters: targetChapters,
        volume_count: volumeCount,
        card_id: cardId || undefined,
      })
      // Registered so the dock keeps it alive across a refresh or page change.
      registerJob({ jobId: job_id, projectId, kind: 'outline_generate', label: '智能大纲规划', startedAt: Date.now() })
      setJobId(job_id)
    } catch (err) {
      setError(errMessage(err, '生成大纲失败'))
      setLoading(false)
    }
  }

  function onOutlineDone(job: Job) {
    setLoading(false)
    setJobId('')
    if (job.status === 'succeeded') {
      const data = resultData<OutlineResponse>(job.result)
      if (data) {
        setOutline(data)
        notify('AI 已完成全书分卷与章节细纲架构！', 'success')
        return
      }
      setError('大纲已生成，但结果无法解析')
      return
    }
    setError(job.error || '生成大纲失败')
  }

  async function handleImport() {
    if (!outline || !outline.volumes.length) return
    const allChapters: OutlineChapterItem[] = []
    for (const v of outline.volumes) {
      if (v.chapters && v.chapters.length) {
        allChapters.push(...v.chapters)
      }
    }
    if (!allChapters.length) {
      setError('大纲中暂无章节可导入')
      return
    }

    setImporting(true)
    try {
      const res = await api.importOutline(projectId, {
        chapters: allChapters,
        // Keep the volumes: they become the 卷 of the book's structure tree.
        volumes: outline.volumes.map((v) => ({
          title: v.volume_title,
          brief: v.volume_brief,
          chapter_count: v.chapters?.length ?? 0,
        })),
        replace_existing: replaceExisting,
        card_id: cardId || undefined,
      })
      notify(
        res.protected_count > 0
          ? `已导入 ${res.inserted_count} 个章节，并保留了 ${res.protected_count} 个已写正文的章节。`
          : `已成功批量导入 ${res.inserted_count} 个章节至目录！`,
        'success',
      )
      onImportSuccess()
      onClose()
    } catch (err) {
      setError(errMessage(err, '导入章节失败'))
    } finally {
      setImporting(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal-panel"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: '850px', width: '92%', maxHeight: '88vh', display: 'flex', flexDirection: 'column' }}
      >
        <div className="modal-head" style={{ borderBottom: '1px solid var(--line)', paddingBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '8px',
                background: 'rgba(212, 175, 55, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid rgba(212, 175, 55, 0.3)',
              }}
            >
              <Wand2 size={20} color="var(--gold-hi)" />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', margin: 0, color: 'var(--gold-hi)' }}>AI 全书大纲与分卷规划器</h2>
              <span className="muted" style={{ fontSize: '0.84rem' }}>
                输入核心梗概与题材，智能构筑高潮起伏的起承转合分卷细纲并一键入库
              </span>
            </div>
          </div>
          <button className="btn icon-only secondary" onClick={onClose} type="button" title="关闭">
            <X size={18} />
          </button>
        </div>

        <div style={{ overflowY: 'auto', padding: '1.2rem 0', flex: 1 }}>
          {error ? <p className="error" style={{ marginBottom: '1rem' }}>{error}</p> : null}

          {!outline ? (
            <div className="stack" style={{ gap: '1.2rem' }}>
              {saved ? (
                <div className="saved-result-banner">
                  <span>
                    上次生成的大纲 · {timeAgo(saved.at)}（{saved.result.volumes.reduce((n, v) => n + v.chapters.length, 0)} 章）
                  </span>
                  <button className="btn secondary sm" type="button" onClick={() => setOutline(saved.result)}>
                    载入
                  </button>
                </div>
              ) : null}
              <label>
                核心故事梗概 / 金手指 / 核心冲突目标
                <textarea
                  rows={4}
                  value={premise}
                  placeholder="例如：主角韩立本是山村穷小子，意外在杂役院泥土中挖出一只神秘铜绿小瓶，发现能通过吸收月华催熟灵药。面对门派大劫与魔道逼近，他隐忍低调，凭借小瓶灵液暗中筑基，步步为营踏上修真大道..."
                  onChange={(e) => setPremise(e.target.value)}
                  disabled={loading}
                />
              </label>

              <div className="row" style={{ flexWrap: 'wrap', gap: '1rem' }}>
                <label style={{ flex: '2 1 240px' }}>
                  题材类型风格
                  <select value={genre} onChange={(e) => setGenre(e.target.value)} disabled={loading}>
                    {GENRES.map((g) => (
                      <option key={g} value={g}>
                        {g}
                      </option>
                    ))}
                  </select>
                </label>

                <label style={{ flex: '1 1 120px' }}>
                  规划总章节数
                  <select
                    value={targetChapters}
                    onChange={(e) => setTargetChapters(Number(e.target.value))}
                    disabled={loading}
                  >
                    <option value={10}>10 章 (短篇精炼)</option>
                    <option value={15}>15 章 (推荐起步)</option>
                    <option value={20}>20 章 (标准首卷)</option>
                    <option value={30}>30 章 (中篇宏大)</option>
                    <option value={50}>50 章 (长篇史诗)</option>
                  </select>
                </label>

                <label style={{ flex: '1 1 120px' }}>
                  分卷规划数
                  <select
                    value={volumeCount}
                    onChange={(e) => setVolumeCount(Number(e.target.value))}
                    disabled={loading}
                  >
                    <option value={1}>1 卷</option>
                    <option value={2}>2 卷</option>
                    <option value={3}>3 卷</option>
                    <option value={4}>4 卷</option>
                  </select>
                </label>
              </div>

              {cards.length > 0 ? (
                <label>
                  默认绑定风格卡 (可选)
                  <select value={cardId} onChange={(e) => setCardId(e.target.value)} disabled={loading}>
                    <option value="">（可选）自动应用选定风格卡</option>
                    {cards.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name} · v{c.current_version} ({c.kind})
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}

              <button
                className="btn"
                type="button"
                onClick={handleGenerate}
                disabled={loading || !premise.trim()}
                style={{ alignSelf: 'flex-start', padding: '0.75rem 1.8rem' }}
              >
                {loading ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}
                {loading ? 'AI 架构师正在推演全书大纲…' : '开始智能推演大纲'}
              </button>
              {jobId ? (
                <JobProgress
                  jobId={jobId}
                  projectId={projectId}
                  autoNavigate={false}
                  onSucceeded={(_r, job) => onOutlineDone(job)}
                  onStatus={(status) => {
                    if (status === 'failed' || status === 'canceled') {
                      onOutlineDone({ id: jobId, status } as Job)
                    }
                  }}
                />
              ) : null}
            </div>
          ) : (
            <div className="stack" style={{ gap: '1.5rem' }}>
              <div
                style={{
                  background: 'rgba(212, 175, 55, 0.08)',
                  padding: '1rem 1.2rem',
                  borderRadius: '8px',
                  border: '1px solid rgba(212, 175, 55, 0.2)',
                }}
              >
                <strong style={{ color: 'var(--gold-hi)', display: 'block', marginBottom: '0.4rem' }}>
                  📖 全书总纲提要
                </strong>
                <p style={{ margin: 0, fontSize: '0.92rem', lineHeight: 1.6 }}>{outline.synopsis}</p>
              </div>

              <div className="stack" style={{ gap: '1.2rem' }}>
                {outline.volumes.map((v, vIdx) => (
                  <div
                    key={vIdx}
                    className="panel"
                    style={{ background: 'rgba(15, 20, 30, 0.8)', borderColor: 'rgba(255, 255, 255, 0.08)' }}
                  >
                    <div style={{ borderBottom: '1px solid var(--line)', paddingBottom: '0.6rem', marginBottom: '0.8rem' }}>
                      <h3 style={{ margin: '0 0 0.2rem', color: '#fff', fontSize: '1.05rem' }}>
                        {v.volume_title}
                      </h3>
                      <span className="muted" style={{ fontSize: '0.85rem' }}>
                        {v.volume_brief}
                      </span>
                    </div>

                    <div className="stack" style={{ gap: '0.6rem' }}>
                      {v.chapters.map((c, cIdx) => (
                        <div
                          key={cIdx}
                          style={{
                            padding: '0.6rem 0.8rem',
                            background: 'rgba(0, 0, 0, 0.25)',
                            borderRadius: '6px',
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '0.3rem',
                            border: '1px solid rgba(255, 255, 255, 0.04)',
                          }}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                            <strong style={{ fontSize: '0.92rem', color: 'var(--gold-hi)' }}>{c.title}</strong>
                            {c.hook ? (
                              <span
                                style={{
                                  fontSize: '0.75rem',
                                  padding: '2px 6px',
                                  background: 'rgba(224, 104, 79, 0.2)',
                                  color: '#f08a72',
                                  borderRadius: '4px',
                                  border: '1px solid rgba(224, 104, 79, 0.3)',
                                }}
                              >
                                伏笔: {c.hook}
                              </span>
                            ) : null}
                          </div>
                          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--ink-soft)', lineHeight: 1.5 }}>
                            {c.brief}
                          </p>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>

              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  flexWrap: 'wrap',
                  gap: '1rem',
                  paddingTop: '1rem',
                  borderTop: '1px solid var(--line)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={replaceExisting}
                      onChange={(e) => setReplaceExisting(e.target.checked)}
                    />
                    <span style={{ fontSize: '0.85rem' }}>替换现有章节目录（已写正文的章节会保留）</span>
                  </label>
                  <button
                    className="btn secondary"
                    type="button"
                    onClick={() => setOutline(null)}
                    disabled={importing}
                  >
                    重新调整要求
                  </button>
                </div>

                <button
                  className="btn"
                  type="button"
                  onClick={handleImport}
                  disabled={importing}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}
                >
                  {importing ? <Loader2 size={16} className="spin" /> : <ArrowDownToLine size={16} />}
                  {importing ? '正在写入章节库…' : '一键批量导入目录'}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
