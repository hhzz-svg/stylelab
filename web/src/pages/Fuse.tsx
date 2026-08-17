import { Link, useParams } from 'react-router-dom'

export default function Fuse() {
  const { id } = useParams()
  return (
    <div className="page">
      <div className="card">
        <h1>风格融合</h1>
        <p className="placeholder">融合工作台将在后续任务接入。</p>
        <p>
          <Link to={`/p/${id}`}>返回项目</Link>
        </p>
      </div>
    </div>
  )
}
