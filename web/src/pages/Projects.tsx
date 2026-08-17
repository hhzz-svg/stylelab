import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { APIError, api } from '../api'
import type { Project } from '../types'

export default function Projects() {
  const [projects, setProjects] = useState<Project[]>([])
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      const data = await api.listProjects()
      setProjects(data.projects ?? [])
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const created = await api.createProject(name.trim())
      setName('')
      setProjects((prev) => [created, ...prev])
    } catch (err) {
      setError(err instanceof APIError ? err.message : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      <div className="card">
        <h1>项目</h1>
        <form className="stack" onSubmit={onCreate}>
          <label>
            新项目名称
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={80}
              required
            />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <button className="btn" type="submit" disabled={busy}>
            {busy ? '创建中…' : '创建项目'}
          </button>
        </form>
      </div>
      <div className="card">
        <h2>我的项目</h2>
        {projects.length === 0 ? (
          <p className="muted">还没有项目。先创建一个，再上传文本抽离风格。</p>
        ) : (
          <ul className="list">
            {projects.map((p) => (
              <li key={p.id}>
                <Link to={`/p/${p.id}`}>{p.name}</Link>
                <span className="muted">{p.created_at}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
