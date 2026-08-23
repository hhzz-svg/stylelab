import type { BibleEntrySummary, ChapterSummary, StyleCard } from './types'

export const PREV_TAIL_RUNES = 300

export type ChapterIntel = {
  card: StyleCard | null
  cardError: string
  prev: ChapterSummary | null
  next: ChapterSummary | null
  prevSummary: string
  prevTail: string
  prevError: string
  bible: BibleEntrySummary[]
  bibleError: string
}

export function emptyIntel(): ChapterIntel {
  return {
    card: null,
    cardError: '',
    prev: null,
    next: null,
    prevSummary: '',
    prevTail: '',
    prevError: '',
    bible: [],
    bibleError: '',
  }
}

export function lastRunes(text: string, n: number): string {
  const runes = Array.from((text ?? '').trim())
  if (runes.length <= n) return runes.join('')
  return runes.slice(-n).join('')
}

export function neighbors(list: ChapterSummary[], currentId: string) {
  const index = list.findIndex((item) => item.id === currentId)
  return {
    prev: index > 0 ? list[index - 1] : null,
    next: index >= 0 && index < list.length - 1 ? list[index + 1] : null,
  }
}
