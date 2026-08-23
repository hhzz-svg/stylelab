import { useEffect } from 'react'
import { Keyboard, X } from 'lucide-react'

type ShortcutGroup = {
  title: string
  shortcuts: { keys: string[]; label: string }[]
}

const SHORTCUTS: ShortcutGroup[] = [
  {
    title: '全局导航与搜索',
    shortcuts: [
      { keys: ['Ctrl', 'K'], label: '打开全局快速指令台 / 搜索' },
      { keys: ['?'], label: '显示 / 隐藏此快捷键面板' },
      { keys: ['Esc'], label: '关闭弹窗 / 抽屉' },
    ],
  },
  {
    title: '小说写作台',
    shortcuts: [
      { keys: ['Ctrl', 'S'], label: '保存当前章节修改' },
      { keys: ['Ctrl', 'Enter'], label: '触发 AI 章节生成 / 重写' },
      { keys: ['Ctrl', '['], label: '跳转上一章' },
      { keys: ['Ctrl', ']'], label: '跳转下一章' },
    ],
  },
  {
    title: '风格工坊与熔炉',
    shortcuts: [
      { keys: ['Ctrl', 'Enter'], label: '开始样章风格抽离' },
      { keys: ['Ctrl', 'S'], label: '保存风格卡版本修改' },
    ],
  },
]

type HelpDialogProps = {
  onClose: () => void
}

export default function HelpDialog({ onClose }: HelpDialogProps) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="cmd-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div className="cmd-modal" style={{ maxWidth: '520px' }} onClick={(e) => e.stopPropagation()}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '1.2rem 1.5rem', borderBottom: '1px solid var(--line-glass)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <Keyboard size={20} color="var(--gold-hi)" />
            <h2 style={{ margin: 0, fontFamily: 'var(--font-serif)', fontSize: '1.25rem', color: 'var(--ink)' }}>
              快捷键指令指南
            </h2>
          </div>
          <button className="btn ghost sm" type="button" onClick={onClose} aria-label="关闭">
            <X size={18} />
          </button>
        </div>

        <div style={{ padding: '1.2rem 1.5rem', display: 'flex', flexDirection: 'column', gap: '1.4rem', maxHeight: '60vh', overflowY: 'auto' }}>
          {SHORTCUTS.map((group) => (
            <div key={group.title}>
              <h3 style={{ margin: '0 0 0.6rem', fontSize: '0.8rem', color: 'var(--gold)', letterSpacing: '0.1em', textTransform: 'uppercase' }}>
                {group.title}
              </h3>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {group.shortcuts.map((s, i) => (
                  <div
                    key={i}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '0.5rem 0.8rem',
                      background: 'rgba(255,255,255,0.03)',
                      border: '1px solid var(--line-glass)',
                      borderRadius: 'var(--radius-sm)',
                    }}
                  >
                    <span style={{ fontSize: '0.88rem', color: 'var(--ink)' }}>{s.label}</span>
                    <div style={{ display: 'flex', gap: '0.3rem' }}>
                      {s.keys.map((k, ki) => (
                        <kbd
                          key={ki}
                          style={{
                            padding: '0.15rem 0.45rem',
                            background: 'rgba(247,203,104,0.12)',
                            border: '1px solid rgba(247,203,104,0.3)',
                            borderRadius: '4px',
                            color: 'var(--gold-hi)',
                            fontSize: '0.76rem',
                            fontFamily: 'var(--mono)',
                            fontWeight: 600,
                          }}
                        >
                          {k}
                        </kbd>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div
          style={{
            padding: '0.8rem 1.5rem',
            borderTop: '1px solid var(--line-glass)',
            background: 'rgba(7,10,15,0.7)',
            fontSize: '0.78rem',
            color: 'var(--ink-faint)',
            textAlign: 'center',
          }}
        >
          按 <kbd style={{ color: 'var(--gold-hi)' }}>?</kbd> 或 <kbd style={{ color: 'var(--gold-hi)' }}>Esc</kbd> 随时开启或退出
        </div>
      </div>
    </div>
  )
}
