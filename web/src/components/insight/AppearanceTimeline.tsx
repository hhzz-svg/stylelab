import { useEffect, useState } from 'react'
import { api, errMessage, notify } from '../../api'
import type { Cooccurrence, GraphNode } from '../../types'

type Props = {
  projectId: string
  nodes: GraphNode[]
  /** Changes whenever the graph does, to refetch. */
  version: unknown
  onSelect: (nodeId: string) => void
  /** Called after a suggested relation is drawn. */
  onGraphChanged: () => void
}

const UNIT_LABEL = { paragraph: '段落', scene: '场景', mixed: '段落/场景' } as const

/** Heat from a mention count: none is blank, the busiest cell is solid. */
function heat(count: number, max: number): string | undefined {
  if (count === 0 || max === 0) return undefined
  const a = 0.18 + (0.82 * Math.log1p(count)) / Math.log1p(max)
  return `rgba(247, 203, 104, ${a.toFixed(3)})`
}

/** 出场时间线：谁在哪一章出场、和谁同场、谁久未露面。全部从正文统计，不调用模型。 */
export default function AppearanceTimeline({ projectId, nodes, version, onSelect, onGraphChanged }: Props) {
  const [report, setReport] = useState<Cooccurrence | null>(null)
  const [error, setError] = useState('')
  const [linking, setLinking] = useState('')
  const [splitting, setSplitting] = useState(false)
  const [refresh, setRefresh] = useState(0)

  useEffect(() => {
    let live = true
    api.graphCooccurrence(projectId).then(
      (r) => {
        if (!live) return
        setReport(r)
        setError('')
      },
      (err) => {
        if (live) setError(errMessage(err, '加载出场时间线失败'))
      },
    )
    return () => {
      live = false
    }
  }, [projectId, version, refresh])

  const byId = new Map(nodes.map((n) => [n.id, n]))
  const name = (id: string) => byId.get(id)?.name ?? '？'

  async function link(a: string, b: string) {
    setLinking(a + b)
    try {
      await api.saveGraphEdge(projectId, { source_id: a, target_id: b, relation: '同场' })
      notify(`已建立「${name(a)}」与「${name(b)}」的关系`, 'success')
      onGraphChanged()
    } catch (err) {
      notify(errMessage(err, '建立关系失败'), 'error')
    } finally {
      setLinking('')
    }
  }

  async function splitAll() {
    setSplitting(true)
    try {
      const r = await api.splitAllScenes(projectId)
      notify(`已把 ${r.chapters} 章切分为 ${r.scenes} 个场景，改按场景统计`, 'success')
      setRefresh((n) => n + 1)
    } catch (err) {
      notify(errMessage(err, '切分场景失败'), 'error')
    } finally {
      setSplitting(false)
    }
  }

  if (error) return <p className="error">{error}</p>
  if (!report) return <div className="timeline-view" />
  if (report.chapters.length === 0 || report.appearances.length === 0) {
    return (
      <div className="timeline-view">
        <p className="insight-note lineage-empty">
          {report.chapters.length === 0
            ? '还没有写好的章节。写出正文后，这里会统计每个人物在哪些章节出场。'
            : '正文里还没找到图谱中的任何名字。给人物补上别名（如「林师兄」）能提高命中。'}
        </p>
      </div>
    )
  }

  const max = Math.max(...report.appearances.flatMap((a) => a.per_chapter))
  // Pairs come ordered by id; show (and link) the more-mentioned one first.
  const total = new Map(report.appearances.map((a) => [a.id, a.total]))
  const ordered = (p: { a: string; b: string }) =>
    (total.get(p.b) ?? 0) > (total.get(p.a) ?? 0) ? [p.b, p.a] : [p.a, p.b]
  const unit = UNIT_LABEL[report.unit_kind]

  return (
    <div className="timeline-view">
      <div className="timeline-main">
        <div className="timeline-head">
          <p className="insight-note">
            {report.chapters.length} 章 · {report.units} 个{unit}。格子越亮，那一章提到得越多；名字按首次出场排序。
          </p>
          {report.unit_kind !== 'scene' ? (
            <button
              type="button"
              className="btn secondary sm"
              disabled={splitting}
              title="同一场景里出现才算同场，比按段落更贴近剧情"
              onClick={() => void splitAll()}
            >
              {splitting ? '切分中…' : '全书切分场景，改按场景统计'}
            </button>
          ) : null}
        </div>
        <div className="timeline-scroll">
          <table className="timeline-grid">
            <thead>
              <tr>
                <th className="timeline-name" scope="col">
                  人物 / 章
                </th>
                {report.chapters.map((seq, i) => (
                  <th key={seq} scope="col" title={`第 ${seq} 章 ${report.chapter_titles[i]}`}>
                    {seq}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {report.appearances.map((a) => (
                <tr key={a.id}>
                  <th scope="row" className="timeline-name">
                    <button type="button" className="link-btn" onClick={() => onSelect(a.id)}>
                      {name(a.id)}
                    </button>
                    <span className="timeline-total">{a.total}</span>
                  </th>
                  {a.per_chapter.map((c, i) => (
                    <td
                      key={report.chapters[i]}
                      data-count={c}
                      style={{ background: heat(c, max) }}
                      title={`${name(a.id)} · 第 ${report.chapters[i]} 章 ${report.chapter_titles[i]}：${c} 次`}
                    />
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <aside className="insight-panel timeline-side" aria-label="时间线洞察">
        <section className="insight-section">
          <h3 className="insight-head">久未出场</h3>
          {report.absent.length === 0 ? (
            <p className="insight-note">没有被冷落的人物。</p>
          ) : (
            <ul className="plain-list">
              {report.absent.map((a) => (
                <li key={a.id}>
                  <button type="button" className="link-btn" onClick={() => onSelect(a.id)}>
                    {name(a.id)}
                  </button>
                  ：已 {a.chapters_since} 章没有出场（最后在第 {a.last_seq} 章）
                </li>
              ))}
            </ul>
          )}
          <p className="insight-note">提到过至少 5 次、却已连续 10 章不见的人物。伏笔要不要收？</p>
        </section>

        <section className="insight-section">
          <h3 className="insight-head">潜在关系</h3>
          {report.suggestions.length === 0 ? (
            <p className="insight-note">常同场出现的人物，图谱里都已有关系。</p>
          ) : (
            <ul className="plain-list">
              {report.suggestions.map((p) => {
                const [x, y] = ordered(p)
                return (
                  <li key={p.a + p.b} className="suggest-row">
                    <span>
                      {name(x)} × {name(y)}
                      <span className="timeline-total">同场 {p.count} 次</span>
                    </span>
                    <button
                      type="button"
                      className="btn secondary sm"
                      disabled={linking === x + y}
                      onClick={() => void link(x, y)}
                    >
                      建立关系
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
          <p className="insight-note">正文里常在同一{unit}出现、图谱里却没连线的人物。</p>
        </section>
      </aside>
    </div>
  )
}
