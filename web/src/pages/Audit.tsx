import { Link, useParams } from 'react-router-dom'

export default function Audit() {
  const { id, auditId } = useParams()
  return (
    <div className="page">
      <div className="card">
        <h1>风格审计</h1>
        <p className="placeholder">审计报告页将在后续任务接入。</p>
        <p className="muted">报告 {auditId}</p>
        <p>
          <Link to={`/p/${id}`}>返回项目</Link>
        </p>
      </div>
    </div>
  )
}
