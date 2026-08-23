import { useEffect, useRef, useState } from 'react'
import { api } from './api'
import type { Job, JobStatus } from './types'

/** 每页设置文档标题。 */
export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} · 风格工坊`
    return () => {
      document.title = '风格工坊 · Style Lab'
    }
  }, [title])
}

/**
 * 未保存守卫：dirty 时拦刷新/关页。
 * BrowserRouter 不是 data router，useBlocker 会直接抛错，所以路由跳转不在这里拦。
 */
export function useDirtyGuard(dirty: boolean) {
  const dirtyRef = useRef(dirty)
  dirtyRef.current = dirty

  useEffect(() => {
    function onBeforeUnload(e: BeforeUnloadEvent) {
      if (dirtyRef.current) {
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [])
}

const POLL_MS = 1000
const MAX_GET_FAILURES = 3

/**
 * 任务轮询：1s 一次，容忍 3 次连续失败，终态（succeeded/failed/canceled）即停。
 * 返回最近一次任务快照与轮询错误。
 */
export function useJobPoll(jobId: string | undefined, onStatus?: (status: JobStatus) => void) {
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState('')
  const onStatusRef = useRef(onStatus)
  onStatusRef.current = onStatus

  useEffect(() => {
    if (!jobId) return
    const id = jobId
    let stopped = false
    let timer: number | undefined
    let failures = 0

    async function tick() {
      try {
        const rec = await api.getJob(id)
        if (stopped) return
        failures = 0
        setError('')
        setJob(rec)
        onStatusRef.current?.(rec.status)
        if (rec.status === 'succeeded' || rec.status === 'failed' || rec.status === 'canceled') {
          return
        }
        timer = window.setTimeout(() => {
          void tick()
        }, POLL_MS)
      } catch {
        if (stopped) return
        failures += 1
        if (failures < MAX_GET_FAILURES) {
          timer = window.setTimeout(() => {
            void tick()
          }, POLL_MS)
          return
        }
        setError('无法读取任务进度，请检查网络后刷新')
        onStatusRef.current?.('failed')
      }
    }

    setJob(null)
    setError('')
    void tick()
    return () => {
      stopped = true
      if (timer !== undefined) window.clearTimeout(timer)
    }
  }, [jobId])

  return { job, error }
}
