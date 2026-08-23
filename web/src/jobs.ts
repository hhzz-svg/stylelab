import type { Job, JobResult } from './types'

/**
 * 后台任务登记簿：提交慢任务（抽离/融合/审计/试写）时写入 localStorage，
 * JobDock 据此在任意页面恢复轮询——刷新或跳转后任务不再失联。
 */

export type JobRecord = {
  jobId: string
  projectId: string
  kind: string
  label: string
  startedAt: number
}

const KEY = 'stylelab.jobs'
const MAX_AGE_MS = 24 * 60 * 60 * 1000

function read(): JobRecord[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const list = JSON.parse(raw) as JobRecord[]
    if (!Array.isArray(list)) return []
    const fresh = list.filter(
      (r) => r && typeof r.jobId === 'string' && Date.now() - (r.startedAt ?? 0) < MAX_AGE_MS,
    )
    if (fresh.length !== list.length) {
      localStorage.setItem(KEY, JSON.stringify(fresh))
    }
    return fresh
  } catch {
    return []
  }
}

function write(list: JobRecord[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(list))
  } catch {
    // 存储被禁用或已满：任务坞本次会话内仍可用
  }
  window.dispatchEvent(new CustomEvent('app:jobs'))
}

export function listJobs(): JobRecord[] {
  return read()
}

export function registerJob(rec: JobRecord) {
  write([...read().filter((r) => r.jobId !== rec.jobId), rec])
}

export function removeJob(jobId: string) {
  write(read().filter((r) => r.jobId !== jobId))
}

/** 订阅任务列表变化（本标签页 + 其他标签页）。返回取消函数。 */
export function onJobsChange(cb: () => void): () => void {
  function onStorage(e: StorageEvent) {
    if (e.key === KEY) cb()
  }
  window.addEventListener('app:jobs', cb)
  window.addEventListener('storage', onStorage)
  return () => {
    window.removeEventListener('app:jobs', cb)
    window.removeEventListener('storage', onStorage)
  }
}

export function resultFromJob(result: Job['result']): JobResult {
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

/** 任务结果 → 前端路由。无结果返回空串。 */
export function resultRoute(projectId: string, result: JobResult): string {
  if (result.card_id) return `/p/${projectId}/lab/${result.card_id}`
  if (result.audit_id) return `/p/${projectId}/audit/${result.audit_id}`
  if (result.sample_id) return `/p/${projectId}/sample/${result.sample_id}`
  if (result.chapter_id) return `/p/${projectId}/chapter/${result.chapter_id}`
  return ''
}
