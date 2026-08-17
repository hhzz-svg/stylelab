import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { APIError, api } from '../api'
import type { Job, JobResult } from '../types'

function cardIdFromResult(result: Job['result']): string | undefined {
  if (!result) return undefined
  if (typeof result === 'string') {
    try {
      const parsed = JSON.parse(result) as JobResult
      return parsed.card_id
    } catch {
      return undefined
    }
  }
  return result.card_id
}

type Props = {
  jobId: string
  projectId?: string
}

export default function JobProgress({ jobId, projectId }: Props) {
  const navigate = useNavigate()
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let stopped = false
    async function tick() {
      try {
        const rec = await api.getJob(jobId)
        if (stopped) return
        setJob(rec)
        if (rec.status === 'succeeded') {
          const cardId = cardIdFromResult(rec.result)
          if (cardId && projectId) {
            navigate(`/p/${projectId}/lab/${cardId}`)
          }
          return
        }
        if (rec.status === 'failed' || rec.status === 'canceled') {
          return
        }
        window.setTimeout(() => {
          void tick()
        }, 1000)
      } catch (err) {
        if (stopped) return
        setError(err instanceof APIError ? err.message : '无法读取任务进度')
      }
    }
    void tick()
    return () => {
      stopped = true
    }
  }, [jobId, navigate, projectId])

  if (error) {
    return <p className="error">{error}</p>
  }
  if (!job) {
    return <p className="muted">任务已提交，正在读取进度…</p>
  }

  return (
    <div className="helper">
      <p>
        状态：{job.status} · 进度 {job.progress}%
      </p>
      {job.stage ? <p className="muted">{job.stage}</p> : null}
      {job.error ? <p className="error">{job.error}</p> : null}
    </div>
  )
}
