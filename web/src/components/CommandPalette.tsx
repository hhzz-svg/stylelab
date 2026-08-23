import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  BookMarked,
  BookOpen,
  Boxes,
  FlaskConical,
  KeyRound,
  Library,
  PenLine,
  Search,
  Sparkles,
  X,
} from 'lucide-react'
import { api } from '../api'
import type { CardSummary, ChapterSummary, Project } from '../types'

type Props = {
  isOpen: boolean
  onClose: () => void
  currentProjectId?: string
}

type CommandAction = {
  id: string
  title: string
  subtitle?: string
  icon: typeof Search
  category: '导航' | '作品' | '章节' | '风格卡' | '操作'
  onSelect: () => void
}

export default function CommandPalette({ isOpen, onClose, currentProjectId }: Props) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [projects, setProjects] = useState<Project[]>([])
  const [chapters, setChapters] = useState<ChapterSummary[]>([])
  const [cards, setCards] = useState<CardSummary[]>([])
  const [selectedIndex, setSelectedIndex] = useState(0)

  useEffect(() => {
    if (!isOpen) {
      setQuery('')
      setSelectedIndex(0)
      return
    }

    // Load contextual items for search
    void api.listProjects().then((d) => setProjects(d.projects ?? [])).catch(() => null)
    if (currentProjectId) {
      void api.listChapters(currentProjectId).then((d) => setChapters(d.chapters ?? [])).catch(() => null)
      void api.listCards(currentProjectId).then((d) => setCards(d.cards ?? [])).catch(() => null)
    }
  }, [isOpen, currentProjectId])

  // Build command actions list
  const actions: CommandAction[] = [
    {
      id: 'nav-projects',
      title: '作品集',
      subtitle: '回到主工作台',
      icon: Library,
      category: '导航',
      onSelect: () => { navigate('/'); onClose() },
    },
    {
      id: 'nav-settings',
      title: '模型与密钥设置',
      subtitle: '配置 BYOK API Key',
      icon: KeyRound,
      category: '导航',
      onSelect: () => { navigate('/settings'); onClose() },
    },
  ]

  if (currentProjectId) {
    actions.push(
      {
        id: 'nav-extract',
        title: '风格抽离工坊',
        subtitle: '上传样章并提炼九维技法',
        icon: BookOpen,
        category: '导航',
        onSelect: () => { navigate(`/p/${currentProjectId}`); onClose() },
      },
      {
        id: 'nav-cards',
        title: '牌库总览',
        subtitle: '查看、对比与导出风格卡片',
        icon: Boxes,
        category: '导航',
        onSelect: () => { navigate(`/p/${currentProjectId}/cards`); onClose() },
      },
      {
        id: 'nav-fuse',
        title: '风格融合炼金阵',
        subtitle: '按维度融合多张风格卡',
        icon: FlaskConical,
        category: '导航',
        onSelect: () => { navigate(`/p/${currentProjectId}/fuse`); onClose() },
      },
      {
        id: 'nav-write',
        title: '章节写作台',
        subtitle: '小说目录与大纲管理',
        icon: PenLine,
        category: '导航',
        onSelect: () => { navigate(`/p/${currentProjectId}/write`); onClose() },
      },
      {
        id: 'nav-bible',
        title: '设定集世界观',
        subtitle: '人物、设定与伏笔管理',
        icon: BookMarked,
        category: '导航',
        onSelect: () => { navigate(`/p/${currentProjectId}/bible`); onClose() },
      },
    )

    chapters.forEach((ch) => {
      actions.push({
        id: `ch-${ch.id}`,
        title: `第 ${ch.seq} 章：${ch.title}`,
        subtitle: ch.brief || `${ch.rune_count || 0} 字`,
        icon: PenLine,
        category: '章节',
        onSelect: () => { navigate(`/p/${currentProjectId}/chapter/${ch.id}`); onClose() },
      })
    })

    cards.forEach((c) => {
      actions.push({
        id: `card-${c.id}`,
        title: `风格卡：${c.name}`,
        subtitle: `v${c.current_version} · ${c.kind}`,
        icon: Sparkles,
        category: '风格卡',
        onSelect: () => { navigate(`/p/${currentProjectId}/lab/${c.id}`); onClose() },
      })
    })
  }

  projects.forEach((p) => {
    if (p.id !== currentProjectId) {
      actions.push({
        id: `proj-${p.id}`,
        title: `切换作品：${p.name}`,
        subtitle: `创建于 ${p.created_at.slice(0, 10)}`,
        icon: Library,
        category: '作品',
        onSelect: () => { navigate(`/p/${p.id}`); onClose() },
      })
    }
  })

  const filtered = actions.filter((a) => {
    if (!query.trim()) return true
    const q = query.toLowerCase()
    return a.title.toLowerCase().includes(q) || (a.subtitle && a.subtitle.toLowerCase().includes(q))
  })

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (!isOpen) return
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((prev) => (prev + 1) % Math.max(1, filtered.length))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => (prev - 1 + filtered.length) % Math.max(1, filtered.length))
      } else if (e.key === 'Enter') {
        e.preventDefault()
        if (filtered[selectedIndex]) {
          filtered[selectedIndex].onSelect()
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [isOpen, filtered, selectedIndex])

  if (!isOpen) return null

  return (
    <div className="cmd-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div className="cmd-modal" onClick={(e) => e.stopPropagation()}>
        <div className="cmd-search-wrap">
          <Search size={19} color="var(--gold-hi)" aria-hidden="true" />
          <input
            className="cmd-input"
            autoFocus
            placeholder="搜索指令、章节、风格卡或作品…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button className="btn ghost sm" onClick={onClose} aria-label="关闭">
            <X size={16} />
          </button>
        </div>
        <div className="cmd-list">
          {filtered.length === 0 ? (
            <div style={{ padding: '2rem 1rem', textAlign: 'center', color: 'var(--ink-faint)' }}>
              未找到匹配项
            </div>
          ) : (
            filtered.map((action, i) => {
              const Icon = action.icon
              const isSelected = i === selectedIndex
              return (
                <div
                  key={action.id}
                  className={`cmd-item${isSelected ? ' selected' : ''}`}
                  onClick={action.onSelect}
                  onMouseEnter={() => setSelectedIndex(i)}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
                    <div
                      style={{
                        width: '28px',
                        height: '28px',
                        borderRadius: '6px',
                        background: isSelected ? 'var(--gold-gradient)' : 'rgba(255,255,255,0.05)',
                        color: isSelected ? '#070a0f' : 'var(--gold)',
                        display: 'grid',
                        placeItems: 'center',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      <Icon size={15} />
                    </div>
                    <div>
                      <div style={{ fontWeight: 600, color: isSelected ? 'var(--gold-hi)' : 'var(--ink)' }}>
                        {action.title}
                      </div>
                      {action.subtitle && (
                        <div style={{ fontSize: '0.76rem', color: 'var(--ink-faint)' }}>
                          {action.subtitle}
                        </div>
                      )}
                    </div>
                  </div>
                  <span className="cmd-key">{action.category}</span>
                </div>
              )
            })
          )}
        </div>
        <div
          style={{
            padding: '0.65rem 1.2rem',
            borderTop: '1px solid var(--line-glass)',
            background: 'rgba(7,10,15,0.6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            fontSize: '0.75rem',
            color: 'var(--ink-faint)',
          }}
        >
          <div style={{ display: 'flex', gap: '1rem' }}>
            <span>↑↓ 切换</span>
            <span>↵ 确认</span>
            <span>ESC 关闭</span>
          </div>
          <span>Style Lab Spotlight</span>
        </div>
      </div>
    </div>
  )
}
