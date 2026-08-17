import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { APIError, api } from '../api'
import { DIMENSION_LABELS, type AuditReport, type DimensionKey } from '../types'

const PERSONA_LABELS: Record<string, string> = {
  commercial_web: '网文可读',
  literary_texture: '文学质地',
}

function dimLabel(key: string) {
  return DIMENSION_LABELS[key as DimensionKey] ?? key
}

function PersonaColumn({ name, view }: { name: string; view?: AuditReport['by_persona'][string] }) {
  return (
    <section className="persona-col">
      <h2>{PERSONA_LABELS[name] ?? name}</h2>
      <h3>长处</h3>
      <ul>
        {(view?.strengths ?? []).map((s) => (
          <li key={s}>{s}</li>
        ))}
      </ul>
      <h3>风险</h3>
      <ul>
        {(view?.risks ?? []).map((s) => (
          <li key={s}>{s}</li>
        ))}
      </ul>
      <h3>建议</h3>
      <ul>
        {(view?.suggestions ?? []).map((s) => (
          <li key={s}>{s}</li>
        ))}
      </ul>
    </section>
  )
}

export default function Audit() {
  const { id, auditId } = useParams()
  const [report, setReport] = useState<AuditReport | null>(null)
  const [error, setError] = useState('')

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

  return (
    <div className="page page-wide">
      <p>
        <Link to={`/p/${id}`}>← 返回项目</Link>
        {report?.card_id ? (
          <>
            {' · '}
            <Link to={`/p/${id}/lab/${report.card_id}`}>打开工坊</Link>
          </>
        ) : null}
      </p>
      <div className="card">
        <h1>风格审计</h1>
        <p className="muted">从网文可读与文学质地两个视角检查技法配置，不评价是否像某个人。</p>
        {error ? <p className="error">{error}</p> : null}
        {!report && !error ? <p className="muted">正在加载报告…</p> : null}
        {report ? (
          <>
            <p className="muted">
              卡片 {report.card_id} · v{report.card_version}
            </p>
            <div className="two-col">
              {personas.map((p) => (
                <PersonaColumn key={p} name={p} view={report.by_persona?.[p]} />
              ))}
            </div>
          </>
        ) : null}
      </div>
      {report ? (
        <>
          <div className="card">
            <h2>冲突</h2>
            {report.conflicts?.length ? (
              <ul>
                {report.conflicts.map((c, i) => (
                  <li key={`${c.dimension}-${i}`}>
                    <strong>{dimLabel(c.dimension)}</strong>：{c.summary}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">没有列出冲突。</p>
            )}
          </div>
          <div className="card">
            <h2>建议调整</h2>
            {report.recommended_edits?.length ? (
              <ul>
                {report.recommended_edits.map((e, i) => (
                  <li key={`${e.dimension}-${i}`}>
                    <strong>{dimLabel(e.dimension)}</strong> → {e.target_level}：{e.reason}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">没有列出调整建议。</p>
            )}
          </div>
        </>
      ) : null}
    </div>
  )
}
