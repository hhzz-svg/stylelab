import type { GraphAnalysis, GraphNode, NodeRole } from '../../types'

const ROLE_LABEL: Record<Exclude<NodeRole, ''>, string> = {
  core: '核心',
  hub: '枢纽',
  peripheral: '边缘',
  isolated: '孤立',
}

const TOP_N = 10

type Props = {
  analysis: GraphAnalysis | null
  nodes: GraphNode[]
  selectedId?: string
  onSelect: (nodeId: string) => void
}

/** 图谱洞察侧栏：按关系网络结构算出的人物重要度排行。 */
export default function InsightPanel({ analysis, nodes, selectedId, onSelect }: Props) {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const ranked = (analysis?.ranking ?? []).filter((r) => r.degree > 0 && byId.has(r.id)).slice(0, TOP_N)
  const hubs = (analysis?.ranking ?? []).filter((r) => r.role === 'hub' && byId.has(r.id))

  return (
    <aside className="insight-panel" aria-label="图谱洞察">
      <section className="insight-section">
        <h3 className="insight-head">重要度排行</h3>
        {ranked.length === 0 ? (
          <p className="insight-note">建立几条关系后，这里会按关系网络算出谁是核心人物。</p>
        ) : (
          <ol className="rank-list">
            {ranked.map((r) => {
              const node = byId.get(r.id)!
              return (
                <li key={r.id}>
                  <button
                    type="button"
                    className={'rank-row' + (selectedId === r.id ? ' active' : '')}
                    onClick={() => onSelect(r.id)}
                    title={`PageRank ${r.pagerank.toFixed(3)} · 介数 ${r.betweenness.toFixed(3)} · ${r.degree} 条关系`}
                  >
                    <span className="rank-no">{r.rank}</span>
                    <span className="rank-name">{node.name}</span>
                    {r.role ? <span className={`role-chip ${r.role}`}>{ROLE_LABEL[r.role]}</span> : null}
                    <span className="rank-bar" aria-hidden="true">
                      <span className="rank-fill" style={{ width: `${Math.max(4, r.score)}%` }} />
                    </span>
                    <span className="rank-score">{r.score}</span>
                  </button>
                </li>
              )
            })}
          </ol>
        )}
        {hubs.length > 0 ? (
          <p className="insight-note">
            <span className="role-chip hub">枢纽</span>
            {hubs.map((h) => byId.get(h.id)!.name).join('、')}
            ：本身不一定显眼，但不同圈子之间的往来都要经过他们。
          </p>
        ) : null}
        <p className="insight-note">
          分数按 PageRank 计算：被重要人物紧密关联的人更重要，满分 100。枢纽按介数中心性计算。
        </p>
      </section>
    </aside>
  )
}
