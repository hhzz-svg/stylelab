import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, errMessage } from '../../api'
import type { Places } from '../../types'
import { ForestDiagram } from './LineageTree'

type Props = {
  projectId: string
  /** Changes whenever the graph does, to refetch. */
  version: unknown
  /** Open a place's profile. */
  onOpen: (nodeId: string) => void
}

/** 地理层级：大区域 → 其中的地点，并列出在每个地点发生的场景。 */
export default function PlaceTree({ projectId, version, onOpen }: Props) {
  const [places, setPlaces] = useState<Places | null>(null)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState('')

  useEffect(() => {
    let live = true
    api.graphPlaces(projectId).then(
      (p) => {
        if (!live) return
        setPlaces(p)
        setError('')
      },
      (err) => {
        if (live) setError(errMessage(err, '加载地理层级失败'))
      },
    )
    return () => {
      live = false
    }
  }, [projectId, version])

  const scenes = places?.scenes ?? {}
  const findName = (id: string): string => {
    const walk = (ns: Places['roots']): string | undefined => {
      for (const n of ns) {
        if (n.id === id) return n.name
        const hit = walk(n.children)
        if (hit) return hit
      }
      return undefined
    }
    return walk(places?.roots ?? []) ?? ''
  }
  const here = selected ? scenes[selected] ?? [] : []

  return (
    <div className="timeline-view">
      <div className="timeline-main">
        <ForestDiagram
          lineage={places}
          error={error}
          onSelect={setSelected}
          ariaLabel="地理层级"
          emptyText="还没有地点。添加「地理秘境」类实体，用「位于」关系把小地点连到大区域，这里会长出地理层级。"
          loopHint="请在地点档案里检查「位于」的方向。"
          note="「位于」关系的源头是较小的地点（藏经阁 → 青云山：位于）；地点的「所属」填另一个地点名时也算上级。节点下的数字是在这里发生的场景数，来自章节的场景切分。"
          badge={(id) => (scenes[id]?.length ? `${scenes[id].length} 场` : undefined)}
        />
      </div>
      <aside className="insight-panel timeline-side" aria-label="地点的场景">
        <section className="insight-section">
          <h3 className="insight-head">{selected ? findName(selected) : '在此发生的场景'}</h3>
          {!selected ? (
            <p className="insight-note">点选一个地点，查看在那里发生的场景。</p>
          ) : (
            <>
              {here.length === 0 ? (
                <p className="insight-note">还没有场景以这里为主要地点。</p>
              ) : (
                <ul className="plain-list place-scenes">
                  {here.map((s) => (
                    <li key={s.scene_id}>
                      <Link to={`/p/${projectId}/chapter/${s.chapter_id}`}>
                        第 {s.seq} 章 · 场景 {s.index + 1}
                      </Link>
                      ：{s.title}
                    </li>
                  ))}
                </ul>
              )}
              <button type="button" className="btn secondary sm" onClick={() => onOpen(selected)}>
                打开地点档案
              </button>
            </>
          )}
        </section>
      </aside>
    </div>
  )
}
