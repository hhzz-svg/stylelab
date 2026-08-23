import type { DragEvent } from 'react'
import { GripVertical, Sparkles } from 'lucide-react'
import { KIND_LABEL, type CardSummary } from '../types'

type Props = {
  card: CardSummary
  active?: boolean
  selected?: boolean
  draggable?: boolean
  actionLabel?: string
  highlight?: string
  onClick: () => void
  onDragStart?: (event: DragEvent<HTMLButtonElement>) => void
}

function HighlightedText({ text, highlight }: { text: string; highlight?: string }) {
  if (!highlight) return <>{text}</>
  const parts = text.split(new RegExp(`(${highlight.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'))
  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === highlight.toLowerCase() ? (
          <mark
            key={i}
            style={{
              background: 'rgba(247, 203, 104, 0.35)',
              color: 'var(--gold-hi)',
              padding: '0 3px',
              borderRadius: '3px',
            }}
          >
            {part}
          </mark>
        ) : (
          part
        ),
      )}
    </>
  )
}

export default function StyleCardTile({
  card,
  active,
  selected,
  draggable,
  actionLabel,
  highlight,
  onClick,
  onDragStart,
}: Props) {
  const className = [
    'style-card-tile',
    'kind-' + card.kind,
    active ? 'active' : '',
    selected ? 'selected' : '',
  ].filter(Boolean).join(' ')

  const glyph = card.kind === 'fused' ? '融' : card.kind === 'manual' ? '调' : '抽'

  return (
    <button
      className={className}
      type="button"
      aria-pressed={selected || active}
      draggable={draggable}
      onDragStart={onDragStart}
      onClick={onClick}
    >
      <span className="style-card-topline">
        <span className="style-card-kind">{KIND_LABEL[card.kind] ?? card.kind}</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <span style={{ fontSize: '0.74rem', fontFamily: 'var(--mono)', color: 'var(--ink-faint)' }}>
            v{card.current_version}
          </span>
          {draggable ? <GripVertical size={14} style={{ opacity: 0.6 }} aria-hidden="true" /> : null}
        </div>
      </span>

      <span className="style-card-glyph" aria-hidden="true">
        {glyph}
      </span>

      <strong className="style-card-name">
        <HighlightedText text={card.name} highlight={highlight} />
      </strong>

      <span className="style-card-meta">
        更新于 {card.updated_at.slice(0, 10)}
      </span>

      <span className="style-card-action">
        <Sparkles size={13} />
        {actionLabel ?? (selected ? '已就位' : '查看卡片')}
      </span>
    </button>
  )
}
