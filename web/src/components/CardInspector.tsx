import { ArrowRight, FlaskConical, X } from 'lucide-react'
import { Link } from 'react-router-dom'
import {
  DIMENSION_KEYS,
  DIMENSION_LABELS,
  KIND_LABEL,
  type StyleCard,
} from '../types'

type Props = {
  card: StyleCard | null
  projectId: string
  loading?: boolean
  error?: string
  onClose?: () => void
}

export default function CardInspector({ card, projectId, loading, error, onClose }: Props) {
  return (
    <aside className="card-inspector" aria-label="卡片详情">
      <div className="inspector-head">
        <div>
          <span className="eyebrow">CARD PROFILE</span>
          <h2>{card?.name ?? '选择一张卡片'}</h2>
        </div>
        {onClose ? (
          <button className="icon-btn" type="button" onClick={onClose} aria-label="关闭卡片详情" title="关闭">
            <X size={18} />
          </button>
        ) : null}
      </div>
      {loading ? <p className="muted">正在读取九维数据…</p> : null}
      {error ? <p className="error">{error}</p> : null}
      {!loading && !error && !card ? <p className="muted">点击卡片后在这里查看九维、约束与来源。</p> : null}
      {card ? (
        <>
          <div className="inspector-meta">
            <span>{KIND_LABEL[card.kind] ?? card.kind}</span>
            <span>版本 {card.version}</span>
          </div>
          <div className="dimension-bars">
            {DIMENSION_KEYS.map((key) => {
              const level = card.dimensions?.[key]?.level ?? 0
              return (
                <div className="dimension-bar" key={key}>
                  <span>{DIMENSION_LABELS[key]}</span>
                  <div className="dimension-track" aria-hidden="true">
                    <i style={{ width: String(level) + '%' }} />
                  </div>
                  <strong>{level}</strong>
                </div>
              )
            })}
          </div>
          {card.prohibitions?.length ? (
            <div className="inspector-section">
              <h3>约束</h3>
              <ul>
                {card.prohibitions.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </div>
          ) : null}
          <div className="inspector-actions">
            <Link className="btn" to={'/p/' + projectId + '/lab/' + card.id}>
              <FlaskConical size={16} />进入实验室
            </Link>
            <Link className="btn secondary" to={'/p/' + projectId + '/fuse?cards=' + card.id}>
              加入融合<ArrowRight size={16} />
            </Link>
          </div>
        </>
      ) : null}
    </aside>
  )
}
