import { FormEvent, useEffect, useMemo, useState } from 'react'
import {
  ArrowRight,
  BookOpen,
  Check,
  FilePlus2,
  FlaskConical,
  Library,
  Layers3,
} from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import CardDeckDrawer from '../components/CardDeckDrawer'
import JobProgress from '../components/JobProgress'
import Skeleton from '../components/Skeleton'
import StyleCardTile from '../components/StyleCardTile'
import { Tooltip } from '../components/Tooltip'
import { usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import { rememberProject } from '../projectCache'
import type {
  Asset,
  CardSummary,
  JobResult,
  JobStatus,
  Project,
  StyleCard,
} from '../types'

const MAX_FILE_BYTES = 2 * 1024 * 1024
const MAX_SAMPLE_RUNES = 100_000

export default function ProjectHome() {
  const { id } = useParams()
  const navigate = useNavigate()
  const projectId = id ?? ''
  const [project, setProject] = useState<Project | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [cards, setCards] = useState<CardSummary[]>([])
  const [loaded, setLoaded] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [file, setFile] = useState<File | null>(null)
  const [cardName, setCardName] = useState('风格卡片')
  const [model, setModel] = useState('')
  const [jobId, setJobId] = useState('')
  const [jobActive, setJobActive] = useState(false)
  const [resultCard, setResultCard] = useState<StyleCard | null>(null)
  const [deckOpen, setDeckOpen] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  usePageTitle(project?.name ?? '抽离工作台')

  async function load() {
    if (!projectId) return
    try {
      const [projectData, assetData, cardData] = await Promise.all([
        api.getProject(projectId),
        api.listAssets(projectId),
        api.listCards(projectId),
      ])
      setProject(projectData)
      rememberProject(projectId, projectData.name)
      setAssets(assetData.assets ?? [])
      setCards(cardData.cards ?? [])
      setLoadError('')
      setLoaded(true)
    } catch (err) {
      setLoadError(err instanceof APIError ? err.message : '加载失败')
      setLoaded(true)
    }
  }

  useEffect(() => {
    void load()
  }, [projectId])

  const picked = useMemo(() => assets.filter((asset) => selected[asset.id]), [assets, selected])
  const selectedRunes = picked.reduce((sum, asset) => sum + asset.rune_count, 0)
  const overSampleLimit = selectedRunes > MAX_SAMPLE_RUNES

  // Ctrl+Enter 开始抽离快捷键
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault()
        if (busy || jobActive || !picked.length || overSampleLimit) return
        void startExtract()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [busy, jobActive, picked, overSampleLimit])

  useEffect(() => {
    function onVisible() {
      if (document.visibilityState === 'visible') void load()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [projectId])

  function chooseFile(next: File | null) {
    setError('')
    if (!next) {
      setFile(null)
      return
    }
    const lower = next.name.toLocaleLowerCase()
    if (!lower.endsWith('.txt') && !lower.endsWith('.md')) {
      setFile(null)
      setError('只支持 .txt 或 .md 文件')
      return
    }
    if (next.size > MAX_FILE_BYTES) {
      setFile(null)
      setError('文件不能超过 2 MiB')
      return
    }
    setFile(next)
  }

  async function onUpload(event: FormEvent) {
    event.preventDefault()
    if (!file || !projectId) return
    setError('')
    setBusy(true)
    try {
      const created = await api.uploadAsset(projectId, file)
      setAssets((prev) => [created, ...prev])
      setSelected((prev) => ({ ...prev, [created.id]: true }))
      setFile(null)
      notify('样章已加入原料托盘', 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '上传失败')
    } finally {
      setBusy(false)
    }
  }

  function onJobStatus(status: JobStatus) {
    setJobActive(status === 'queued' || status === 'running')
  }

  async function onExtractSucceeded(result: JobResult) {
    if (!result.card_id) return
    try {
      const [card, cardData] = await Promise.all([
        api.getCard(result.card_id),
        api.listCards(projectId),
      ])
      setResultCard(card)
      setCards(cardData.cards ?? [])
      notify('新风格卡已进入牌库', 'success')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '新卡片读取失败')
    }
  }

  async function startExtract() {
    if (!picked.length) {
      setError('请先选择至少一份样章')
      return
    }
    if (overSampleLimit) {
      setError('已选样章超过 100,000 字，请移除部分样章')
      return
    }
    setError('')
    setResultCard(null)
    setBusy(true)
    const outputName = cardName.trim() || '风格卡片'
    try {
      const result = await api.extract(projectId, picked.map((asset) => asset.id), outputName, model.trim())
      setJobId(result.job_id)
      setJobActive(true)
      registerJob({
        jobId: result.job_id,
        projectId,
        kind: 'extract',
        label: '抽离「' + outputName + '」',
        startedAt: Date.now(),
      })
      notify('抽离任务已提交，卡片将在原位显现', 'info')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '抽离失败')
    } finally {
      setBusy(false)
    }
  }

  const resultSummary: CardSummary | null = resultCard ? {
    id: resultCard.id,
    name: resultCard.name,
    kind: resultCard.kind,
    current_version: resultCard.version,
    updated_at: new Date().toISOString(),
  } : null

  return (
    <div className="page page-wide">
      <div className="page-head">
        <div>
          <p className="kicker">EXTRACT BENCH</p>
          <h1>{project?.name ?? '抽离工作台'}</h1>
          <p className="sub">把有权使用的样章放进原料托盘，生成一张只保留抽象技法的风格卡。</p>
        </div>
        <div className="page-actions">
          <button className="btn secondary" type="button" onClick={() => setDeckOpen(true)}>
            <Library size={17} />牌库 {cards.length}
          </button>
          <Link className="btn secondary" to={'/p/' + projectId + '/fuse'}>
            <FlaskConical size={17} />融合台
          </Link>
        </div>
      </div>

      {loadError ? (
        <div className="empty-state">
          <h2>工作台加载失败</h2>
          <p className="error">{loadError}</p>
          <button className="btn secondary" type="button" onClick={() => void load()}>重试</button>
        </div>
      ) : !loaded ? (
        <div className="extract-workbench">
          {Array.from({ length: 3 }, (_, index) => <Skeleton key={index} h={480} />)}
        </div>
      ) : (
        <>
          <div className="extract-workbench">
            <section className="work-zone asset-shelf">
              <div className="zone-heading">
                <span className="zone-index">01</span>
                <div><h2>样章架</h2><p>上传并管理本次可选素材</p></div>
              </div>
              <form onSubmit={onUpload} className="stack">
                <label
                  className={'dropzone' + (file ? ' has' : '')}
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    event.preventDefault()
                    chooseFile(event.dataTransfer.files?.[0] ?? null)
                  }}
                >
                  <input
                    type="file"
                    accept=".txt,.md,text/plain,text/markdown"
                    onChange={(event) => chooseFile(event.target.files?.[0] ?? null)}
                  />
                  <FilePlus2 size={24} aria-hidden="true" />
                  <strong>{file?.name ?? '拖入 .txt / .md'}</strong>
                  <span>{file ? '文件通过前检，可加入样章架' : '单个文件不超过 2 MiB'}</span>
                </label>
                <button className="btn secondary" type="submit" disabled={!file || busy}>
                  {busy ? '处理中…' : '加入样章架'}
                </button>
              </form>
              <div className="asset-list">
                {assets.length ? assets.map((asset) => {
                  const active = !!selected[asset.id]
                  return (
                    <button
                      key={asset.id}
                      className={'asset-card' + (active ? ' selected' : '')}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setSelected((prev) => ({ ...prev, [asset.id]: !prev[asset.id] }))}
                    >
                      <BookOpen size={17} aria-hidden="true" />
                      <span><strong>{asset.filename}</strong><small>{asset.rune_count} 字 · {asset.chapter_count} 章</small></span>
                      {active ? <Check size={17} aria-hidden="true" /> : null}
                    </button>
                  )
                }) : <p className="zone-empty">还没有样章。上传后会出现在这里。</p>}
              </div>
            </section>

            <section className="work-zone ingredient-tray">
              <div className="zone-heading">
                <span className="zone-index">02</span>
                <div><h2>原料托盘</h2><p>确认参与抽离的文本</p></div>
              </div>
              <div className="tray-meter">
                <div><strong>{picked.length}</strong><span>份样章</span></div>
                <div><strong className={overSampleLimit ? 'danger-text' : ''}>{selectedRunes.toLocaleString()}</strong><span>/ 100,000 字</span></div>
              </div>
              <div className="sample-progress" aria-label={'已选 ' + selectedRunes + ' 字'}>
                <i className={overSampleLimit ? 'over' : ''} style={{ width: Math.min(100, selectedRunes / MAX_SAMPLE_RUNES * 100) + '%' }} />
              </div>
              <div className="ingredient-stack">
                {picked.length ? picked.map((asset, index) => (
                  <button
                    key={asset.id}
                    className="ingredient-card"
                    type="button"
                    onClick={() => setSelected((prev) => ({ ...prev, [asset.id]: false }))}
                    style={{ '--stack-index': index } as React.CSSProperties}
                    title="点击移出托盘"
                  >
                    <Layers3 size={18} aria-hidden="true" />
                    <span><strong>{asset.filename}</strong><small>{asset.rune_count} 字</small></span>
                  </button>
                )) : <p className="zone-empty">从左侧点选样章，它们会依次叠放到这里。</p>}
              </div>
              {overSampleLimit ? <p className="error">总字数超过上限，请移除部分样章。</p> : null}
            </section>

            <section className="work-zone card-mold">
              <div className="zone-heading">
                <span className="zone-index">03</span>
                <div><h2>{resultCard ? '卡片已显现' : '卡片模具'}</h2><p>{resultCard ? '已自动收入牌库' : '命名并开始抽离'}</p></div>
              </div>
              {resultSummary && resultCard ? (
                <div className="result-reveal">
                  <StyleCardTile
                    card={resultSummary}
                    active
                    onClick={() => navigate('/p/' + projectId + '/lab/' + resultCard.id)}
                    actionLabel="查看新卡片"
                  />
                  <div className="result-actions">
                    <Link className="btn" to={'/p/' + projectId + '/lab/' + resultCard.id}>
                      进入实验室<ArrowRight size={16} />
                    </Link>
                    <Link className="btn secondary" to={'/p/' + projectId + '/fuse?cards=' + resultCard.id}>
                      加入融合
                    </Link>
                    <button className="btn secondary" type="button" onClick={() => setResultCard(null)}>
                      继续抽离下一张
                    </button>
                  </div>
                </div>
              ) : (
                <>
                  <div className="card-preview-shell">
                    <span className="eyebrow">STYLE CARD</span>
                    <strong>{cardName.trim() || '未命名卡片'}</strong>
                    <span>{picked.length ? '来自 ' + picked.length + ' 份样章' : '等待样章入托'}</span>
                    <div className="preview-dimensions" aria-hidden="true">
                      {Array.from({ length: 9 }, (_, index) => <i key={index} />)}
                    </div>
                  </div>
                  <div className="stack">
                    <label>卡片名称<input value={cardName} maxLength={80} onChange={(event) => setCardName(event.target.value)} /></label>
                    <details className="advanced-settings">
                      <summary>高级设置</summary>
                      <label>模型（可选）<input value={model} onChange={(event) => setModel(event.target.value)} /></label>
                    </details>
                    <Tooltip label="Ctrl+Enter 开始抽离" shortcut="⏎">
                      <button
                        className="btn"
                        type="button"
                        disabled={busy || jobActive || !picked.length || overSampleLimit}
                        onClick={() => void startExtract()}
                      >
                        {jobActive ? '抽离进行中…' : picked.length ? '开始抽离' : '先选择样章'}
                      </button>
                    </Tooltip>
                  </div>
                </>
              )}
              {jobId ? (
                <JobProgress
                  jobId={jobId}
                  projectId={projectId}
                  onStatus={onJobStatus}
                  autoNavigate={false}
                  onSucceeded={(result) => void onExtractSucceeded(result)}
                />
              ) : null}
              {error ? <p className="error">{error}</p> : null}
            </section>
          </div>

          <section className="recent-deck">
            <div className="section-head">
              <div><p className="kicker">RECENT CARDS</p><h2>最近卡片</h2></div>
              <Link className="text-link" to={'/p/' + projectId + '/cards'}>查看完整牌库<ArrowRight size={15} /></Link>
            </div>
            {cards.length ? (
              <div className="recent-card-strip">
                {cards.slice(0, 4).map((card) => (
                  <StyleCardTile
                    key={card.id}
                    card={card}
                    onClick={() => setDeckOpen(true)}
                    actionLabel="打开牌库检视"
                  />
                ))}
              </div>
            ) : <p className="zone-empty">抽离完成后，卡片会在这里出现。</p>}
          </section>
        </>
      )}
      <CardDeckDrawer open={deckOpen} projectId={projectId} cards={cards} onClose={() => setDeckOpen(false)} />
    </div>
  )
}
