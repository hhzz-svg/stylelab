import { useEffect, useState } from 'react'

/**
 * 全局确认对话框：confirm() 返回 Promise<boolean>，ConfirmHost 挂在应用根部。
 * 风格化替代 window.confirm，支持危险操作样式与 Esc 关闭。
 */

export type ConfirmOptions = {
  title: string
  body?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

type Pending = ConfirmOptions & { resolve: (ok: boolean) => void }

let push: ((p: Pending) => void) | null = null

export function confirm(opts: ConfirmOptions): Promise<boolean> {
  const host = push
  if (!host) {
    // 宿主尚未挂载（极端时序）：退化为原生确认，保证行为不丢
    return Promise.resolve(window.confirm(opts.title))
  }
  return new Promise((resolve) => host({ ...opts, resolve }))
}

function DialogItem({ item, onDone }: { item: Pending; onDone: () => void }) {
  function close(ok: boolean) {
    item.resolve(ok)
    onDone()
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') close(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [item])

  return (
    <div
      className="dialog-scrim"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) close(false)
      }}
    >
      <div className="dialog" role="alertdialog" aria-modal="true" aria-label={item.title}>
        <h2>{item.title}</h2>
        {item.body ? <p className="muted">{item.body}</p> : null}
        <div className="dialog-actions">
          <button className="btn secondary" type="button" onClick={() => close(false)}>
            {item.cancelText ?? '取消'}
          </button>
          <button
            className={`btn${item.danger ? ' danger' : ''}`}
            type="button"
            autoFocus
            onClick={() => close(true)}
          >
            {item.confirmText ?? '确定'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function ConfirmHost() {
  const [items, setItems] = useState<Pending[]>([])

  useEffect(() => {
    push = (p) => setItems((prev) => [...prev, p])
    return () => {
      push = null
    }
  }, [])

  return (
    <>
      {items.map((item, i) => (
        <DialogItem
          key={i}
          item={item}
          onDone={() => setItems((prev) => prev.filter((it) => it !== item))}
        />
      ))}
    </>
  )
}
