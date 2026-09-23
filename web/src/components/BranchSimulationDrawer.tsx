import { useEffect, useState } from 'react'
import { GitFork, Loader2, Sparkles, Wand2, X } from 'lucide-react'
import { api, errMessage, notify } from '../api'
import JobProgress from './JobProgress'
import { registerJob, resultData, timeAgo } from '../jobs'
import type { BranchSimulateResponse, Job, PlotBranch } from '../types'

interface BranchSimulationDrawerProps {
  chapterId: string
  projectId: string
  currentText: string
  isOpen: boolean
  onClose: () => void
  onApplyContinuation: (text: string) => void
}

export default function BranchSimulationDrawer({
  chapterId,
  projectId,
  currentText,
  isOpen,
  onClose,
  onApplyContinuation,
}: BranchSimulationDrawerProps) {
  const [loading, setLoading] = useState(false)
  const [continuing, setContinuing] = useState(false)
  const [simulateJobId, setSimulateJobId] = useState('')
  const [continueJobId, setContinueJobId] = useState('')
  const [data, setData] = useState<BranchSimulateResponse | null>(null)
  const [error, setError] = useState('')
  const [selectedBranch, setSelectedBranch] = useState<PlotBranch | null>(null)
  const [customPrompt, setCustomPrompt] = useState('')
  const [savedAt, setSavedAt] = useState('')

  // The drawer stays mounted while the chapter page moves between chapters,
  // so its results must be dropped when the chapter changes -- otherwise one
  // chapter's branches would be shown on the next.
  useEffect(() => {
    setData(null)
    setSelectedBranch(null)
    setSavedAt('')
    setError('')
  }, [chapterId])

  // Show this chapter's last simulation rather than asking for a new, paid one.
  useEffect(() => {
    if (!isOpen || data) return
    let live = true
    api
      .chapterStudioLatest<BranchSimulateResponse>(chapterId)
      .then(({ latest }) => {
        if (!live || !latest) return
        setData(latest.result)
        setSelectedBranch(latest.result.branches[0] ?? null)
        setSavedAt(latest.finished_at)
      })
      .catch(() => {
        // Nothing saved to show; 开始推演 works as before.
      })
    return () => {
      live = false
    }
  }, [isOpen, chapterId, data])

  if (!isOpen) return null

  async function handleSimulate() {
    setError('')
    setLoading(true)
    try {
      const { job_id } = await api.branchSimulate(chapterId, currentText)
      registerJob({ jobId: job_id, projectId, kind: 'branch_simulate', label: '灵感推演', startedAt: Date.now() })
      setSimulateJobId(job_id)
    } catch (err) {
      setError(errMessage(err, '推演失败'))
      setLoading(false)
    }
  }

  function onSimulateDone(job: Job) {
    setLoading(false)
    setSimulateJobId('')
    if (job.status === 'succeeded') {
      const res = resultData<BranchSimulateResponse>(job.result)
      if (res) {
        setData(res)
        if (res.branches.length > 0) setSelectedBranch(res.branches[0])
        setSavedAt(new Date().toISOString())
        notify('AI 剧情推演已生成 3 种破局走向！', 'success')
        return
      }
      setError('推演完成，但结果无法解析')
      return
    }
    setError(job.error || '推演失败')
  }

  async function handleContinue(instruction: string) {
    setError('')
    setContinuing(true)
    try {
      const { job_id } = await api.continueChapter(chapterId, {
        current_text: currentText,
        instruction,
        target_runes: 600,
      })
      registerJob({ jobId: job_id, projectId, kind: 'chapter_continue', label: 'AI 续写', startedAt: Date.now() })
      setContinueJobId(job_id)
    } catch (err) {
      setError(errMessage(err, '续写失败'))
      setContinuing(false)
    }
  }

  function onContinueDone(job: Job) {
    setContinuing(false)
    setContinueJobId('')
    if (job.status === 'succeeded') {
      const res = resultData<{ continued_text: string }>(job.result)
      if (res?.continued_text) {
        onApplyContinuation(res.continued_text)
        notify('已将 AI 续写段落无缝写入正文！', 'success')
        onClose()
        return
      }
      setError('续写完成，但没有返回正文')
      return
    }
    setError(job.error || '续写失败')
  }

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        right: 0,
        bottom: 0,
        width: '460px',
        maxWidth: '92vw',
        background: 'rgba(10, 14, 22, 0.98)',
        backdropFilter: 'blur(16px)',
        borderLeft: '1px solid var(--line)',
        boxShadow: '-12px 0 40px rgba(0, 0, 0, 0.7)',
        zIndex: 100,
        display: 'flex',
        flexDirection: 'column',
        animation: 'slideInRight 0.25s ease-out',
      }}
    >
      {/* Head */}
      <div
        style={{
          padding: '1.2rem',
          borderBottom: '1px solid var(--line)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
          <div
            style={{
              width: '32px',
              height: '32px',
              borderRadius: '6px',
              background: 'rgba(212, 175, 55, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <GitFork size={18} color="var(--gold-hi)" />
          </div>
          <div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', color: 'var(--gold-hi)' }}>卡文推演 · 多分支模拟</h3>
            <span className="muted" style={{ fontSize: '0.78rem' }}>AI 预判破局走向并支持一键顺滑续写</span>
          </div>
        </div>
        <button className="btn icon-only secondary" onClick={onClose} type="button">
          <X size={16} />
        </button>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '1.2rem' }}>
        {error ? <p className="error" style={{ marginBottom: '1rem' }}>{error}</p> : null}

        {simulateJobId ? (
          <div style={{ marginBottom: '1rem' }}>
            <JobProgress
              jobId={simulateJobId}
              projectId={projectId}
              autoNavigate={false}
              onSucceeded={(_r, job) => onSimulateDone(job)}
              onStatus={(status) => {
                if (status === 'failed' || status === 'canceled') {
                  onSimulateDone({ id: simulateJobId, status } as Job)
                }
              }}
            />
          </div>
        ) : null}

        {continueJobId ? (
          <div style={{ marginBottom: '1rem' }}>
            <JobProgress
              jobId={continueJobId}
              projectId={projectId}
              autoNavigate={false}
              onSucceeded={(_r, job) => onContinueDone(job)}
              onStatus={(status) => {
                if (status === 'failed' || status === 'canceled') {
                  onContinueDone({ id: continueJobId, status } as Job)
                }
              }}
            />
          </div>
        ) : null}

        {!data ? (
          <div style={{ textAlign: 'center', padding: '3rem 1rem' }}>
            <Sparkles size={36} color="var(--gold-hi)" style={{ marginBottom: '1rem' }} />
            <h4 style={{ margin: '0 0 0.5rem', color: '#fff' }}>陷入卡文或想探索更多剧情可能？</h4>
            <p className="muted" style={{ fontSize: '0.86rem', lineHeight: 1.5, marginBottom: '1.5rem' }}>
              AI 架构师将结合当前章节前文与大纲，为你推演【突围激战】、【奇谋反转】与【第三方变局】三种截然不同的情节走向。
            </p>
            <button
              className="btn"
              type="button"
              onClick={handleSimulate}
              disabled={loading}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}
            >
              {loading ? <Loader2 size={16} className="spin" /> : <Wand2 size={16} />}
              {loading ? '正在推演多种破局分支…' : '立即推演 3 种情节分支'}
            </button>
          </div>
        ) : (
          <div className="stack" style={{ gap: '1.2rem' }}>
            {savedAt ? (
              <p className="muted" style={{ margin: 0, fontSize: '0.78rem' }}>推演于 {timeAgo(savedAt)}</p>
            ) : null}
            {data.current_analysis ? (
              <div
                style={{
                  padding: '0.8rem',
                  background: 'rgba(212, 175, 55, 0.08)',
                  borderRadius: '6px',
                  border: '1px solid rgba(212, 175, 55, 0.2)',
                  fontSize: '0.84rem',
                  lineHeight: 1.5,
                  color: 'var(--ink)',
                }}
              >
                💡 <strong>当前困局简析：</strong>{data.current_analysis}
              </div>
            ) : null}

            <div className="stack" style={{ gap: '0.8rem' }}>
              {data.branches.map((b) => {
                const isSelected = selectedBranch?.id === b.id
                return (
                  <div
                    key={b.id}
                    onClick={() => setSelectedBranch(b)}
                    style={{
                      padding: '1rem',
                      borderRadius: '8px',
                      background: isSelected ? 'rgba(212, 175, 55, 0.12)' : 'rgba(15, 20, 30, 0.7)',
                      border: isSelected
                        ? '1.5px solid var(--gold-hi)'
                        : '1px solid rgba(255, 255, 255, 0.08)',
                      cursor: 'pointer',
                      transition: 'all 0.2s ease',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.4rem' }}>
                      <span
                        style={{
                          fontSize: '0.74rem',
                          padding: '2px 6px',
                          borderRadius: '4px',
                          background: isSelected ? 'rgba(212, 175, 55, 0.3)' : 'rgba(255, 255, 255, 0.1)',
                          color: isSelected ? 'var(--gold-hi)' : '#ccc',
                          fontWeight: 700,
                        }}
                      >
                        分支 {b.id} · {b.type}
                      </span>
                      <strong style={{ fontSize: '0.92rem', color: '#fff' }}>{b.title}</strong>
                    </div>

                    <p style={{ margin: '0 0 0.5rem', fontSize: '0.85rem', color: 'var(--ink-soft)', lineHeight: 1.5 }}>
                      {b.direction}
                    </p>

                    {b.plot_points && b.plot_points.length > 0 ? (
                      <ul
                        style={{
                          margin: '0 0 0.5rem',
                          paddingLeft: '1.1rem',
                          fontSize: '0.82rem',
                          color: 'var(--ink-soft)',
                          lineHeight: 1.6,
                        }}
                      >
                        {b.plot_points.map((point, idx) => (
                          <li key={idx}>{point}</li>
                        ))}
                      </ul>
                    ) : null}

                    {b.sample_opening ? (
                      <div
                        style={{
                          padding: '0.5rem 0.7rem',
                          background: 'rgba(0, 0, 0, 0.3)',
                          borderRadius: '4px',
                          borderLeft: '2px solid var(--gold-hi)',
                          fontSize: '0.8rem',
                          color: '#e2e8f0',
                          lineHeight: 1.5,
                          fontStyle: 'italic',
                        }}
                      >
                        “{b.sample_opening}”
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>

            {selectedBranch ? (
              <div
                style={{
                  padding: '1rem',
                  background: 'rgba(0, 0, 0, 0.4)',
                  borderRadius: '8px',
                  border: '1px solid var(--line)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.6rem' }}>
                  <strong style={{ fontSize: '0.9rem', color: 'var(--gold-hi)' }}>
                    ✍️ 采纳「{selectedBranch.title}」并续写
                  </strong>
                </div>

                <textarea
                  rows={2}
                  value={customPrompt}
                  placeholder="（可选）补充续写细节要求，如：主角必须保留三成灵力，在招式碰撞时留下暗劲..."
                  onChange={(e) => setCustomPrompt(e.target.value)}
                  style={{ fontSize: '0.82rem', marginBottom: '0.8rem' }}
                />

                <button
                  className="btn"
                  type="button"
                  onClick={() =>
                    handleContinue(
                      `按照分支【${selectedBranch.title}】的剧情走向向下推进：${selectedBranch.direction}。${customPrompt ? '补充要求：' + customPrompt : ''}`
                    )
                  }
                  disabled={continuing}
                  style={{ width: '100%', justifyContent: 'center' }}
                >
                  {continuing ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}
                  {continuing ? 'AI 正在融汇文风续写正文中…' : '采纳此分支并立即续写 (约600字)'}
                </button>
              </div>
            ) : null}
          </div>
        )}
      </div>
    </div>
  )
}
