import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { notify, type ToastAction } from '../api'

type ToastItem = {
  id: number
  message: string
  kind: 'success' | 'error' | 'info'
  action?: ToastAction
}

const DISMISS_MS = 4200
const MAX_TOASTS = 4

let seq = 0

function Toast({ item, onDismiss }: { item: ToastItem; onDismiss: (id: number) => void }) {
  const [paused, setPaused] = useState(false)

  useEffect(() => {
    if (paused) return
    const timer = window.setTimeout(() => onDismiss(item.id), DISMISS_MS)
    return () => window.clearTimeout(timer)
  }, [paused, item.id, onDismiss])

  return (
    <div
      className={`toast ${item.kind}`}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
    >
      <span className="toast-msg">{item.message}</span>
      {item.action ? (
        <Link className="toast-action" to={item.action.to}>
          {item.action.label}
        </Link>
      ) : null}
      <button className="toast-x" type="button" aria-label="关闭" onClick={() => onDismiss(item.id)}>
        ×
      </button>
    </div>
  )
}

/** 渲染全局 toast 栈。消息由 api.notify() 通过事件广播，组件零依赖。 */
export default function ToastHost() {
  const [items, setItems] = useState<ToastItem[]>([])

  useEffect(() => {
    function onToast(e: Event) {
      const detail = (e as CustomEvent).detail as {
        message?: string
        kind?: ToastItem['kind']
        action?: ToastAction
      }
      const message = detail?.message
      if (!message) return
      const kind: ToastItem['kind'] = detail?.kind ?? 'info'
      const id = ++seq
      setItems((prev) => [...prev.slice(-(MAX_TOASTS - 1)), { id, message, kind, action: detail?.action }])
    }
    window.addEventListener('app:toast', onToast)
    return () => window.removeEventListener('app:toast', onToast)
  }, [])

  const dismiss = useRef((id: number) => {
    setItems((prev) => prev.filter((t) => t.id !== id))
  }).current

  return (
    <div className="toast-stack" aria-live="polite">
      {items.map((t) => (
        <Toast key={t.id} item={t} onDismiss={dismiss} />
      ))}
    </div>
  )
}

export { notify }
