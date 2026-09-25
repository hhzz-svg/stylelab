import { useEffect, useMemo, useState } from 'react'
import { api, errMessage } from '../../api'
import type { Lineage, LineageNode } from '../../types'

// Layout units: the server lays the forest out in sibling-gap units
// (Buchheim-Walker); these turn them into pixels.
const UNIT_X = 128
const UNIT_Y = 104
const NODE_W = 108
const NODE_H = 42
const PAD = 32

type DiagramProps = {
  lineage: Lineage | null
  error?: string
  onSelect: (nodeId: string) => void
  ariaLabel: string
  emptyText: string
  /** Shown under the diagram. */
  note: string
  /** How to fix a loop, after "绕成了圈：A → B". */
  loopHint: string
  /** A short line under a node, such as its scene count. */
  badge?: (nodeId: string) => string | undefined
}

type Placed = { node: LineageNode; cx: number; cy: number }

function centre(n: LineageNode) {
  return { cx: PAD + NODE_W / 2 + n.x * UNIT_X, cy: PAD + NODE_H / 2 + n.depth * UNIT_Y }
}

/** 画一片由服务器排好版的树林（谱系树、地理层级共用）。 */
export function ForestDiagram({ lineage, error, onSelect, ariaLabel, emptyText, note, loopHint, badge }: DiagramProps) {
  const { placed, byId } = useMemo(() => {
    const placed: Placed[] = []
    const byId = new Map<string, Placed>()
    const walk = (n: LineageNode) => {
      const p = { node: n, ...centre(n) }
      placed.push(p)
      byId.set(n.id, p)
      n.children.forEach(walk)
    }
    lineage?.roots.forEach(walk)
    return { placed, byId }
  }, [lineage])

  if (error) return <p className="error">{error}</p>
  if (!lineage) return <div className="lineage-scroll" />
  if (lineage.roots.length === 0) {
    return (
      <div className="lineage-scroll">
        <p className="insight-note lineage-empty">{emptyText}</p>
      </div>
    )
  }

  const width = PAD * 2 + NODE_W + lineage.width * UNIT_X
  const height = PAD * 2 + NODE_H + lineage.depth * UNIT_Y
  const name = (id: string) => byId.get(id)?.node.name ?? id

  return (
    <div className="lineage-view">
      {lineage.cycles.length > 0 ? (
        <div className="lineage-warning">
          {lineage.cycles.map((c) => (
            <p key={c.join()}>
              上下级关系绕成了圈：{c.map(name).join(' → ')}。已断开其中一条，{loopHint}
            </p>
          ))}
        </div>
      ) : null}
      <div className="lineage-scroll">
        <svg className="lineage-svg" width={width} height={height + (badge ? 14 : 0)} role="img" aria-label={ariaLabel}>
          <g>
            {placed.map(({ node, cx, cy }) =>
              node.children.map((c) => {
                const child = byId.get(c.id)!
                const y1 = cy + NODE_H / 2
                const y2 = child.cy - NODE_H / 2
                const mid = (y1 + y2) / 2
                return (
                  <path
                    key={`${node.id}>${c.id}`}
                    className={'lineage-link' + (c.relation ? ' ranked' : '')}
                    d={`M ${cx} ${y1} V ${mid} H ${child.cx} V ${y2}`}
                  />
                )
              }),
            )}
            {placed.map(({ node, cx, cy }) =>
              node.extra_parents.map((pid) => {
                const p = byId.get(pid)
                if (!p) return null
                const y1 = p.cy + NODE_H / 2
                const y2 = cy - NODE_H / 2
                return (
                  <path
                    key={`${pid}~${node.id}`}
                    className="lineage-extra"
                    d={`M ${p.cx} ${y1} C ${p.cx} ${y1 + 40}, ${cx} ${y2 - 40}, ${cx} ${y2}`}
                  />
                )
              }),
            )}
          </g>
          <g>
            {placed.map(({ node, cx, cy }) => (
              <g
                key={node.id}
                className={
                  'lineage-node' + (node.kind === 'faction' ? ' faction' : '') + (node.virtual ? ' virtual' : '')
                }
                transform={`translate(${cx - NODE_W / 2}, ${cy - NODE_H / 2})`}
                onClick={node.virtual ? undefined : () => onSelect(node.id)}
                role={node.virtual ? undefined : 'button'}
                data-id={node.id}
              >
                <rect width={NODE_W} height={NODE_H} rx={10} />
                <text x={NODE_W / 2} y={NODE_H / 2 + 5} textAnchor="middle">
                  {node.name.length > 7 ? node.name.slice(0, 6) + '…' : node.name}
                </text>
                {node.relation ? (
                  <text className="lineage-relation" x={NODE_W / 2} y={-7} textAnchor="middle">
                    {node.relation}
                  </text>
                ) : null}
                {badge?.(node.id) ? (
                  <text className="lineage-badge" x={NODE_W / 2} y={NODE_H + 13} textAnchor="middle">
                    {badge(node.id)}
                  </text>
                ) : null}
              </g>
            ))}
          </g>
        </svg>
      </div>
      <p className="insight-note">{note}</p>
    </div>
  )
}

type Props = {
  projectId: string
  /** Changes whenever the graph does, to refetch. */
  version: unknown
  onSelect: (nodeId: string) => void
}

/** 人物谱系树：势力 → 成员，师父 → 徒弟，父母 → 子女。 */
export default function LineageTree({ projectId, version, onSelect }: Props) {
  const [lineage, setLineage] = useState<Lineage | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let live = true
    api.graphLineage(projectId).then(
      (l) => {
        if (!live) return
        setLineage(l)
        setError('')
      },
      (err) => {
        if (live) setError(errMessage(err, '加载谱系树失败'))
      },
    )
    return () => {
      live = false
    }
  }, [projectId, version])

  return (
    <ForestDiagram
      lineage={lineage}
      error={error}
      onSelect={onSelect}
      ariaLabel="人物谱系树"
      emptyText="还没有人物或势力。添加人物并标注门派，或建立师徒、父子等关系后，这里会长出谱系树。"
      loopHint="请在人物档案里检查方向。"
      note="师徒、父子、君臣、主仆等上下级关系按「关系的源头是上位者」排列；方向反了，在人物档案里点 ⇄ 调换。虚线是另有的师承。"
    />
  )
}
