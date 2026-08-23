import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookOpen, Library, Plus, Sparkles, Trash2 } from 'lucide-react'
import { APIError, api, notify } from '../api'
import { confirm } from '../components/ConfirmDialog'
import Skeleton from '../components/Skeleton'
import { usePageTitle } from '../hooks'
import type { Project } from '../types'

export default function Projects() {
  usePageTitle('作品集')
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [listError, setListError] = useState('')
  const [name, setName] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      const data = await api.listProjects()
      setProjects(data.projects ?? [])
      setListError('')
    } catch (err) {
      setListError(err instanceof APIError ? err.message : '加载失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  // 回到本标签页时重取，避免整个会话都看旧列表
  useEffect(() => {
    function onVisible() {
      if (document.visibilityState === 'visible') void load()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setFormError('')
    setBusy(true)
    try {
      const created = await api.createProject(name.trim())
      setName('')
      setProjects((prev) => [created, ...(prev ?? [])])
      notify(`已创建作品「${created.name}」`, 'success')
    } catch (err) {
      setFormError(err instanceof APIError ? err.message : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(id: string, title: string) {
    const ok = await confirm({
      title: `删除作品「${title}」？`,
      body: '其中的样章、风格卡片、章节正文、设定集与审计记录会一并删除，无法恢复。',
      confirmText: '删除作品',
      danger: true,
    })
    if (!ok) return
    try {
      await api.deleteProject(id)
      setProjects((prev) => (prev ?? []).filter((p) => p.id !== id))
      notify(`已删除「${title}」`, 'success')
    } catch (err) {
      notify(err instanceof APIError ? err.message : '删除失败', 'error')
    }
  }

  const projectCount = projects?.length ?? 0

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <p className="kicker">STUDIO WORKSPACE</p>
          <h1>小说作品集</h1>
          <p className="sub">
            汇聚样章语料、九维风格卡、炼金融合矩阵与章节写作台的创作中枢。
          </p>
        </div>
        {projectCount > 0 && (
          <div className="status-chip">
            <span>收录作品</span>
            <strong>{projectCount}</strong>
          </div>
        )}
      </div>

      <section className="panel" style={{ marginBottom: '2rem' }}>
        <div className="section-head">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <Sparkles size={18} color="var(--gold-hi)" />
            <h2>开辟新作品</h2>
          </div>
          <span className="muted">输入作品书名或项目名即可开启</span>
        </div>
        <form className="row" onSubmit={onCreate}>
          <input
            className="grow"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={80}
            placeholder="例如：修仙长篇 · 凡人破虚卷 / 赛博朋克实验集…"
            required
          />
          <button className="btn" type="submit" disabled={busy || !name.trim()}>
            <Plus size={16} />
            {busy ? '正在创建…' : '创建作品'}
          </button>
        </form>
        {formError ? <p className="error" style={{ marginTop: '0.8rem' }}>{formError}</p> : null}
      </section>

      {listError ? (
        <div className="empty-state">
          <h2>作品列表加载失败</h2>
          <p className="error">{listError}</p>
          <button className="btn secondary sm" type="button" onClick={() => void load()} style={{ marginTop: '1rem' }}>
            重试加载
          </button>
        </div>
      ) : projects === null ? (
        <div className="dashboard-grid">
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} h={180} />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <section className="empty-state">
          <Library size={36} color="var(--gold)" style={{ opacity: 0.8, marginBottom: '0.8rem' }} />
          <h2>工坊暂无作品</h2>
          <p className="muted" style={{ maxWidth: '440px', margin: '0 auto 1.5rem' }}>
            先创建一部小说项目，即可上传样章抽取风格卡、融合技法，并开启章节写作。
          </p>
        </section>
      ) : (
        <div className="dashboard-grid">
          {projects.map((p) => (
            <article key={p.id} className="folio">
              <Link to={`/p/${p.id}`} className="folio-main">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="folio-label">NOVEL PROJECT</span>
                  <BookOpen size={16} color="var(--gold)" style={{ opacity: 0.7 }} />
                </div>
                <h3 className="folio-title">{p.name}</h3>
                <p className="folio-meta">创建于 {p.created_at.slice(0, 10)}</p>
              </Link>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: '1.4rem', paddingTop: '0.8rem', borderTop: '1px solid var(--line-dim)' }}>
                <Link to={`/p/${p.id}`} style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--gold-hi)' }}>
                  进入工作台 →
                </Link>
                <button
                  className="btn ghost sm"
                  style={{ color: 'var(--cinnabar-hi)', padding: '0 0.5rem' }}
                  type="button"
                  onClick={() => void onDelete(p.id, p.name)}
                  title="删除作品"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}
