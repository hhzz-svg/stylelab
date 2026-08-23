import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { BookMarked, Search, SlidersHorizontal, Sparkles, Trash2, X } from 'lucide-react'
import { APIError, api, notify } from '../api'
import { confirm } from '../components/ConfirmDialog'
import Crumb from '../components/Crumb'
import JobProgress from '../components/JobProgress'
import Skeleton from '../components/Skeleton'
import { usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import { rememberProject } from '../projectCache'
import type { BibleEntry, BibleEntryKind, BibleEntryStatus, BibleEntrySummary, JobStatus } from '../types'

const KIND_LABEL: Record<BibleEntryKind, string> = {
  character: '人物',
  setting: '设定',
  thread: '伏笔',
}

const STATUS_LABEL: Record<BibleEntryStatus, string> = {
  active: '进行中',
  resolved: '已收线',
}

const ORIGIN_LABEL: Record<string, string> = {
  manual: '手动',
  auto: 'AI',
}

type SortMode = 'updated' | 'kind' | 'name'

export default function Bible() {
  const { id } = useParams()
  const projectId = id ?? ''
  usePageTitle('设定集')

  const [entries, setEntries] = useState<BibleEntrySummary[] | null>(null)
  const [detail, setDetail] = useState<BibleEntry | null>(null)
  const [selectedId, setSelectedId] = useState('')
  const [creating, setCreating] = useState(false)
  const [query, setQuery] = useState('')
  const [kindFilter, setKindFilter] = useState<'all' | BibleEntryKind>('all')
  const [sort, setSort] = useState<SortMode>('kind')
  const [error, setError] = useState('')
  const [detailError, setDetailError] = useState('')
  const [busy, setBusy] = useState(false)
  const [model, setModel] = useState('')
  const [jobId, setJobId] = useState('')

  // 编辑镜像
  const [kind, setKind] = useState<BibleEntryKind>('character')
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [status, setStatus] = useState<BibleEntryStatus>('active')

  useEffect(() => {
    if (!projectId) return
    Promise.all([api.listBible(projectId), api.getProject(projectId)])
      .then(([data, project]) => {
        setEntries(data.entries ?? [])
        rememberProject(projectId, project.name)
        setError('')
      })
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '设定集加载失败')
        setEntries([])
      })
  }, [projectId])

  const counts = useMemo(() => {
    const list = entries ?? []
    const characters = list.filter((e) => e.kind === 'character').length
    const settings = list.filter((e) => e.kind === 'setting').length
    const threadsOpen = list.filter((e) => e.kind === 'thread' && e.status === 'active').length
    return { characters, settings, threadsOpen }
  }, [entries])

  const visible = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase()
    const kindOrder: Record<BibleEntryKind, number> = { character: 0, setting: 1, thread: 2 }
    return [...(entries ?? [])]
      .filter((e) => kindFilter === 'all' || e.kind === kindFilter)
      .filter((e) => !normalized || e.name.toLocaleLowerCase().includes(normalized) || e.content_preview.toLocaleLowerCase().includes(normalized))
      .sort((a, b) => {
        if (sort === 'name') return a.name.localeCompare(b.name, 'zh-CN')
        if (sort === 'updated') return b.updated_at.localeCompare(a.updated_at)
        const k = kindOrder[a.kind] - kindOrder[b.kind]
        return k !== 0 ? k : a.name.localeCompare(b.name, 'zh-CN')
      })
  }, [entries, kindFilter, query, sort])

  function resetEditor() {
    setKind('character')
    setName('')
    setContent('')
    setStatus('active')
    setDetail(null)
    setSelectedId('')
    setCreating(false)
    setDetailError('')
  }

  async function selectEntry(entry: BibleEntrySummary) {
    if (selectedId === entry.id && (detail || creating)) return
    setSelectedId(entry.id)
    setCreating(false)
    setDetailError('')
    try {
      const full = await api.getBible(entry.id)
      setDetail(full)
      setKind(full.kind)
      setName(full.name)
      setContent(full.content)
      setStatus(full.status)
    } catch (err) {
      setDetailError(err instanceof APIError ? err.message : '条目读取失败')
    }
  }

  function startCreate() {
    setSelectedId('')
    setCreating(true)
    setDetail(null)
    setDetailError('')
    setKind('character')
    setName('')
    setContent('')
    setStatus('active')
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setDetailError('名称不能为空')
      return
    }
    setBusy(true)
    setDetailError('')
    try {
      if (creating) {
        const created = await api.createBible(projectId, kind, name.trim(), content.trim(), status)
        setEntries((prev) => (prev ? [...prev, toSummary(created)] : prev))
        resetEditor()
        notify('已加入设定集', 'success')
      } else if (detail) {
        const saved = await api.patchBible(detail.id, {
          kind,
          name: name.trim(),
          content: content.trim(),
          status,
        })
        setEntries((prev) =>
          prev ? prev.map((e) => (e.id === saved.id ? toSummary(saved) : e)) : prev,
        )
        setDetail(saved)
        setKind(saved.kind)
        setName(saved.name)
        setContent(saved.content)
        setStatus(saved.status)
        notify('已保存', 'success')
      }
    } catch (err) {
      setDetailError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete() {
    if (!detail) return
    const ok = await confirm({
      title: '删除这条设定？',
      body: `「${detail.name}」将从设定集移除，已写章节不受影响。`,
      confirmText: '删除',
      danger: true,
    })
    if (!ok) return
    setBusy(true)
    try {
      await api.deleteBible(detail.id)
      setEntries((prev) => (prev ? prev.filter((e) => e.id !== detail.id) : prev))
      resetEditor()
      notify('已删除', 'success')
    } catch (err) {
      setDetailError(err instanceof APIError ? err.message : '删除失败')
    } finally {
      setBusy(false)
    }
  }

  async function onSync() {
    if (jobId) return
    try {
      const res = await api.syncBible(projectId, model.trim())
      setJobId(res.job_id)
      registerJob({
        jobId: res.job_id,
        projectId,
        kind: 'bible_sync',
        label: '从已有章节同步设定集',
        startedAt: Date.now(),
      })
      notify('设定集同步任务已提交', 'info')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '提交同步失败', 'error')
    }
  }

  function onJobStatus(status: JobStatus) {
    if (status === 'succeeded') {
      setJobId('')
      void api.listBible(projectId).then((data) => setEntries(data.entries ?? [])).catch(() => {})
      notify('设定集同步完成', 'success')
    } else if (status === 'failed' || status === 'canceled') {
      setJobId('')
    }
  }

  const editing = creating || !!detail

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current="设定集" />
      <div className="page-head">
        <div>
          <p className="kicker">STORY BIBLE</p>
          <h1>设定集</h1>
          <p className="sub">登记人物、设定、伏笔。写章时注入，写完自动维护，老项目可一键回填。</p>
        </div>
        <div className="status-grid">
          <div className="status-chip">
            <span>人物</span>
            <strong>{counts.characters}</strong>
          </div>
          <div className="status-chip">
            <span>设定</span>
            <strong>{counts.settings}</strong>
          </div>
          <div className="status-chip">
            <span>伏笔未收</span>
            <strong>{counts.threadsOpen}</strong>
          </div>
        </div>
      </div>

      <div className="bible-sync-bar">
        <div className="stack">
          <label>
            同步用模型（留空使用默认）
            <input type="text" value={model} onChange={(e) => setModel(e.target.value)} placeholder="gpt-4o-mini" />
          </label>
        </div>
        <button className="btn secondary" type="button" onClick={() => void onSync()} disabled={!!jobId}>
          <Sparkles size={15} />{jobId ? '同步中…' : '从已有章节同步'}
        </button>
      </div>
      {jobId ? (
        <JobProgress jobId={jobId} projectId={projectId} autoNavigate={false} onStatus={onJobStatus} />
      ) : null}

      <div className="library-tools" aria-label="设定集筛选">
        <label className="search-field">
          <Search size={17} aria-hidden="true" />
          <span className="sr-only">搜索条目</span>
          <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索名称或内容" />
        </label>
        <label className="select-field">
          <SlidersHorizontal size={16} aria-hidden="true" />
          <span className="sr-only">条目类型</span>
          <select value={kindFilter} onChange={(e) => setKindFilter(e.target.value as 'all' | BibleEntryKind)}>
            <option value="all">全部类型</option>
            <option value="character">人物</option>
            <option value="setting">设定</option>
            <option value="thread">伏笔</option>
          </select>
        </label>
        <label className="select-field">
          <span className="sr-only">排序方式</span>
          <select value={sort} onChange={(e) => setSort(e.target.value as SortMode)}>
            <option value="kind">按类型</option>
            <option value="updated">最近更新</option>
            <option value="name">按名称</option>
          </select>
        </label>
      </div>

      {error ? <p className="error">{error}</p> : null}
      <div className="card-library-layout">
        <section className="bible-list" aria-label="设定集条目">
          {entries === null ? (
            Array.from({ length: 5 }, (_, i) => <Skeleton key={i} h={84} />)
          ) : visible.length ? (
            visible.map((entry) => (
              <button
                key={entry.id}
                type="button"
                className={`bible-row${entry.id === selectedId ? ' active' : ''}`}
                onClick={() => void selectEntry(entry)}
              >
                <span className={`bible-kind kind-${entry.kind}`}>{KIND_LABEL[entry.kind]}</span>
                <span className="bible-row-main">
                  <span className="bible-row-name">{entry.name}</span>
                  <span className="bible-row-preview">{entry.content_preview || '（无内容）'}</span>
                </span>
                <span className="bible-row-meta">
                  {entry.kind === 'thread' && entry.status === 'resolved' ? (
                    <span className="bible-tag resolved">已收线</span>
                  ) : null}
                  <span className="bible-tag origin">{ORIGIN_LABEL[entry.origin]}</span>
                  {entry.origin === 'auto' && entry.source_seq > 0 ? (
                    <span className="bible-tag seq">第 {entry.source_seq} 章</span>
                  ) : null}
                </span>
              </button>
            ))
          ) : (
            <div className="empty-state compact">
              <BookMarked size={24} aria-hidden="true" />
              <h2>{entries.length ? '没有符合条件的条目' : '设定集还是空的'}</h2>
              <p className="muted">
                {entries.length ? '换一个名称或类型筛选。' : '手动新建，或写完章节后自动维护。'}
              </p>
            </div>
          )}
        </section>

        <aside className="bible-editor" aria-label="设定集编辑">
          {!editing ? (
            <div className="bible-editor-empty">
              <p className="muted">选中一条编辑，或新建一条。</p>
              <button className="btn" type="button" onClick={startCreate}>新建条目</button>
            </div>
          ) : (
            <form className="stack bible-editor-form" onSubmit={onSubmit}>
              <div className="bible-editor-head">
                <h2>{creating ? '新建条目' : '编辑条目'}</h2>
                <button
                  className="icon-btn"
                  type="button"
                  aria-label="关闭编辑"
                  title="关闭"
                  onClick={resetEditor}
                >
                  <X size={18} />
                </button>
              </div>
              {!creating && detail ? (
                <div className="inspector-meta">
                  <span>{ORIGIN_LABEL[detail.origin]}</span>
                  {detail.origin === 'auto' && detail.source_seq > 0 ? <span>第 {detail.source_seq} 章</span> : null}
                  {detail.kind === 'thread' ? <span>{STATUS_LABEL[detail.status]}</span> : null}
                </div>
              ) : null}
              <label>
                类型
                <select value={kind} onChange={(e) => setKind(e.target.value as BibleEntryKind)}>
                  <option value="character">人物</option>
                  <option value="setting">设定</option>
                  <option value="thread">伏笔</option>
                </select>
              </label>
              <label>
                名称
                <input type="text" value={name} maxLength={40} onChange={(e) => setName(e.target.value)} />
              </label>
              <label>
                内容（给后续章节参考）
                <textarea value={content} rows={6} maxLength={2000} onChange={(e) => setContent(e.target.value)} />
              </label>
              {kind === 'thread' ? (
                <label>
                  状态
                  <select value={status} onChange={(e) => setStatus(e.target.value as BibleEntryStatus)}>
                    <option value="active">进行中</option>
                    <option value="resolved">已收线</option>
                  </select>
                </label>
              ) : null}
              {detailError ? <p className="error">{detailError}</p> : null}
              <div className="inspector-actions">
                <button className="btn" type="submit" disabled={busy}>
                  {busy ? '保存中…' : creating ? '新建' : '保存'}
                </button>
                {!creating && detail ? (
                  <button className="btn danger sm" type="button" onClick={() => void onDelete()} disabled={busy}>
                    <Trash2 size={15} />删除
                  </button>
                ) : null}
              </div>
            </form>
          )}
        </aside>
      </div>

      {entries !== null && !entries.length && !editing ? (
        <div className="bible-empty-hint">
          <p className="muted">
            没有已写章节也能手动建档。写完章节后 AI 会自动追加；老项目点
            <Link to={`/p/${projectId}/bible`}>从已有章节同步</Link>回填。
          </p>
        </div>
      ) : null}
    </div>
  )
}

function toSummary(e: BibleEntry): BibleEntrySummary {
  const preview = Array.from(e.content.trim()).slice(0, 80).join('')
  return {
    id: e.id,
    kind: e.kind,
    name: e.name,
    status: e.status,
    origin: e.origin,
    source_seq: e.source_seq,
    content_preview: preview,
    updated_at: e.updated_at,
  }
}
