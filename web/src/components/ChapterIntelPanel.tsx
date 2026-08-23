import { type MouseEvent, type RefObject, useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { BookMarked, Sparkles, X } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { ChapterIntel } from '../chapterContext'
import {
  DIMENSION_KEYS,
  DIMENSION_LABELS,
  KIND_LABEL,
} from '../types'

const BIBLE_KIND_LABEL: Record<string, string> = {
  character: '人物',
  setting: '设定',
  thread: '伏笔',
}

type Props = {
  projectId: string
  intel: ChapterIntel
  loadingCard?: boolean
  loadingPrev?: boolean
  loadingBible?: boolean
  onRetryCard?: () => void
  onRetryPrev?: () => void
  onRetryBible?: () => void
  variant: 'aside' | 'drawer'
  open?: boolean
  onClose?: () => void
  openerRef?: RefObject<HTMLButtonElement>
  onNavigate?: (to: string) => void | Promise<void>
}

export default function ChapterIntelPanel({
  projectId,
  intel,
  loadingCard,
  loadingPrev,
  loadingBible,
  onRetryCard,
  onRetryPrev,
  onRetryBible,
  variant,
  open = true,
  onClose,
  openerRef,
  onNavigate,
}: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const dialogTitleId = useId()

  useEffect(() => {
    if (variant !== 'drawer') return
    const dialog = dialogRef.current
    if (!dialog) return

    if (open) {
      if (!dialog.open) dialog.showModal()
      closeRef.current?.focus()
    } else if (dialog.open) {
      dialog.close()
    }
  }, [open, variant])

  function closeDialog(restoreFocus: boolean) {
    if (dialogRef.current?.open) dialogRef.current.close()
    onClose?.()
    if (restoreFocus && openerRef?.current?.isConnected) openerRef.current.focus()
  }

  function handleNavigate(event: MouseEvent<HTMLAnchorElement>, to: string) {
    if (
      !onNavigate
      || event.defaultPrevented
      || event.button !== 0
      || event.metaKey
      || event.ctrlKey
      || event.shiftKey
      || event.altKey
    ) return

    event.preventDefault()
    if (variant === 'drawer') closeDialog(false)
    void onNavigate(to)
  }

  const cardsPath = '/p/' + projectId + '/cards'
  const biblePath = '/p/' + projectId + '/bible'
  const card = intel.card

  const body = (
    <div className="chapter-intel">
      {/* Style Card Intel Section */}
      <section className="work-zone">
        <div className="zone-heading">
          <span className="zone-index">技</span>
          <div>
            <h2>风格卡技法</h2>
            <p>九维指标与行文约束</p>
          </div>
        </div>
        {loadingCard ? (
          <p className="muted">正在读取风格卡技法…</p>
        ) : intel.cardError ? (
          <div className="stack">
            <p className="error">{intel.cardError}</p>
            {onRetryCard ? (
              <button className="btn secondary sm" type="button" onClick={onRetryCard}>重试</button>
            ) : null}
          </div>
        ) : !card ? (
          <div style={{ padding: '0.8rem', background: 'rgba(255,255,255,0.03)', borderRadius: 'var(--radius)', border: '1px dashed var(--line-glass)' }}>
            <p className="muted" style={{ margin: '0 0 0.5rem', fontSize: '0.82rem' }}>
              当前章节尚未绑定风格卡。
            </p>
            <Link className="btn secondary sm" to={cardsPath} onClick={(event) => handleNavigate(event, cardsPath)}>
              <Sparkles size={14} /> 去牌库绑定
            </Link>
          </div>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0.5rem 0.8rem', background: 'rgba(247,203,104,0.08)', borderRadius: 'var(--radius)', border: '1px solid rgba(247,203,104,0.2)' }}>
              <div>
                <span style={{ fontSize: '0.72rem', color: 'var(--gold-hi)', fontWeight: 700, textTransform: 'uppercase' }}>
                  {KIND_LABEL[card.kind] ?? card.kind} · v{card.version}
                </span>
                <div style={{ fontWeight: 700, color: 'var(--ink)', fontSize: '0.95rem' }}>{card.name}</div>
              </div>
              <Link
                className="btn secondary sm"
                to={'/p/' + projectId + '/lab/' + card.id}
                onClick={(event) => handleNavigate(event, '/p/' + projectId + '/lab/' + card.id)}
                style={{ padding: '0 0.6rem', height: '1.8rem', fontSize: '0.75rem' }}
              >
                实验室
              </Link>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '0.4rem', marginTop: '0.4rem' }}>
              {DIMENSION_KEYS.map((key) => {
                const level = card.dimensions?.[key]?.level ?? 0
                return (
                  <div
                    key={key}
                    style={{
                      padding: '0.4rem 0.5rem',
                      background: 'rgba(255,255,255,0.03)',
                      border: '1px solid var(--line-glass)',
                      borderRadius: 'var(--radius-sm)',
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                    }}
                  >
                    <span style={{ fontSize: '0.68rem', color: 'var(--ink-faint)' }}>{DIMENSION_LABELS[key]}</span>
                    <strong style={{ fontSize: '0.9rem', color: 'var(--gold-hi)', fontFamily: 'var(--mono)' }}>{level}</strong>
                  </div>
                )
              })}
            </div>

            {card.prohibitions?.length ? (
              <div style={{ marginTop: '0.6rem' }}>
                <span style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--cinnabar-hi)' }}>禁忌与约束</span>
                <ul style={{ margin: '0.3rem 0 0', paddingLeft: '1.2rem', fontSize: '0.8rem', color: 'var(--ink-soft)' }}>
                  {card.prohibitions.map((item) => <li key={item}>{item}</li>)}
                </ul>
              </div>
            ) : null}
          </>
        )}
      </section>

      {/* Previous Chapter Tail Section */}
      <section className="work-zone">
        <div className="zone-heading">
          <span className="zone-index">前</span>
          <div>
            <h2>前情承接</h2>
            <p>上章剧情摘要与文末锚点</p>
          </div>
        </div>
        {loadingPrev ? (
          <p className="muted">正在读取上一章…</p>
        ) : intel.prevError ? (
          <div className="stack">
            <p className="error">{intel.prevError}</p>
            {onRetryPrev ? (
              <button className="btn secondary sm" type="button" onClick={onRetryPrev}>重试</button>
            ) : null}
          </div>
        ) : !intel.prev ? (
          <p className="muted" style={{ fontSize: '0.85rem' }}>此为开篇第一章，无前情承接。</p>
        ) : (
          <>
            <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--gold-hi)' }}>
              第 {intel.prev.seq} 章 · {intel.prev.title}
            </div>
            <p style={{ fontSize: '0.84rem', color: 'var(--ink-soft)', lineHeight: 1.6, margin: '0.2rem 0' }}>
              {intel.prevSummary || '尚未生成章节摘要。'}
            </p>
            {intel.prevTail ? (
              <div
                style={{
                  padding: '0.6rem 0.8rem',
                  background: 'rgba(11,15,23,0.7)',
                  borderLeft: '3px solid var(--gold)',
                  borderRadius: '0 var(--radius-sm) var(--radius-sm) 0',
                  fontFamily: 'var(--font-serif)',
                  fontSize: '0.82rem',
                  color: 'var(--ink)',
                  lineHeight: 1.7,
                  maxHeight: '120px',
                  overflowY: 'auto',
                }}
              >
                {intel.prevTail}
              </div>
            ) : null}
          </>
        )}
      </section>

      {/* Story Bible Lore Section */}
      <section className="work-zone">
        <div className="zone-heading">
          <span className="zone-index">志</span>
          <div>
            <h2>设定集世界观</h2>
            <p>登场人物、世界观与伏笔</p>
          </div>
        </div>
        {loadingBible ? (
          <p className="muted">正在读取设定集…</p>
        ) : intel.bibleError ? (
          <div className="stack">
            <p className="error">{intel.bibleError}</p>
            {onRetryBible ? (
              <button className="btn secondary sm" type="button" onClick={onRetryBible}>重试</button>
            ) : null}
          </div>
        ) : !intel.bible.length ? (
          <div style={{ padding: '0.8rem', background: 'rgba(255,255,255,0.03)', borderRadius: 'var(--radius)', border: '1px dashed var(--line-glass)' }}>
            <p className="muted" style={{ margin: '0 0 0.5rem', fontSize: '0.82rem' }}>
              尚未录入设定集条目。
            </p>
            <Link className="btn secondary sm" to={biblePath} onClick={(event) => handleNavigate(event, biblePath)}>
              <BookMarked size={14} /> 去设定集登记
            </Link>
          </div>
        ) : (
          <>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.45rem', maxHeight: '200px', overflowY: 'auto' }}>
              {intel.bible.map((entry) => (
                <div
                  key={entry.id}
                  style={{
                    padding: '0.45rem 0.65rem',
                    background: 'rgba(255,255,255,0.03)',
                    border: '1px solid var(--line-glass)',
                    borderRadius: 'var(--radius-sm)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '0.2rem',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                      <span className={`bible-badge ${entry.kind === 'character' ? 'badge-char' : entry.kind === 'setting' ? 'badge-setting' : 'badge-thread'}`}>
                        {BIBLE_KIND_LABEL[entry.kind] ?? entry.kind}
                      </span>
                      <strong style={{ fontSize: '0.85rem', color: 'var(--ink)' }}>{entry.name}</strong>
                    </div>
                    {entry.kind === 'thread' && entry.status === 'resolved' && (
                      <span style={{ fontSize: '0.7rem', color: 'var(--ink-faint)' }}>已收线</span>
                    )}
                  </div>
                  {entry.content_preview ? (
                    <span style={{ fontSize: '0.76rem', color: 'var(--ink-soft)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {entry.content_preview}
                    </span>
                  ) : null}
                </div>
              ))}
            </div>
            <Link
              className="btn secondary sm"
              to={biblePath}
              onClick={(event) => handleNavigate(event, biblePath)}
              style={{ marginTop: '0.4rem' }}
            >
              <BookMarked size={14} /> 打开设定集全览
            </Link>
          </>
        )}
      </section>
    </div>
  )

  if (variant !== 'drawer') {
    return <aside style={{ position: 'sticky', top: '1.5rem' }} aria-label="风格卡和前情">{body}</aside>
  }

  return createPortal(
    <dialog
      ref={dialogRef}
      style={{
        background: 'rgba(11,15,23,0.96)',
        backdropFilter: 'blur(28px)',
        border: '1px solid var(--line-hi)',
        borderRadius: 'var(--radius-xl)',
        padding: '1.8rem',
        maxWidth: '520px',
        width: '90%',
        color: 'var(--ink)',
        boxShadow: 'var(--shadow-lg)',
      }}
      aria-labelledby={dialogTitleId}
      onCancel={(event) => {
        event.preventDefault()
        closeDialog(true)
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) closeDialog(true)
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.2rem', paddingBottom: '0.8rem', borderBottom: '1px solid var(--line-glass)' }}>
        <div>
          <span className="kicker">INTEL PANEL</span>
          <h2 id={dialogTitleId} style={{ margin: '0.2rem 0 0', fontFamily: 'var(--font-serif)' }}>智慧写作侧舱</h2>
        </div>
        <button
          ref={closeRef}
          className="btn ghost sm"
          type="button"
          onClick={() => closeDialog(true)}
          aria-label="关闭智慧侧舱"
          title="关闭"
        >
          <X size={18} />
        </button>
      </div>
      <div style={{ maxHeight: '70vh', overflowY: 'auto' }}>
        {body}
      </div>
    </dialog>,
    document.body,
  )
}
