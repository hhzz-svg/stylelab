import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { BookOpen, FlaskConical, PenTool, Sparkles } from 'lucide-react'
import { APIError, api, notify } from '../api'
import Crumb from '../components/Crumb'
import JobProgress from '../components/JobProgress'
import Skeleton from '../components/Skeleton'
import { usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import type { JobStatus, SampleChapter } from '../types'

export default function Sample() {
  const { id, sampleId } = useParams()
  const projectId = id ?? ''
  const [sample, setSample] = useState<SampleChapter | null>(null)
  const [premise, setPremise] = useState('')
  const [target, setTarget] = useState(1200)
  const [model, setModel] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState('')
  const [jobActive, setJobActive] = useState(false)
  usePageTitle('试写样章')

  function onJobStatus(status: JobStatus) {
    setJobActive(status === 'queued' || status === 'running')
  }

  useEffect(() => {
    if (!sampleId) return
    api
      .getSample(sampleId)
      .then((data) => {
        setSample(data)
        setPremise(data.premise)
      })
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '加载试写失败')
      })
  }, [sampleId])

  async function onRetry(e: FormEvent) {
    e.preventDefault()
    if (!sample?.card_id) return
    const text = premise.trim()
    if ([...text].length === 0 || [...text].length > 80) {
      setError('试写前提必填且不超过 80 字')
      return
    }
    setError('')
    setBusy(true)
    try {
      const res = await api.sampleCard(sample.card_id, text, target, model.trim())
      setJobId(res.job_id)
      setJobActive(true)
      registerJob({ jobId: res.job_id, projectId, kind: 'sample', label: '试写样章', startedAt: Date.now() })
      notify('试写任务已提交，后台生成中', 'info')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '试写提交失败')
    } finally {
      setBusy(false)
    }
  }

  const factsText = sample ? JSON.stringify(sample.facts ?? {}, null, 2) : ''
  const premiseCount = [...premise].length

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current="试写样章" />
      <div className="page-head">
        <div>
          <p className="kicker">SAMPLE CHAMBER</p>
          <h1>试写样章</h1>
          <p className="sub">用当前风格卡试写一段正文，实地检阅九维技法配置在正文中的流淌质感。</p>
        </div>
        {sample?.card_id ? (
          <Link className="btn secondary" to={`/p/${projectId}/lab/${sample.card_id}`}>
            <FlaskConical size={16} />
            回到风格实验室
          </Link>
        ) : null}
      </div>

      {error ? (
        <div className="empty-state">
          <h2>试写加载失败</h2>
          <p className="error">{error}</p>
        </div>
      ) : !sample ? (
        <div className="card">
          <Skeleton h={32} w="30%" />
          <div style={{ display: 'grid', gap: '1rem', marginTop: '1.2rem' }}>
            <Skeleton h={80} />
            <Skeleton h={360} />
          </div>
        </div>
      ) : (
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.8rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingBottom: '0.8rem', borderBottom: '1px solid var(--line-glass)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <Sparkles size={18} color="var(--gold-hi)" />
              <span style={{ fontWeight: 600, color: 'var(--ink)' }}>
                风格卡 {sample.card_id.slice(0, 8)}…
              </span>
              <span className="chapter-seq-badge">版本 v{sample.card_version}</span>
            </div>
          </div>

          <form className="stack" onSubmit={onRetry}>
            <label>
              试写前提与情节起笔（{premiseCount}/80）
              <textarea
                value={premise}
                rows={3}
                maxLength={80}
                onChange={(e) => setPremise(e.target.value)}
                placeholder="例如：深夜古刹，断烛明灭，老僧拭剑…"
              />
            </label>
            <div className="row" style={{ flexWrap: 'wrap' }}>
              <label style={{ flex: '1 1 200px' }}>
                目标字数：<span style={{ color: 'var(--gold-hi)', fontFamily: 'var(--mono)', fontWeight: 700 }}>{target}</span>
                <input
                  type="range"
                  min={800}
                  max={2000}
                  step={50}
                  value={target}
                  onChange={(e) => setTarget(Number(e.target.value))}
                />
              </label>
              <label style={{ flex: '1 1 200px' }}>
                模型（可选）
                <input
                  type="text"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  placeholder="留空使用默认模型"
                />
              </label>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button className="btn" type="submit" disabled={busy || jobActive}>
                <PenTool size={16} />
                {busy || jobActive ? '正在运笔生成中…' : '重新试写一篇'}
              </button>
            </div>
          </form>

          {jobId ? <JobProgress jobId={jobId} projectId={projectId} onStatus={onJobStatus} /> : null}

          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', marginBottom: '1rem' }}>
              <BookOpen size={18} color="var(--gold-hi)" />
              <h2 style={{ margin: 0, fontFamily: 'var(--font-serif)' }}>试写正文</h2>
            </div>
            <pre className="sample-body">{sample.body}</pre>
          </div>

          <details className="facts-fold">
            <summary style={{ cursor: 'pointer', fontWeight: 600, color: 'var(--gold-hi)' }}>
              Stylestat 风格质地统计事实数据
            </summary>
            <pre className="facts-json">{factsText}</pre>
          </details>
        </div>
      )}
    </div>
  )
}
