import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { claimJob, resultFromJob, resultRoute } from '../jobs'
import { useJobPoll } from '../hooks'
import type { Job, JobResult, JobStatus } from '../types'

type Props = {
  jobId: string
  projectId?: string
  onStatus?: (status: JobStatus) => void
  autoNavigate?: boolean
  /** `job` is passed too: the studio kinds return their data as the result
   *  rather than an id, so callers read job.result with their own type. */
  onSucceeded?: (result: JobResult, job: Job) => void
  onPollError?: (error: string) => void
}

/** 发起页内嵌的任务进度条：默认完成后跳转，也可由发起页原位接管结果。 */
export default function JobProgress({
  jobId,
  projectId,
  onStatus,
  autoNavigate = true,
  onSucceeded,
  onPollError,
}: Props) {
  const navigate = useNavigate()
  const reportPollErrors = onPollError !== undefined
  const { job, error } = useJobPoll(jobId, reportPollErrors ? undefined : onStatus)
  const handled = useRef('')
  const onStatusRef = useRef(onStatus)
  const onPollErrorRef = useRef(onPollError)
  onStatusRef.current = onStatus
  onPollErrorRef.current = onPollError

  useEffect(() => {
    if (reportPollErrors && job) onStatusRef.current?.(job.status)
  }, [job, reportPollErrors])

  useEffect(() => {
    if (reportPollErrors && error) onPollErrorRef.current?.(error)
  }, [error, reportPollErrors])

  useEffect(() => {
    if (job?.status !== 'succeeded' || handled.current === job.id) return
    handled.current = job.id
    const result = resultFromJob(job.result)
    if (onSucceeded) {
      // This page shows the result and toasts it; the dock need not.
      claimJob(job.id)
      onSucceeded(result, job)
    }
    if (!autoNavigate || !projectId) return
    const to = resultRoute(projectId, result)
    if (to) navigate(to)
  }, [job, projectId, autoNavigate, onSucceeded, navigate])

  if (error) {
    return <p className="error">{error}</p>
  }
  if (!job) {
    return (
      <div className="job-bar">
        <p>任务已提交…</p>
        <div className="job-track">
          <div className="job-fill indet" />
        </div>
      </div>
    )
  }

  return (
    <div className="job-bar">
      <p>
        {job.stage || job.status} · {job.progress}%
      </p>
      <div className="job-track">
        <div className="job-fill" style={{ width: `${Math.max(4, Math.min(100, job.progress))}%` }} />
      </div>
      {job.error ? <p className="error">{job.error}</p> : null}
    </div>
  )
}
