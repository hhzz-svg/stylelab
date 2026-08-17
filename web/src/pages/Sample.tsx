import { Link, useParams } from 'react-router-dom'

export default function Sample() {
  const { id, sampleId } = useParams()
  return (
    <div className="page">
      <div className="card">
        <h1>试写样本</h1>
        <p className="placeholder">试写页面将在后续任务接入。</p>
        <p className="muted">样本 {sampleId}</p>
        <p>
          <Link to={`/p/${id}`}>返回项目</Link>
        </p>
      </div>
    </div>
  )
}
