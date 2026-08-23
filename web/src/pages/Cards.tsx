import { useEffect, useMemo, useState } from 'react'
import {
  Download,
  FlaskConical,
  Library,
  Plus,
  Search,
  SlidersHorizontal,
  Sparkles,
  X,
} from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import CardInspector from '../components/CardInspector'
import Crumb from '../components/Crumb'
import Skeleton from '../components/Skeleton'
import StyleCardTile from '../components/StyleCardTile'
import { Tooltip } from '../components/Tooltip'
import { usePageTitle } from '../hooks'
import { PRESET_CARDS, type PresetCardTemplate } from '../presetCards'
import { rememberProject } from '../projectCache'
import { KIND_LABEL, type CardSummary, type StyleCard } from '../types'

type SortMode = 'updated' | 'name'

export default function Cards() {
  const { id } = useParams()
  const navigate = useNavigate()
  const projectId = id ?? ''
  const [cards, setCards] = useState<CardSummary[] | null>(null)
  const [details, setDetails] = useState<Record<string, StyleCard>>({})
  const [selectedId, setSelectedId] = useState('')
  const [batchMode, setBatchMode] = useState(false)
  const [batchSelected, setBatchSelected] = useState<Record<string, boolean>>({})
  const [query, setQuery] = useState('')
  const [kind, setKind] = useState('all')
  const [sort, setSort] = useState<SortMode>('updated')
  const [error, setError] = useState('')
  const [detailError, setDetailError] = useState('')
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [presetModalOpen, setPresetModalOpen] = useState(false)
  const [importingPresetId, setImportingPresetId] = useState('')
  usePageTitle('牌库')

  useEffect(() => {
    if (!projectId) return
    Promise.all([api.listCards(projectId), api.getProject(projectId)])
      .then(([cardData, project]) => {
        const list = cardData.cards ?? []
        setCards(list)
        rememberProject(projectId, project.name)
        setError('')
      })
      .catch((err: unknown) => {
        setError(err instanceof APIError ? err.message : '牌库加载失败')
        setCards([])
      })
  }, [projectId])

  const visibleCards = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase()
    return [...(cards ?? [])]
      .filter((card) => kind === 'all' || card.kind === kind)
      .filter((card) => !normalized || card.name.toLocaleLowerCase().includes(normalized))
      .sort((a, b) => (
        sort === 'name'
          ? a.name.localeCompare(b.name, 'zh-CN')
          : b.updated_at.localeCompare(a.updated_at)
      ))
  }, [cards, kind, query, sort])

  const batchIds = useMemo(
    () => Object.keys(batchSelected).filter((id) => batchSelected[id]),
    [batchSelected],
  )

  function toggleBatchMode() {
    if (batchMode) {
      setBatchSelected({})
    }
    setBatchMode((v) => !v)
    setSelectedId('')
  }

  function toggleBatchCard(cardId: string) {
    setBatchSelected((prev) => ({ ...prev, [cardId]: !prev[cardId] }))
  }

  function selectAll() {
    const next: Record<string, boolean> = {}
    visibleCards.forEach((card) => { next[card.id] = true })
    setBatchSelected(next)
  }

  function deselectAll() {
    setBatchSelected({})
  }

  async function selectCard(card: CardSummary) {
    setSelectedId(card.id)
    setDetailError('')
    if (details[card.id]) return
    setLoadingDetail(true)
    try {
      const detail = await api.getCard(card.id)
      setDetails((prev) => ({ ...prev, [card.id]: detail }))
    } catch (err) {
      setDetailError(err instanceof APIError ? err.message : '卡片详情加载失败')
    } finally {
      setLoadingDetail(false)
    }
  }

  function onBatchFuse() {
    if (batchIds.length < 2) {
      notify('至少选择 2 张卡片才能融合', 'error')
      return
    }
    if (batchIds.length > 4) {
      notify('最多只能融合 4 张卡片', 'error')
      return
    }
    setBatchMode(false)
    navigate(`/p/${projectId}/fuse?cards=${batchIds.join(',')}`)
  }

  async function onImportPreset(preset: PresetCardTemplate) {
    setImportingPresetId(preset.id)
    try {
      const created = await api.createCard(projectId, {
        name: preset.name,
        kind: 'preset',
        dimensions: preset.dimensions,
        prohibitions: preset.prohibitions,
      })
      notify(`已成功导入官方风格卡「${created.name}」`, 'success')
      const cardData = await api.listCards(projectId)
      setCards(cardData.cards ?? [])
      setSelectedId(created.id)
      setPresetModalOpen(false)
    } catch (err) {
      notify(err instanceof APIError ? err.message : '导入预设卡失败', 'error')
    } finally {
      setImportingPresetId('')
    }
  }

  return (
    <div className="page page-wide">
      <Crumb projectId={projectId} current="牌库" />
      <div className="page-head">
        <div>
          <p className="kicker">CARD DECK</p>
          <h1>风格牌库</h1>
          <p className="sub">
            {batchMode
              ? `已选 ${batchIds.length} 张卡片`
              : '检视、筛选并挑选已有风格卡。支持一键领用官方精调经典流派风格卡。'}
          </p>
        </div>
        <div className="status-grid">
          <button
            className="btn"
            type="button"
            onClick={() => setPresetModalOpen(true)}
            style={{ background: 'linear-gradient(135deg, #f7cb68 0%, #e2b04a 100%)', color: '#070a0f', fontWeight: 700 }}
          >
            <Sparkles size={16} />
            导入官方经典预设
          </button>
          <div className="status-chip">
            <span>卡片</span>
            <strong>{cards?.length ?? '—'}</strong>
          </div>
          <Tooltip label={batchMode ? '退出批量模式' : '进入批量模式选多张卡融合'}>
            <button
              className="btn secondary"
              type="button"
              onClick={toggleBatchMode}
              style={batchMode ? { borderColor: 'var(--cinnabar-hi)', color: 'var(--cinnabar-hi)' } : {}}
            >
              {batchMode ? <><X size={16} />退出批量</> : <><Plus size={16} />批量选择</>}
            </button>
          </Tooltip>
        </div>
      </div>

      {/* 批量工具栏 */}
      {batchMode && batchIds.length > 0 && (
        <div className="batch-toolbar">
          <span className="batch-count">{batchIds.length} 张已选</span>
          <div className="batch-actions">
            <Tooltip label={batchIds.length < 2 ? '至少选择 2 张卡片' : batchIds.length > 4 ? '最多只能融合 4 张' : '跳转到融合台开始融合'}>
              <button
                className="btn"
                type="button"
                onClick={onBatchFuse}
                disabled={batchIds.length < 2 || batchIds.length > 4}
              >
                <FlaskConical size={16} />
                融合 {batchIds.length >= 2 ? `(${batchIds.length})` : ''}
              </button>
            </Tooltip>
          </div>
          <div className="batch-select-all">
            <button className="btn secondary sm" type="button" onClick={selectAll}>全选</button>
            <button className="btn secondary sm" type="button" onClick={deselectAll}>清空</button>
          </div>
        </div>
      )}

      {batchMode && batchIds.length === 0 && cards && cards.length > 0 && (
        <div className="batch-hint">
          <span>点击卡片进行选择，再点「融合」跳转到融合台</span>
          <button className="btn secondary sm" type="button" onClick={selectAll}>全选</button>
        </div>
      )}

      <div className="library-tools" aria-label="牌库筛选">
        <label className="search-field">
          <Search size={17} aria-hidden="true" />
          <span className="sr-only">搜索卡片</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索卡片名称" />
        </label>
        <label className="select-field">
          <SlidersHorizontal size={16} aria-hidden="true" />
          <span className="sr-only">卡片类型</span>
          <select value={kind} onChange={(event) => setKind(event.target.value)}>
            <option value="all">全部类型</option>
            {Object.entries(KIND_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </label>
        <label className="select-field">
          <span className="sr-only">排序方式</span>
          <select value={sort} onChange={(event) => setSort(event.target.value as SortMode)}>
            <option value="updated">最近更新</option>
            <option value="name">按名称</option>
          </select>
        </label>
      </div>

      {error ? <p className="error">{error}</p> : null}
      <div className="card-library-layout">
        <section className="card-library-grid" aria-label="风格卡片">
          {cards === null ? (
            Array.from({ length: 6 }, (_, index) => <Skeleton key={index} h={280} />)
          ) : visibleCards.length ? (
            visibleCards.map((card) => (
              <BatchableCardTile
                key={card.id}
                card={card}
                batchMode={batchMode}
                batchSelected={batchSelected[card.id] ?? false}
                active={card.id === selectedId && !batchMode}
                highlight={query.trim()}
                onSelect={() => void selectCard(card)}
                onBatchToggle={() => toggleBatchCard(card.id)}
              />
            ))
          ) : (
            <div className="empty-state compact">
              <Library size={24} aria-hidden="true" />
              <h2>{cards.length ? '没有符合条件的卡片' : '牌库还是空的'}</h2>
              <p className="muted">{cards.length ? '换一个名称或类型筛选。' : '可一键领用官方预设，或去抽离工作台生成。'}</p>
              <div style={{ display: 'flex', gap: '0.6rem', marginTop: '0.8rem' }}>
                <button className="btn sm" type="button" onClick={() => setPresetModalOpen(true)}>
                  <Sparkles size={14} /> 领用官方预设卡
                </button>
                <Link className="btn secondary sm" to={`/p/${projectId}`}>
                  去抽离卡片
                </Link>
              </div>
            </div>
          )}
        </section>
        {!batchMode && (
          <CardInspector
            card={selectedId ? details[selectedId] ?? null : null}
            projectId={projectId}
            loading={loadingDetail}
            error={detailError}
          />
        )}
      </div>

      {/* 官方经典预设风格卡库弹窗 */}
      {presetModalOpen && (
        <div className="modal-backdrop" onClick={() => setPresetModalOpen(false)}>
          <div className="preset-deck-modal" onClick={(e) => e.stopPropagation()}>
            <div className="preset-deck-head">
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Sparkles size={18} color="var(--gold-hi)" />
                  <h2 style={{ margin: 0, fontFamily: 'var(--font-serif)', fontSize: '1.35rem' }}>
                    官方经典流派风格卡库
                  </h2>
                </div>
                <p className="muted" style={{ margin: '0.3rem 0 0', fontSize: '0.86rem' }}>
                  由资深网文架构师精调的 6 大主流流派技法模版，一键导入本项目即可直接写作与融合。
                </p>
              </div>
              <button className="icon-btn" type="button" onClick={() => setPresetModalOpen(false)}>
                <X size={18} />
              </button>
            </div>

            <div className="preset-deck-grid">
              {PRESET_CARDS.map((preset) => {
                const isImporting = importingPresetId === preset.id
                return (
                  <div key={preset.id} className="preset-card-item">
                    <div className="preset-item-header">
                      <span className="preset-icon">{preset.icon}</span>
                      <div style={{ flex: 1 }}>
                        <h3 className="preset-name">{preset.name}</h3>
                        <p className="preset-tagline">{preset.tagline}</p>
                      </div>
                    </div>

                    <p className="preset-desc">{preset.description}</p>

                    <div className="preset-dim-badges">
                      <span className="dim-chip">对白 {preset.dimensions.dialogue_density.level}</span>
                      <span className="dim-chip">节奏 {preset.dimensions.sentence_rhythm.level}</span>
                      <span className="dim-chip">感官 {preset.dimensions.sensory_description.level}</span>
                      <span className="dim-chip">张力 {preset.dimensions.tension_hook.level}</span>
                    </div>

                    <button
                      className="btn sm"
                      type="button"
                      disabled={isImporting}
                      onClick={() => void onImportPreset(preset)}
                      style={{ width: '100%', marginTop: '0.8rem' }}
                    >
                      {isImporting ? <div className="spinner sm" /> : <Download size={14} />}
                      {isImporting ? '正在导入中…' : '一键导入此卡'}
                    </button>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// 可批量选择的卡片瓦片
function BatchableCardTile({
  card,
  batchMode,
  batchSelected,
  active,
  highlight,
  onSelect,
  onBatchToggle,
}: {
  card: CardSummary
  batchMode: boolean
  batchSelected: boolean
  active: boolean
  highlight?: string
  onSelect: () => void
  onBatchToggle: () => void
}) {
  return (
    <div
      className={[
        'batch-card-wrapper',
        batchSelected ? 'batch-selected' : '',
        active ? 'active' : '',
      ].filter(Boolean).join(' ')}
      onClick={batchMode ? onBatchToggle : onSelect}
    >
      {batchMode && (
        <div className="batch-checkbox">
          <span className={`checkbox ${batchSelected ? 'checked' : ''}`}>
            {batchSelected ? '✓' : ''}
          </span>
        </div>
      )}
      <StyleCardTile
        card={card}
        active={active && !batchMode}
        selected={batchMode && batchSelected}
        highlight={highlight}
        onClick={batchMode ? onBatchToggle : onSelect}
      />
    </div>
  )
}
