import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { notify } from '../api'
import { isClaimed, listJobs, onJobsChange, removeJob, resultFromJob, resultRoute, type JobRecord } from '../jobs'
import { useJobPoll } from '../hooks'

/**
 * 任务坞：登录后常驻右下角。所有慢任务提交时写入 jobsStore，
 * 刷新或跳转后这里继续轮询，完成时 toast 提醒并给出跳转。
 */

// Longer than one poll, so an open launching page has seen the job finish
// and claimed it before the dock decides whether to toast.
const CLAIM_GRACE_MS = 1500

function DockItem({ rec }: { rec: JobRecord }) {
  const { job, error } = useJobPoll(rec.jobId)
  const handled = useRef(false)

  useEffect(() => {
    if (!job || handled.current) return
    if (job.status !== 'succeeded' && job.status !== 'failed' && job.status !== 'canceled') return
    handled.current = true
    if (job.status === 'succeeded') {
      const to = resultRoute(rec.projectId, resultFromJob(job.result))
      // Deferred, and it outlives this item: removeJob below unmounts it.
      window.setTimeout(() => {
        // 发起页仍开着、已自己提醒并展示结果
        if (isClaimed(rec.jobId)) return
        if (to) {
          // 已在结果页（发起页的进度条刚跳过来）就不重复提醒
          if (window.location.pathname !== to) {
            notify(`${rec.label}完成`, 'success', { label: '查看', to })
          }
        } else {
          // 结果即数据的任务（大纲/雷达/推演/续写）没有结果页可跳。
          // 仍然要提醒，否则它会从任务坞里悄无声息地消失。
          notify(`${rec.label}完成`, 'success')
        }
      }, CLAIM_GRACE_MS)
    } else if (job.status === 'failed') {
      notify(`${rec.label}失败：${job.error || '未知原因'}`, 'error')
    }
    removeJob(rec.jobId)
  }, [job, rec.jobId, rec.label, rec.projectId])

  const pct = job ? Math.max(3, Math.min(100, job.progress)) : 6
  const failed = job?.status === 'failed' || job?.status === 'canceled' || !!error

  return (
    <div className={`dock-card${failed ? ' failed' : ''}`}>
      <div className="dock-head">
        <Link to={`/p/${rec.projectId}`}>{rec.label}</Link>
        <button
          className="dock-x"
          type="button"
          aria-label="隐藏此任务"
          title="隐藏（任务仍在后台运行）"
          onClick={() => removeJob(rec.jobId)}
        >
          ×
        </button>
      </div>
      <p className="dock-stage">{error || job?.error || job?.stage || job?.status || '排队中…'}</p>
      <div className="dock-track">
        <div className={failed ? 'dock-fill err' : 'dock-fill'} style={{ width: `${failed ? 100 : pct}%` }} />
      </div>
    </div>
  )
}

export default function JobDock() {
  const [jobs, setJobs] = useState(listJobs())

  useEffect(() => onJobsChange(() => setJobs(listJobs())), [])

  if (jobs.length === 0) return null

  return (
    <div className="job-dock" aria-label="进行中的任务">
      {jobs.map((rec) => (
        <DockItem key={rec.jobId} rec={rec} />
      ))}
    </div>
  )
}
