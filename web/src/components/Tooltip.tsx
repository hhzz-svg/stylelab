import { type ReactNode, useRef, useState } from 'react'

type TooltipProps = {
  label: string
  shortcut?: string
  children: ReactNode
  position?: 'top' | 'bottom' | 'left' | 'right'
}

export function Tooltip({ label, shortcut, children, position = 'top' }: TooltipProps) {
  const [visible, setVisible] = useState(false)
  const timeoutRef = useRef<number | undefined>()
  const triggerRef = useRef<HTMLSpanElement>(null)

  function show() {
    timeoutRef.current = window.setTimeout(() => setVisible(true), 400)
  }

  function hide() {
    if (timeoutRef.current) clearTimeout(timeoutRef.current)
    setVisible(false)
  }

  return (
    <span
      ref={triggerRef}
      style={{ position: 'relative', display: 'inline-flex' }}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
    >
      {children}
      {visible && (
        <span
          className="tooltip-bubble"
          role="tooltip"
          style={{
            position: 'absolute',
            ...(position === 'top' ? { bottom: 'calc(100% + 6px)', left: '50%', transform: 'translateX(-50%)' } : {}),
            ...(position === 'bottom' ? { top: 'calc(100% + 6px)', left: '50%', transform: 'translateX(-50%)' } : {}),
            ...(position === 'left' ? { right: 'calc(100% + 6px)', top: '50%', transform: 'translateY(-50%)' } : {}),
            ...(position === 'right' ? { left: 'calc(100% + 6px)', top: '50%', transform: 'translateY(-50%)' } : {}),
            whiteSpace: 'nowrap',
            zIndex: 50,
          }}
        >
          {label}
          {shortcut ? <kbd style={{ marginLeft: '0.4rem', padding: '0.1rem 0.3rem', background: 'rgba(242,234,217,0.1)', borderRadius: '4px', fontSize: '0.72rem' }}>{shortcut}</kbd> : null}
        </span>
      )}
    </span>
  )
}
