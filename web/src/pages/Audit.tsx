import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AlertTriangle, CheckCircle2, FlaskConical, Scale, Sparkles } from 'lucide-react'
import { APIError, api } from '../api'
import Crumb from '../components/Crumb'
import Skeleton from '../components/Skeleton'
import { usePageTitle } from '../hooks'
import { projectName } from '../projectCache'
import { DIMENSION_LABELS, type AuditReport, type DimensionKey } from '../types'

const PERSONA_LABELS: Record<string, string> = {
  commercial_web: '网文商业可读视角',
  literary_texture: '纯文学品质视角',
}

function dimLabel(key: string) {
  return DIMENSION_LABELS[key as DimensionKey] ?? key
}

function PersonaColumn({ name, view }: { name: string; view?: AuditReport['by_persona'][string] }) {
  return (
    <section className="persona-col">
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', marginBottom: '0.8rem' }}>
        <Scale size={18} color="var(--gold-hi)" />
        <h2>{PERSONA_LABELS[name] ?? name}</h2>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.2rem' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', color: 'var(--jade-hi)', fontSize: '0.88rem', fontWeight: 600, marginBottom: '0.4rem' }}>
            <CheckCircle2 size={15} />
            <span>核心长处</span>
          </div>
          <ul>
            {(view?.strengths ?? []).map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ul>
        </div>

        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', color: 'var(--cinnabar-hi)', fontSize: '0.88rem', fontWeight: 600, marginBottom: '0.4rem' }}>
            <AlertTriangle size={15} />
            <span>潜在风险</span>
          </div>
          <ul>
            {(view?.risks ?? []).map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ul>
        </div>

        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', color: 'var(--gold-hi)', fontSize: '0.88rem', fontWeight: 600, marginBottom: '0.4rem' }}>
            <Sparkles size={15} />
            <span>优化建议</span>
          </div>
          <ul>
            {(view?.suggestions ?? []).map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  )
}

export default function Audit() {
  const { id, auditId } = useParams()
  const navigate = useNavigate()
  const [report, setReport] = useState<AuditReport | null>(null)
  const [error, setError] = useState('')
  usePageTitle('审稿报告')

  useEffect(() => {
    if (!auditId) return
    api
      .getAudit(auditId)
      .then(setReport)
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '加载审计报告失败')
      })
  }, [auditId])

  const personas = report?.personas?.length
    ? report.personas
    : ['commercial_web', 'literary_texture']

  /** 把「建议调整」一键写回实验室滑条（不自动保存，用户检查后手动出版本） */
  function applyEdits() {
    if (!report?.card_id || !report.recommended_edits?.length) return
    navigate(`/p/${id}/lab/${report.card_id}`, { state: { applyEdits: report.recommended_edits } })
  }

  return (
    <div className="page page-wide">
      <Crumb projectId={id} current="审稿报告" />
      <div className="page-head">
        <div>
          <p className="kicker">AUDIT REPORT</p>
          <h1>双视角审稿报告</h1>
          <p className="sub">从商业爽感与文学质地双重视角审计风格卡技法配置，剖析冲突并提供微调建议。</p>
        </div>
        {report?.card_id ? (
          <Link className="btn secondary" to={`/p/${id}/lab/${report.card_id}`}>
            <FlaskConical size={16} />
            回到风格卡调试
          </Link>
        ) : null}
      </div>

      {error ? (
        <div className="empty-state">
          <h2>报告加载失败</h2>
          <p className="error">{error}</p>
        </div>
      ) : !report ? (
        <div style={{ display: 'grid', gap: '1.4rem' }}>
          <div className="two-col">
            <Skeleton h={320} />
            <Skeleton h={320} />
          </div>
          <Skeleton h={140} />
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.8rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem', fontSize: '0.85rem', color: 'var(--ink-soft)' }}>
            <span style={{ fontWeight: 600, color: 'var(--gold-hi)' }}>{projectName(id ?? '')}</span>
            <span>·</span>
            <span>风格卡 {report.card_id.slice(0, 8)}…</span>
            <span>·</span>
            <span className="chapter-seq-badge">版本 v{report.card_version}</span>
          </div>

          <div className="two-col">
            {personas.map((p) => (
              <PersonaColumn key={p} name={p} view={report.by_persona?.[p]} />
            ))}
          </div>

          {report.conflicts?.length ? (
            <div className="card" style={{ borderColor: 'rgba(255,107,74,0.3)', background: 'rgba(255,107,74,0.04)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', marginBottom: '0.8rem' }}>
                <AlertTriangle size={18} color="var(--cinnabar-hi)" />
                <h2 style={{ color: 'var(--cinnabar-hi)' }}>维度冲突与张力撕裂点</h2>
              </div>
              <ul style={{ margin: 0, paddingLeft: '1.2rem', lineHeight: 1.8, fontSize: '0.9rem' }}>
                {report.conflicts.map((c, i) => (
                  <li key={`${c.dimension}-${i}`}>
                    <strong style={{ color: 'var(--ink)' }}>{dimLabel(c.dimension)}</strong>：
                    <span style={{ color: 'var(--ink-soft)' }}>{c.summary}</span>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          <div className="card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '1rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
                <Sparkles size={18} color="var(--gold-hi)" />
                <h2>建议参数调整</h2>
              </div>
              {report.recommended_edits?.length ? (
                <button className="btn" type="button" onClick={applyEdits}>
                  <FlaskConical size={16} />
                  一键载入实验室并应用
                </button>
              ) : null}
            </div>

            {report.recommended_edits?.length ? (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '0.8rem', margin: '1rem 0' }}>
                  {report.recommended_edits.map((e, i) => (
                    <div
                      key={`${e.dimension}-${i}`}
                      style={{
                        padding: '0.8rem 1rem',
                        background: 'rgba(255,255,255,0.03)',
                        border: '1px solid var(--line-glass)',
                        borderRadius: 'var(--radius)',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.3rem' }}>
                        <strong style={{ color: 'var(--gold-hi)' }}>{dimLabel(e.dimension)}</strong>
                        <span style={{ fontFamily: 'var(--mono)', fontWeight: 700, color: 'var(--jade-hi)' }}>
                          目标 {e.target_level}
                        </span>
                      </div>
                      <p style={{ margin: 0, fontSize: '0.82rem', color: 'var(--ink-soft)', lineHeight: 1.5 }}>
                        {e.reason}
                      </p>
                    </div>
                  ))}
                </div>
                <p className="muted" style={{ fontSize: '0.82rem' }}>
                  点击上方按钮会将建议目标值填入实验室均衡推杆，经人工核验后可点击保存为全新版本。
                </p>
              </>
            ) : (
              <p className="muted">当前配置良好，没有列出调整建议。</p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
