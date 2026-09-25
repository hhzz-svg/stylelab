import { type RefObject, useEffect, useState } from 'react'
import { Pencil, Scissors } from 'lucide-react'
import { api, errMessage, notify } from '../api'
import { confirm } from './ConfirmDialog'
import type { SceneRecord } from '../types'

const CUE_LABEL: Record<string, string> = {
  separator: '分隔线',
  shift: '话题转换',
}

type Props = {
  projectId: string
  chapterId: string
  /** The saved body the scenes are cut from. */
  savedBody: string
  /** The editor holds unsaved changes. */
  dirty: boolean
  /** Changes when the saved chapter does, to refetch. */
  version: unknown
  bodyRef: RefObject<HTMLTextAreaElement>
}

/** Rune (code point) offset to the UTF-16 index a textarea uses. */
function utf16Index(text: string, runeOffset: number): number {
  let i = 0
  let runes = 0
  while (i < text.length && runes < runeOffset) {
    const code = text.charCodeAt(i)
    i += code >= 0xd800 && code <= 0xdbff ? 2 : 1
    runes++
  }
  return i
}

/** 本章场景：按正文自动切分出的场景卡片，点击在正文中选中该场景。 */
export default function ScenePanel({ projectId, chapterId, savedBody, dirty, version, bodyRef }: Props) {
  const [scenes, setScenes] = useState<SceneRecord[]>([])
  const [stale, setStale] = useState(false)
  const [names, setNames] = useState<Map<string, string>>(new Map())
  const [splitting, setSplitting] = useState(false)
  const [editing, setEditing] = useState('')
  const [draft, setDraft] = useState('')

  useEffect(() => {
    let live = true
    api.chapterScenes(chapterId).then(
      (r) => {
        if (!live) return
        setScenes(r.scenes)
        setStale(r.stale)
      },
      () => undefined,
    )
    return () => {
      live = false
    }
  }, [chapterId, version])

  // Names for the cast and location ids.
  useEffect(() => {
    if (scenes.length === 0) return
    let live = true
    api.getGraph(projectId).then(
      (g) => {
        if (live) setNames(new Map(g.nodes.map((n) => [n.id, n.name])))
      },
      () => undefined,
    )
    return () => {
      live = false
    }
  }, [projectId, scenes.length])

  const canSplit = savedBody.trim() !== '' && !dirty
  const aligned = !stale && !dirty

  async function split() {
    if (scenes.some((s) => s.origin === 'edited')) {
      const ok = await confirm({
        title: '重新切分场景？',
        body: '你改过的场景标题和摘要会被新的切分结果覆盖。',
        confirmText: '重新切分',
      })
      if (!ok) return
    }
    setSplitting(true)
    try {
      const r = await api.splitScenes(chapterId)
      setScenes(r.scenes)
      setStale(false)
      notify(`已切分为 ${r.scenes.length} 个场景`, 'success')
    } catch (err) {
      notify(errMessage(err, '切分场景失败'), 'error')
    } finally {
      setSplitting(false)
    }
  }

  async function saveTitle(scene: SceneRecord) {
    setEditing('')
    const title = draft.trim()
    if (!title || title === scene.title) return
    try {
      const updated = await api.updateScene(scene.id, { title })
      setScenes((prev) => prev.map((s) => (s.id === updated.id ? updated : s)))
    } catch (err) {
      notify(errMessage(err, '保存场景标题失败'), 'error')
    }
  }

  function jumpTo(scene: SceneRecord) {
    const ta = bodyRef.current
    if (!ta) return
    if (!aligned) {
      notify(dirty ? '正文有未保存的修改，保存并重新切分后再定位' : '正文改过了，重新切分后再定位', 'info')
      return
    }
    const start = utf16Index(savedBody, scene.start)
    const end = utf16Index(savedBody, scene.end)
    ta.focus()
    ta.setSelectionRange(start, end)
    // Selecting does not scroll a textarea; place the scene a third down.
    ta.scrollTop = Math.max(0, (start / Math.max(1, ta.value.length)) * ta.scrollHeight - ta.clientHeight / 3)
  }

  return (
    <section className="scene-panel" aria-label="本章场景">
      <div className="scene-panel-head">
        <h3>本章场景{scenes.length ? `（${scenes.length}）` : ''}</h3>
        <button
          type="button"
          className="btn secondary sm"
          disabled={!canSplit || splitting}
          title={dirty ? '先保存正文再切分' : undefined}
          onClick={() => void split()}
        >
          <Scissors size={14} /> {splitting ? '切分中…' : scenes.length ? '重新切分' : '切分场景'}
        </button>
      </div>
      {stale ? <p className="scene-warning">正文改过了，下面的场景位置可能已对不上，建议重新切分。</p> : null}
      {scenes.length === 0 ? (
        <p className="insight-note">
          按分隔线、「次日」「与此同时」这类转换词、用词和出场人物的变化，把本章自动切成场景。不调用模型。
        </p>
      ) : (
        <ol className="scene-list">
          {scenes.map((s) => (
            <li key={s.id} className="scene-card" data-index={s.index}>
              <div className="scene-card-head">
                <span className="scene-no">场景 {s.index + 1}</span>
                {s.cue ? (
                  <span className="role-chip">{s.cue === 'transition' ? s.cue_text : CUE_LABEL[s.cue]}</span>
                ) : null}
                <span className="scene-runes">{s.runes} 字</span>
                <button
                  type="button"
                  className="icon-btn sm"
                  title="改标题"
                  aria-label={`改场景 ${s.index + 1} 的标题`}
                  onClick={() => {
                    setDraft(s.title)
                    setEditing(s.id)
                  }}
                >
                  <Pencil size={12} />
                </button>
              </div>
              {editing === s.id ? (
                <input
                  className="scene-title-input"
                  value={draft}
                  autoFocus
                  aria-label="场景标题"
                  onChange={(e) => setDraft(e.target.value)}
                  onBlur={() => void saveTitle(s)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void saveTitle(s)
                    if (e.key === 'Escape') setEditing('')
                  }}
                />
              ) : (
                <button
                  type="button"
                  className="scene-title"
                  title="在正文中定位这一场景"
                  onClick={() => jumpTo(s)}
                >
                  {s.title}
                </button>
              )}
              <p className="scene-meta">
                {s.location ? <span>📍 {names.get(s.location) ?? '…'}</span> : null}
                {s.characters.length ? <span>🧑 {s.characters.map((id) => names.get(id) ?? '…').join('、')}</span> : null}
              </p>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
