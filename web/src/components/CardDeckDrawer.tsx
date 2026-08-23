import type { DragEvent } from 'react'
import { Library, X } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import StyleCardTile from './StyleCardTile'
import type { CardSummary } from '../types'

const CARD_DRAG_TYPE = 'application/x-stylelab-card'

type Props = {
  open: boolean
  projectId: string
  cards: CardSummary[]
  selectedIds?: string[]
  onToggle?: (cardId: string) => void
  onClose: () => void
}

export default function CardDeckDrawer({
  open,
  projectId,
  cards,
  selectedIds = [],
  onToggle,
  onClose,
}: Props) {
  const navigate = useNavigate()
  if (!open) return null

  return (
    <>
      <button className="deck-scrim" type="button" onClick={onClose} aria-label="关闭牌库" />
      <aside className="deck-drawer" aria-label="牌库抽屉">
        <div className="deck-drawer-head">
          <div>
            <span className="eyebrow"><Library size={14} />CARD DECK</span>
            <h2>牌库</h2>
          </div>
          <button className="icon-btn" type="button" onClick={onClose} aria-label="关闭牌库" title="关闭">
            <X size={18} />
          </button>
        </div>
        {cards.length ? (
          <div className="deck-drawer-grid">
            {cards.map((card) => (
              <StyleCardTile
                key={card.id}
                card={card}
                selected={selectedIds.includes(card.id)}
                draggable={!!onToggle}
                actionLabel={onToggle ? (selectedIds.includes(card.id) ? '移出融合槽' : '加入融合槽') : '进入实验室'}
                onDragStart={(event: DragEvent<HTMLButtonElement>) => {
                  event.dataTransfer.setData(CARD_DRAG_TYPE, card.id)
                  event.dataTransfer.setData('text/plain', card.id)
                  event.dataTransfer.effectAllowed = 'copy'
                }}
                onClick={() => {
                  if (onToggle) onToggle(card.id)
                  else navigate('/p/' + projectId + '/lab/' + card.id)
                }}
              />
            ))}
          </div>
        ) : <p className="muted">牌库还是空的，先抽离一张风格卡。</p>}
        <button className="btn secondary" type="button" onClick={() => navigate('/p/' + projectId + '/cards')}>
          打开完整牌库
        </button>
      </aside>
    </>
  )
}
