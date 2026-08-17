import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { APIError, api } from '../api'
import type { Job, JobResult, JobStatus } from '../types'

function resultFromJob(result: Job['result']): JobResult {
  if (!result) return {}
  if (typeof result === 'string') {
    try {
      return JSON.parse(result) as JobResult
    } catch {
      return {}
    }
  }
  return result
}

type Props = {
  jobId: string
  projectId?: string
  onStatus?: (status: JobStatus) => void
}

const POLL_MS = 1000
const MAX_GET_FAILURES = 3

export default function JobProgress({ jobId, projectId, onStatus }: Props) {
  const navigate = useNavigate()
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState('')
  const onStatusRef = useRef(onStatus)
  onStatusRef.current = onStatus

  useEffect(() => {
    let stopped = false
    let timer: number | undefined
    let failures = 0

    async function tick() {
      try {
        const rec = await api.getJob(jobId)
        if (stopped) return
        failures = 0
        setError('')
        setJob(rec)
        onStatusRef.current?.(rec.status)
        if (rec.status === 'succeeded') {
          const result = resultFromJob(rec.result)
          if (projectId && result.card_id) {
            navigate(`/p/${projectId}/lab/${result.card_id}`)
            return
          }
          if (projectId && result.audit_id) {
            navigate(`/p/${projectId}/audit/${result.audit_id}`)
            return
          }
          if (projectId && result.sample_id) {
            navigate(`/p/${projectId}/sample/${result.sample_id}`)
            return
          }
          return
        }
        if (rec.status === 'failed' || rec.status === 'canceled') {
          return
        }
        timer = window.setTimeout(() => {
          void tick()
        }, POLL_MS)
      } catch (err) {
        if (stopped) return
        failures += 1
        if (failures < MAX_GET_FAILURES) {
          timer = window.setTimeout(() => {
            void tick()
          }, POLL_MS)
          return
        }
        setError(err instanceof APIError ? err.message : '无法读取任务进度')
        onStatusRef.current?.('failed')
      }
    }
    void tick()
    return () => {
      stopped = true
      if (timer !== undefined) window.clearTimeout(timer)
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
