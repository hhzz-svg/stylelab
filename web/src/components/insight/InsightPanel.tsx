import type { AnalysisWeights, GraphAnalysis, GraphNode, NodeRole } from '../../types'
import { communityColor } from './palette'

const ROLE_LABEL: Record<Exclude<NodeRole, ''>, string> = {
  core: '核心',
  hub: '枢纽',
  peripheral: '边缘',
  isolated: '孤立',
}

const TOP_N = 10
const MEMBERS_SHOWN = 6

/** How clearly the network splits into groups, in words. */
function modularityVerdict(q: number): string {
  if (q >= 0.3) return '分群明显'
  if (q >= 0.1) return '有一定分群'
  return '关系网较松散，分群不明显'
}

const WEIGHT_OPTIONS: { value: AnalysisWeights; label: string; title: string }[] = [
  { value: 'graph', label: '图谱关系', title: '按你画出的关系线计算' },
  { value: 'text', label: '正文共现', title: '按人物在正文同一段落出现的次数计算' },
  { value: 'both', label: '两者', title: '关系线与正文共现相加' },
]

type Props = {
  analysis: GraphAnalysis | null
  weights: AnalysisWeights
  onWeightsChange: (w: AnalysisWeights) => void
  nodes: GraphNode[]
  selectedId?: string
  onSelect: (nodeId: string) => void
}

/** 图谱洞察侧栏：由关系网络结构算出的重要度排行与自动发现的阵营。 */
export default function InsightPanel({ analysis, weights, onWeightsChange, nodes, selectedId, onSelect }: Props) {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const ranked = (analysis?.ranking ?? []).filter((r) => r.degree > 0 && byId.has(r.id)).slice(0, TOP_N)
  const hubs = (analysis?.ranking ?? []).filter((r) => r.role === 'hub' && byId.has(r.id))
  const communities = analysis?.communities ?? []
  const name = (id: string) => byId.get(id)?.name ?? '？'

  return (
    <aside className="insight-panel" aria-label="图谱洞察">
      <div className="weights-switch" role="radiogroup" aria-label="分析依据">
        <span>依据</span>
        {WEIGHT_OPTIONS.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={weights === o.value}
            className={'weights-option' + (weights === o.value ? ' active' : '')}
            title={o.title}
            onClick={() => onWeightsChange(o.value)}
          >
            {o.label}
          </button>
        ))}
      </div>
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
            ：不同圈子之间的往来大多要经过他们。
          </p>
        ) : null}
        <p className="insight-note">
          分数按 PageRank 计算：被重要人物紧密关联的人更重要，满分 100。枢纽按介数中心性计算。
        </p>
      </section>

      <section className="insight-section">
        <h3 className="insight-head">阵营自动发现</h3>
        {communities.length === 0 ? (
          <p className="insight-note">关系还不够多，暂时分不出群落。</p>
        ) : (
          <>
            <p className="insight-note">
              模块度 {analysis!.modularity.toFixed(2)}：{modularityVerdict(analysis!.modularity)}
            </p>
            <ul className="community-list">
              {communities.map((c) => (
                <li key={c.index} className="community-card">
                  <div className="community-head">
                    <span className="community-dot" style={{ background: communityColor(c.index) }} aria-hidden="true" />
                    <strong>{c.faction || `以「${name(c.members[0])}」为核心`}</strong>
                    <span className="community-size">{c.members.length} 个</span>
                  </div>
                  <p className="community-members">
                    {c.members.slice(0, MEMBERS_SHOWN).map((id, i) => (
                      <span key={id}>
                        {i > 0 ? '、' : ''}
                        <button type="button" className="link-btn" onClick={() => onSelect(id)}>
                          {name(id)}
                        </button>
                      </span>
                    ))}
                    {c.members.length > MEMBERS_SHOWN ? ` 等 ${c.members.length} 个` : ''}
                  </p>
                  {c.outliers.map((o) => (
                    <p key={o.id} className="community-outlier">
                      <button type="button" className="link-btn" onClick={() => onSelect(o.id)}>
                        {name(o.id)}
                      </button>
                      标注为「{o.faction}」，却与{c.faction}往来密切：暗线同盟、卧底，还是标注过时了？
                    </p>
                  ))}
                </li>
              ))}
            </ul>
          </>
        )}
        <p className="insight-note">
          按 Louvain 算法把关系紧密的人自动分成群落，不看你标注的门派；和标注对不上的人会被单独指出。图上用「按算法阵营着色」查看。
        </p>
      </section>
    </aside>
  )
}
