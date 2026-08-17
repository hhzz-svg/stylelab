import { Link, useParams } from 'react-router-dom'

export default function Lab() {
  const { id, cardId } = useParams()
  return (
    <div className="page">
      <div className="card">
        <h1>风格工坊</h1>
        <p className="placeholder">工坊页面将在后续任务接入九维编辑与导出。</p>
        <p className="muted">
          项目 {id} · 卡片 {cardId}
        </p>
        <p>
          <Link to={`/p/${id}`}>返回项目</Link>
        </p>
      </div>
    </div>
  )
}
