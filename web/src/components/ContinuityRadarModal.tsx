import { useState, useEffect } from 'react'
import { CheckCircle2, Loader2, RefreshCw, ShieldAlert, X } from 'lucide-react'
import { api, APIError, notify } from '../api'
import type { ContinuityAuditResponse } from '../types'

interface ContinuityRadarModalProps {
  projectId: string
  isOpen: boolean
  onClose: () => void
}

export default function ContinuityRadarModal({ projectId, isOpen, onClose }: ContinuityRadarModalProps) {
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<ContinuityAuditResponse | null>(null)
  const [error, setError] = useState('')
  const [filterSeverity, setFilterSeverity] = useState<string>('all')

  useEffect(() => {
    if (isOpen && !report) {
      void runAudit()
    }
  }, [isOpen])

  if (!isOpen) return null

  async function runAudit() {
    setError('')
    setLoading(true)
    try {
      const res = await api.continuityAudit(projectId)
      setReport(res)
      notify('伏笔与战力逻辑雷达扫描完成！', 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '逻辑雷达扫描失败')
    } finally {
      setLoading(false)
    }
  }

  const issues = report?.issues ?? []
  const filteredIssues =
    filterSeverity === 'all' ? issues : issues.filter((i) => i.severity === filterSeverity)

  const criticalCount = issues.filter((i) => i.severity === 'critical').length
  const warningCount = issues.filter((i) => i.severity === 'warning').length
  const infoCount = issues.filter((i) => i.severity === 'info').length

  const score = report?.score ?? 100
  const scoreColor = score >= 85 ? '#4ade80' : score >= 70 ? '#facc15' : '#f87171'

  return (
    <div className="modal-backdrop" onClick={onClose} style={{ zIndex: 1100 }}>
      <div
        className="modal-panel"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: '820px', width: '92%', maxHeight: '88vh', display: 'flex', flexDirection: 'column' }}
      >
        <div className="modal-head" style={{ borderBottom: '1px solid var(--border)', paddingBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '8px',
                background: 'rgba(239, 68, 68, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid rgba(239, 68, 68, 0.3)',
              }}
            >
              <ShieldAlert size={20} color="#f87171" />
            </div>
            <div>
              <h2 style={{ fontSize: '1.25rem', margin: 0, color: '#f87171' }}>伏笔与战力逻辑冲突雷达</h2>
              <span className="muted" style={{ fontSize: '0.84rem' }}>
                深度交叉比对全书章节与世界观设定，探测战力崩塌、死者复生与遗忘伏笔
              </span>
            </div>
          </div>
          <button className="btn icon-only secondary" onClick={onClose} type="button" title="关闭">
            <X size={18} />
          </button>
        </div>

        <div style={{ overflowY: 'auto', padding: '1.2rem 0', flex: 1 }}>
          {error ? <p className="error" style={{ marginBottom: '1rem' }}>{error}</p> : null}

          {loading ? (
            <div style={{ textAlign: 'center', padding: '4rem 1rem' }}>
              <Loader2 size={36} className="spin" style={{ color: 'var(--gold-hi)', marginBottom: '1rem' }} />
              <p style={{ fontSize: '1rem', color: '#fff', margin: '0 0 0.5rem' }}>AI 审计官正在深度巡检全书章节与设定集…</p>
              <span className="muted" style={{ fontSize: '0.85rem' }}>比对境界体系、人设一致性、物品归属与伏笔网络</span>
            </div>
          ) : report ? (
            <div className="stack" style={{ gap: '1.4rem' }}>
              {/* Score card */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '1.5rem',
                  padding: '1.2rem',
                  background: 'rgba(15, 20, 30, 0.85)',
                  borderRadius: '10px',
                  border: '1px solid var(--border)',
                  flexWrap: 'wrap',
                }}
              >
                <div
                  style={{
                    width: '80px',
                    height: '80px',
                    borderRadius: '50%',
                    border: `4px solid ${scoreColor}`,
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    background: 'rgba(0, 0, 0, 0.3)',
                  }}
                >
                  <span style={{ fontSize: '1.6rem', fontWeight: 900, color: scoreColor, lineHeight: 1 }}>
                    {score}
                  </span>
                  <span style={{ fontSize: '0.65rem', color: 'var(--muted)', marginTop: '2px' }}>逻辑指数</span>
                </div>

                <div style={{ flex: 1, minWidth: '240px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.4rem' }}>
                    <strong style={{ color: '#fff', fontSize: '1.05rem' }}>全书逻辑严谨度评估</strong>
                    <button
                      className="btn secondary"
                      type="button"
                      style={{ padding: '0.3rem 0.7rem', fontSize: '0.8rem' }}
                      onClick={runAudit}
                    >
                      <RefreshCw size={12} />
                      重新扫描
                    </button>
                  </div>
                  <p style={{ margin: 0, fontSize: '0.88rem', color: 'var(--muted)', lineHeight: 1.5 }}>
                    {report.overall}
                  </p>
                </div>
              </div>

              {/* Severity filter chips */}
              <div style={{ display: 'flex', gap: '0.6rem', flexWrap: 'wrap' }}>
                <button
                  type="button"
                  className={`btn ${filterSeverity === 'all' ? '' : 'secondary'}`}
                  style={{ padding: '0.35rem 0.8rem', fontSize: '0.82rem' }}
                  onClick={() => setFilterSeverity('all')}
                >
                  全部问题 ({issues.length})
                </button>
                {criticalCount > 0 ? (
                  <button
                    type="button"
                    className={`btn ${filterSeverity === 'critical' ? '' : 'secondary'}`}
                    style={{
                      padding: '0.35rem 0.8rem',
                      fontSize: '0.82rem',
                      color: filterSeverity === 'critical' ? '#fff' : '#f87171',
                      borderColor: 'rgba(239, 68, 68, 0.4)',
                    }}
                    onClick={() => setFilterSeverity('critical')}
                  >
                    🔴 致命毒点 ({criticalCount})
                  </button>
                ) : null}
                {warningCount > 0 ? (
                  <button
                    type="button"
                    className={`btn ${filterSeverity === 'warning' ? '' : 'secondary'}`}
                    style={{
                      padding: '0.35rem 0.8rem',
                      fontSize: '0.82rem',
                      color: filterSeverity === 'warning' ? '#fff' : '#facc15',
                      borderColor: 'rgba(250, 204, 21, 0.4)',
                    }}
                    onClick={() => setFilterSeverity('warning')}
                  >
                    🟡 逻辑疑点 ({warningCount})
                  </button>
                ) : null}
                {infoCount > 0 ? (
                  <button
                    type="button"
                    className={`btn ${filterSeverity === 'info' ? '' : 'secondary'}`}
                    style={{
                      padding: '0.35rem 0.8rem',
                      fontSize: '0.82rem',
                      color: filterSeverity === 'info' ? '#fff' : '#60a5fa',
                      borderColor: 'rgba(96, 165, 250, 0.4)',
                    }}
                    onClick={() => setFilterSeverity('info')}
                  >
                    🔵 细节提醒 ({infoCount})
                  </button>
                ) : null}
              </div>

              {/* Issues list */}
              <div className="stack" style={{ gap: '0.8rem' }}>
                {filteredIssues.length === 0 ? (
                  <div
                    style={{
                      padding: '2.5rem 1rem',
                      textAlign: 'center',
                      background: 'rgba(0, 0, 0, 0.2)',
                      borderRadius: '8px',
                    }}
                  >
                    <CheckCircle2 size={32} color="#4ade80" style={{ marginBottom: '0.6rem' }} />
                    <p style={{ margin: 0, color: '#fff', fontSize: '0.95rem' }}>未检测到此类逻辑冲突，剧情运行严谨稳定！</p>
                  </div>
                ) : (
                  filteredIssues.map((item, idx) => {
                    const isCrit = item.severity === 'critical'
                    const isWarn = item.severity === 'warning'
                    const borderCol = isCrit ? 'rgba(239, 68, 68, 0.35)' : isWarn ? 'rgba(250, 204, 21, 0.35)' : 'rgba(96, 165, 250, 0.35)'
                    const tagBg = isCrit ? 'rgba(239, 68, 68, 0.15)' : isWarn ? 'rgba(250, 204, 21, 0.15)' : 'rgba(96, 165, 250, 0.15)'
                    const tagCol = isCrit ? '#f87171' : isWarn ? '#facc15' : '#60a5fa'

                    return (
                      <div
                        key={idx}
                        style={{
                          padding: '0.9rem 1rem',
                          background: 'rgba(12, 16, 24, 0.9)',
                          borderRadius: '8px',
                          border: `1px solid ${borderCol}`,
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.4rem' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                            <span
                              style={{
                                fontSize: '0.72rem',
                                padding: '2px 6px',
                                borderRadius: '4px',
                                background: tagBg,
                                color: tagCol,
                                fontWeight: 700,
                              }}
                            >
                              {isCrit ? '致命毒点' : isWarn ? '逻辑疑点' : '优化提醒'}
                            </span>
                            <strong style={{ color: '#fff', fontSize: '0.95rem' }}>{item.title}</strong>
                          </div>
                          {item.location ? (
                            <span style={{ fontSize: '0.78rem', color: 'var(--muted)' }}>
                              📍 {item.location}
                            </span>
                          ) : null}
                        </div>

                        <p style={{ margin: '0 0 0.5rem', fontSize: '0.86rem', color: 'var(--text)', lineHeight: 1.5 }}>
                          {item.description}
                        </p>

                        {item.suggestion ? (
                          <div
                            style={{
                              padding: '0.4rem 0.6rem',
                              background: 'rgba(212, 175, 55, 0.08)',
                              borderRadius: '4px',
                              borderLeft: '3px solid var(--gold-hi)',
                              fontSize: '0.82rem',
                              color: 'var(--gold-hi)',
                            }}
                          >
                            💡 <strong>修正建议：</strong>{item.suggestion}
                          </div>
                        ) : null}
                      </div>
                    )
                  })
                )}
              </div>

              {/* Foreshadows list */}
              {report.foreshadows && report.foreshadows.length > 0 ? (
                <div
                  style={{
                    padding: '1rem',
                    background: 'rgba(212, 175, 55, 0.05)',
                    borderRadius: '8px',
                    border: '1px solid rgba(212, 175, 55, 0.2)',
                  }}
                >
                  <strong style={{ color: 'var(--gold-hi)', display: 'block', marginBottom: '0.5rem', fontSize: '0.92rem' }}>
                    📌 全书已埋伏笔待回收追踪 ({report.foreshadows.length})
                  </strong>
                  <ul style={{ margin: 0, paddingLeft: '1.2rem', fontSize: '0.85rem', color: 'var(--muted)' }}>
                    {report.foreshadows.map((f, i) => (
                      <li key={i} style={{ marginBottom: '0.3rem', lineHeight: 1.4 }}>
                        {f}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}
