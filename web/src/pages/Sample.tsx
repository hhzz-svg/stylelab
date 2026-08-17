import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { APIError, api } from '../api'
import JobProgress from '../components/JobProgress'
import type { SampleChapter } from '../types'

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
      <p>
        <Link to={`/p/${projectId}`}>← 返回项目</Link>
        {sample?.card_id ? (
          <>
            {' · '}
            <Link to={`/p/${projectId}/lab/${sample.card_id}`}>打开工坊</Link>
          </>
        ) : null}
      </p>
      <div className="card">
        <h1>试写样本</h1>
        {error ? <p className="error">{error}</p> : null}
        {!sample && !error ? <p className="muted">正在加载样本…</p> : null}
        {sample ? (
          <>
            <p className="muted">
              卡片 {sample.card_id} · v{sample.card_version}
            </p>
            <form className="stack" onSubmit={onRetry}>
              <label>
                试写前提（{premiseCount}/80）
                <textarea
                  value={premise}
                  rows={3}
                  maxLength={240}
                  onChange={(e) => setPremise(e.target.value)}
                />
              </label>
              <label>
                目标字数 {target}
                <input
                  type="range"
                  min={800}
                  max={2000}
                  step={50}
                  value={target}
                  onChange={(e) => setTarget(Number(e.target.value))}
                />
              </label>
              <label>
                模型（可选）
                <input type="text" value={model} onChange={(e) => setModel(e.target.value)} />
              </label>
              <button className="btn" type="submit" disabled={busy || !!jobId}>
                {busy ? '提交中…' : '再写一篇'}
              </button>
            </form>
            {jobId ? <JobProgress jobId={jobId} projectId={projectId} /> : null}
            <h2>正文</h2>
            <pre className="sample-body">{sample.body}</pre>
            <h2>事实 JSON</h2>
            <pre className="facts-json">{factsText}</pre>
          </>
        ) : null}
      </div>
    </div>
  )
}
