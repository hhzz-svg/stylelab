import { FormEvent, useEffect, useRef, useState } from 'react'
import {
  Activity,
  ArrowLeft,
  ArrowRight,
  BookOpenText,
  CloudRain,
  GitFork,
  Headphones,
  Maximize2,
  Minimize2,
  Music,
  PanelRightOpen,
  Save,
  Sparkles,
  Volume2,
  VolumeX,
  WandSparkles,
  Wind,
} from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import { ambient, type AmbientSoundType } from '../ambientSound'
import { emptyIntel, lastRunes, neighbors, PREV_TAIL_RUNES, type ChapterIntel } from '../chapterContext'
import AudioNarrationBar from '../components/AudioNarrationBar'
import BranchSimulationDrawer from '../components/BranchSimulationDrawer'
import { confirm } from '../components/ConfirmDialog'
import ChapterIntelPanel from '../components/ChapterIntelPanel'
import ScenePanel from '../components/ScenePanel'
import Crumb from '../components/Crumb'
import InlineAIMenu from '../components/InlineAIMenu'
import JobProgress from '../components/JobProgress'
import RhythmVisualizer from '../components/RhythmVisualizer'
import Skeleton from '../components/Skeleton'
import { useDirtyGuard, usePageTitle } from '../hooks'
import { registerJob } from '../jobs'
import type { CardSummary, Chapter, ChapterSummary, JobStatus } from '../types'

type Operation = 'idle' | 'saving' | 'submitting' | 'generating' | 'refreshing'

type ActiveJob = {
  id: string
  projectId: string
  chapterId: string
  operationToken: number
}

const CHAPTER_POLL_MS = 1000

export default function ChapterPage() {
  const { id, chapterId } = useParams()
  const navigate = useNavigate()
  const projectId = id ?? ''
  const idOfChapter = chapterId ?? ''

  const [ch, setCh] = useState<Chapter | null>(null)
  const [cards, setCards] = useState<CardSummary[]>([])
  const cacheRef = useRef<Record<string, Chapter>>({})
  const [intel, setIntel] = useState<ChapterIntel>(emptyIntel())
  const [intelOpen, setIntelOpen] = useState(false)
  const [loadingCards, setLoadingCards] = useState(false)
  const [loadingChapters, setLoadingChapters] = useState(false)
  const [loadingCard, setLoadingCard] = useState(false)
  const [loadingPrev, setLoadingPrev] = useState(false)
  const [loadingBible, setLoadingBible] = useState(false)
  const [title, setTitle] = useState('')
  const [brief, setBrief] = useState('')
  const [body, setBody] = useState('')
  const [summary, setSummary] = useState('')
  const [cardId, setCardId] = useState('')
  const [target, setTarget] = useState(2500)
  const [model, setModel] = useState('')
  const [modelBaseline, setModelBaseline] = useState('')
  const [note, setNote] = useState('')
  const [pageError, setPageError] = useState('')
  const [cardsError, setCardsError] = useState('')
  const [chaptersError, setChaptersError] = useState('')
  const [operationError, setOperationError] = useState('')
  const [operation, setOperation] = useState<Operation>('idle')
  const [activeJob, setActiveJob] = useState<ActiveJob | null>(null)
  const [jobPollKey, setJobPollKey] = useState(0)
  const [jobError, setJobError] = useState('')
  const [refreshError, setRefreshError] = useState('')
  const [chapterPollPaused, setChapterPollPaused] = useState(false)
  const [setupOpen, setSetupOpen] = useState(false)
  const [zenMode, setZenMode] = useState(false)
  const [ambientType, setAmbientType] = useState<AmbientSoundType | null>(null)
  const [ambientVol, setAmbientVol] = useState(0.35)
  const [showAmbientPicker, setShowAmbientPicker] = useState(false)
  const [rhythmOpen, setRhythmOpen] = useState(false)
  const [selectedText, setSelectedText] = useState('')
  const [selectionRange, setSelectionRange] = useState<{ start: number; end: number } | null>(null)
  const [selectionPos, setSelectionPos] = useState<{ top: number; left: number } | null>(null)
  const [inlineAIOpen, setInlineAIOpen] = useState(false)
  const [branchOpen, setBranchOpen] = useState(false)
  const [audioOpen, setAudioOpen] = useState(false)

  const bodyTextareaRef = useRef<HTMLTextAreaElement>(null)
  const routeRef = useRef({ projectId, chapterId: idOfChapter })
  const loadEpochRef = useRef(0)
  const cardsRequestRef = useRef(0)
  const chaptersRequestRef = useRef(0)
  const cardRequestRef = useRef(0)
  const prevRequestRef = useRef(0)
  const bibleRequestRef = useRef(0)
  const operationRef = useRef<Operation>('idle')
  const operationTokenRef = useRef(0)
  const setupRef = useRef<HTMLDetailsElement>(null)
  const cardSelectRef = useRef<HTMLSelectElement>(null)
  const briefRef = useRef<HTMLTextAreaElement>(null)
  const intelOpenerRef = useRef<HTMLButtonElement>(null)
  const leavePromptRef = useRef(false)
  routeRef.current = { projectId, chapterId: idOfChapter }

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape' && zenMode) {
        setZenMode(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [zenMode])

  function toggleAmbient(type: AmbientSoundType) {
    if (ambientType === type) {
      ambient.stop()
      setAmbientType(null)
    } else {
      ambient.play(type, ambientVol)
      setAmbientType(type)
    }
  }

  function handleBodySelect(e: React.SyntheticEvent<HTMLTextAreaElement>) {
    const el = e.currentTarget
    const start = el.selectionStart
    const end = el.selectionEnd
    if (start !== end && end - start > 2) {
      const text = el.value.slice(start, end)
      setSelectedText(text)
      setSelectionRange({ start, end })
      const rect = el.getBoundingClientRect()
      setSelectionPos({
        top: Math.max(10, rect.top - 140),
        left: Math.min(window.innerWidth - 380, Math.max(20, rect.left + 40)),
      })
      setInlineAIOpen(true)
    }
  }

  function handleInlineReplace(replacement: string) {
    if (!selectionRange) return
    const newBody = body.slice(0, selectionRange.start) + replacement + body.slice(selectionRange.end)
    setBody(newBody)
    notify('已替换选区', 'success')
    setInlineAIOpen(false)
  }

  function handleInlineAppend(appendix: string) {
    if (!selectionRange) return
    const newBody = body.slice(0, selectionRange.end) + appendix + body.slice(selectionRange.end)
    setBody(newBody)
    notify('已在选区后追加', 'success')
    setInlineAIOpen(false)
  }


  const loadedChapter =
    ch?.id === idOfChapter && ch.project_id === projectId ? ch : null
  usePageTitle(loadedChapter ? `第 ${loadedChapter.seq} 章 ${loadedChapter.title}` : '章节')

  const dirty =
    !!loadedChapter &&
    (title !== loadedChapter.title ||
      brief !== loadedChapter.brief ||
      body !== loadedChapter.body ||
      summary !== loadedChapter.summary ||
      cardId !== loadedChapter.card_id ||
      target !== loadedChapter.target_runes)
  const generationSettingsDirty =
    !!loadedChapter &&
    (title !== loadedChapter.title ||
      brief !== loadedChapter.brief ||
      cardId !== loadedChapter.card_id ||
      target !== loadedChapter.target_runes)
  const generationDraftDirty = model !== modelBaseline || note.trim() !== ''
  const leaveDirty = dirty || generationDraftDirty
  useDirtyGuard(leaveDirty)
  const editingLocked = operation !== 'idle'

  function replaceOperation(next: Operation) {
    const token = operationTokenRef.current + 1
    operationTokenRef.current = token
    operationRef.current = next
    setOperation(next)
    return token
  }

  function beginOperation(next: Exclude<Operation, 'idle'>) {
    if (operationRef.current !== 'idle') return null
    return replaceOperation(next)
  }

  function moveOperation(token: number, next: Operation) {
    if (operationTokenRef.current !== token) return false
    operationRef.current = next
    setOperation(next)
    return true
  }

  function isCurrentRequest(epoch: number, targetProjectId: string, targetChapterId: string) {
    const route = routeRef.current
    return (
      loadEpochRef.current === epoch &&
      route.projectId === targetProjectId &&
      route.chapterId === targetChapterId
    )
  }

  function setChapterBaseline(data: Chapter) {
    setCh(data)
    cacheRef.current = { ...cacheRef.current, [data.id]: data }
  }

  function applyChapter(data: Chapter, syncModel = true) {
    setChapterBaseline(data)
    setTitle(data.title)
    setBrief(data.brief)
    setBody(data.body)
    setSummary(data.summary)
    setCardId(data.card_id)
    setTarget(data.target_runes || 2500)
    if (syncModel) {
      setModel(data.model)
      setModelBaseline(data.model)
    }
  }

  async function loadCard(
    nextCardId: string,
    epoch = loadEpochRef.current,
    targetProjectId = projectId,
    targetChapterId = idOfChapter,
  ) {
    if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
    const request = ++cardRequestRef.current
    setIntel((previous) => ({ ...previous, card: null, cardError: '' }))
    if (!nextCardId) {
      setLoadingCard(false)
      return
    }
    setLoadingCard(true)
    try {
      const card = await api.getCard(nextCardId)
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        cardRequestRef.current !== request ||
        card.id !== nextCardId
      ) return
      setIntel((previous) => ({ ...previous, card, cardError: '' }))
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        cardRequestRef.current !== request
      ) return
      setIntel((previous) => ({
        ...previous,
        card: null,
        cardError: err instanceof APIError ? err.message : '风格卡读取失败',
      }))
    } finally {
      if (
        isCurrentRequest(epoch, targetProjectId, targetChapterId) &&
        cardRequestRef.current === request
      ) setLoadingCard(false)
    }
  }

  async function loadPrev(
    prev: ChapterSummary | null,
    epoch = loadEpochRef.current,
    targetProjectId = projectId,
    targetChapterId = idOfChapter,
  ) {
    if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
    const request = ++prevRequestRef.current
    setIntel((previous) => ({
      ...previous,
      prevSummary: '',
      prevTail: '',
      prevError: '',
    }))
    if (!prev) {
      setLoadingPrev(false)
      return
    }
    const cached = cacheRef.current[prev.id]
    if (cached) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        prevRequestRef.current !== request
      ) return
      setIntel((previous) => ({
        ...previous,
        prevSummary: cached.summary,
        prevTail: lastRunes(cached.body, PREV_TAIL_RUNES),
        prevError: '',
      }))
      setLoadingPrev(false)
      return
    }
    setLoadingPrev(true)
    try {
      const data = await api.getChapter(prev.id)
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        prevRequestRef.current !== request ||
        data.id !== prev.id
      ) return
      cacheRef.current = { ...cacheRef.current, [data.id]: data }
      setIntel((previous) => ({
        ...previous,
        prevSummary: data.summary,
        prevTail: lastRunes(data.body, PREV_TAIL_RUNES),
        prevError: '',
      }))
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        prevRequestRef.current !== request
      ) return
      setIntel((previous) => ({
        ...previous,
        prevSummary: '',
        prevTail: '',
        prevError: err instanceof APIError ? err.message : '上一章读取失败',
      }))
    } finally {
      if (
        isCurrentRequest(epoch, targetProjectId, targetChapterId) &&
        prevRequestRef.current === request
      ) setLoadingPrev(false)
    }
  }

  async function loadBible(
    epoch = loadEpochRef.current,
    targetProjectId = projectId,
    targetChapterId = idOfChapter,
  ) {
    if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
    const request = ++bibleRequestRef.current
    setLoadingBible(true)
    setIntel((previous) => ({ ...previous, bible: [], bibleError: '' }))
    try {
      const res = await api.listBible(targetProjectId)
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        bibleRequestRef.current !== request
      ) return
      setIntel((previous) => ({ ...previous, bible: res.entries ?? [], bibleError: '' }))
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        bibleRequestRef.current !== request
      ) return
      setIntel((previous) => ({
        ...previous,
        bible: [],
        bibleError: err instanceof APIError ? err.message : '设定集读取失败',
      }))
    } finally {
      if (
        isCurrentRequest(epoch, targetProjectId, targetChapterId) &&
        bibleRequestRef.current === request
      ) setLoadingBible(false)
    }
  }

  async function loadCards(
    epoch = loadEpochRef.current,
    targetProjectId = projectId,
    targetChapterId = idOfChapter,
  ) {
    if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
    const request = ++cardsRequestRef.current
    setLoadingCards(true)
    setCardsError('')
    try {
      const response = await api.listCards(targetProjectId)
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        cardsRequestRef.current !== request
      ) return
      setCards(response.cards ?? [])
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        cardsRequestRef.current !== request
      ) return
      setCards([])
      setCardsError(err instanceof APIError ? err.message : '牌库读取失败')
    } finally {
      if (
        isCurrentRequest(epoch, targetProjectId, targetChapterId) &&
        cardsRequestRef.current === request
      ) setLoadingCards(false)
    }
  }

  async function loadNeighbors(
    epoch = loadEpochRef.current,
    targetProjectId = projectId,
    targetChapterId = idOfChapter,
  ) {
    if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
    const request = ++chaptersRequestRef.current
    setLoadingChapters(true)
    setChaptersError('')
    try {
      const response = await api.listChapters(targetProjectId)
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        chaptersRequestRef.current !== request
      ) return
      const around = neighbors(response.chapters ?? [], targetChapterId)
      setIntel((previous) => ({ ...previous, prev: around.prev, next: around.next }))
      void loadPrev(around.prev, epoch, targetProjectId, targetChapterId)
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
        chaptersRequestRef.current !== request
      ) return
      ++prevRequestRef.current
      setLoadingPrev(false)
      setIntel((previous) => ({
        ...previous,
        prev: null,
        next: null,
        prevSummary: '',
        prevTail: '',
        prevError: '',
      }))
      setChaptersError(err instanceof APIError ? err.message : '章节列表读取失败')
    } finally {
      if (
        isCurrentRequest(epoch, targetProjectId, targetChapterId) &&
        chaptersRequestRef.current === request
      ) setLoadingChapters(false)
    }
  }

  async function load() {
    if (!idOfChapter || !projectId) return
    const targetProjectId = projectId
    const targetChapterId = idOfChapter
    const epoch = ++loadEpochRef.current
    ++cardsRequestRef.current
    ++chaptersRequestRef.current
    ++cardRequestRef.current
    ++prevRequestRef.current
    ++bibleRequestRef.current
    cacheRef.current = {}
    setCh(null)
    setCards([])
    setIntel(emptyIntel())
    setIntelOpen(false)
    setLoadingCards(false)
    setLoadingChapters(false)
    setLoadingCard(false)
    setLoadingPrev(false)
    setLoadingBible(false)
    setTitle('')
    setBrief('')
    setBody('')
    setSummary('')
    setCardId('')
    setTarget(2500)
    setModel('')
    setModelBaseline('')
    setNote('')
    setPageError('')
    setCardsError('')
    setChaptersError('')
    setOperationError('')
    replaceOperation('idle')
    setActiveJob(null)
    setJobPollKey(0)
    setJobError('')
    setRefreshError('')
    setChapterPollPaused(false)
    setSetupOpen(false)

    void loadCards(epoch, targetProjectId, targetChapterId)
    void loadNeighbors(epoch, targetProjectId, targetChapterId)
    void loadBible(epoch, targetProjectId, targetChapterId)
    try {
      const data = await api.getChapter(targetChapterId)
      if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
      if (data.id !== targetChapterId || data.project_id !== targetProjectId) {
        setPageError('章节身份不匹配，请重试。')
        return
      }
      applyChapter(data)
      setPageError('')
      if (data.status === 'writing') {
        replaceOperation('generating')
      } else if (data.status === 'failed') {
        setJobError('上一次生成失败，可以调整设定后重试。')
      }
      void loadCard(data.card_id, epoch, targetProjectId, targetChapterId)
    } catch (err) {
      if (!isCurrentRequest(epoch, targetProjectId, targetChapterId)) return
      setPageError(err instanceof APIError ? err.message : '加载章节失败')
    }
  }

  useEffect(() => {
    void load()
    return () => {
      ++loadEpochRef.current
      ++cardsRequestRef.current
      ++chaptersRequestRef.current
      ++cardRequestRef.current
      ++prevRequestRef.current
      ++bibleRequestRef.current
    }
  }, [idOfChapter, projectId])

  function isCurrentOperation(token: number, targetProjectId: string, targetChapterId: string) {
    return (
      operationTokenRef.current === token &&
      isCurrentRequest(loadEpochRef.current, targetProjectId, targetChapterId)
    )
  }

  function failWritePreflight(
    token: number,
    message: string,
    field: { current: HTMLElement | null },
  ) {
    setOperationError(message)
    if (setupRef.current) setupRef.current.open = true
    setSetupOpen(true)
    moveOperation(token, 'idle')
    window.requestAnimationFrame(() => field.current?.focus())
  }

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!loadedChapter) return
    const token = beginOperation('saving')
    if (token === null) return
    const targetChapter = loadedChapter
    const epoch = loadEpochRef.current
    setOperationError('')
    try {
      const saved = await api.patchChapter(targetChapter.id, {
        title: title.trim(),
        brief: brief.trim(),
        body,
        summary,
        card_id: cardId,
        target_runes: target,
      })
      if (
        !isCurrentRequest(epoch, targetChapter.project_id, targetChapter.id) ||
        operationTokenRef.current !== token
      ) return
      if (saved.id !== targetChapter.id || saved.project_id !== targetChapter.project_id) {
        setOperationError('保存响应与当前章节不匹配，请重试。')
        return
      }
      applyChapter(saved, false)
      notify('已保存', 'success')
      if (saved.card_id !== intel.card?.id) {
        void loadCard(saved.card_id, epoch, targetChapter.project_id, targetChapter.id)
      }
    } catch (err) {
      if (
        !isCurrentRequest(epoch, targetChapter.project_id, targetChapter.id) ||
        operationTokenRef.current !== token
      ) return
      setOperationError(err instanceof APIError ? err.message : '保存失败')
    } finally {
      if (isCurrentRequest(epoch, targetChapter.project_id, targetChapter.id)) {
        moveOperation(token, 'idle')
      }
    }
  }

  async function onWrite() {
    if (!loadedChapter) return
    const token = beginOperation('submitting')
    if (token === null) return
    const targetChapter = loadedChapter
    const targetProjectId = targetChapter.project_id
    const targetChapterId = targetChapter.id
    const epoch = loadEpochRef.current
    setOperationError('')
    setJobError('')
    setRefreshError('')

    if (!cardId) {
      failWritePreflight(token, '先选择一张风格卡再写。', cardSelectRef)
      return
    }
    if (!brief.trim()) {
      failWritePreflight(token, '先填写章概括再写。', briefRef)
      return
    }

    if (targetChapter.body.trim() || body.trim()) {
      const ok = await confirm({
        title: '重写这一章？',
        body: '现有正文会被覆盖。未保存的手改也会丢掉。',
        confirmText: '重写',
        danger: true,
      })
      if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
      if (!ok) {
        moveOperation(token, 'idle')
        return
      }
    }

    let baseline = targetChapter
    if (generationSettingsDirty) {
      try {
        const saved = await api.patchChapter(targetChapterId, {
          title: title.trim(),
          brief: brief.trim(),
          card_id: cardId,
          target_runes: target,
        })
        if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
        if (saved.id !== targetChapterId || saved.project_id !== targetProjectId) {
          setOperationError('保存响应与当前章节不匹配，请重试。')
          moveOperation(token, 'idle')
          return
        }
        baseline = saved
        setChapterBaseline(saved)
        if (saved.card_id !== intel.card?.id) {
          void loadCard(saved.card_id, epoch, targetProjectId, targetChapterId)
        }
      } catch (err) {
        if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
        const message = err instanceof APIError ? err.message : '未能保存设定后再写'
        setOperationError(message)
        notify(message, 'error')
        moveOperation(token, 'idle')
        return
      }
    }

    if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
    try {
      const res = await api.writeChapter(targetChapterId, model.trim(), target, note.trim())
      if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
      const job: ActiveJob = {
        id: res.job_id,
        projectId: targetProjectId,
        chapterId: targetChapterId,
        operationToken: token,
      }
      setActiveJob(job)
      setJobPollKey(0)
      setChapterBaseline({ ...baseline, status: 'writing' })
      setNote('')
      setModelBaseline(model)
      registerJob({
        jobId: res.job_id,
        projectId: targetProjectId,
        kind: 'write',
        label: `写第 ${targetChapter.seq} 章「${title}」`,
        startedAt: Date.now(),
      })
      moveOperation(token, 'generating')
      notify('写章任务已提交', 'info')
    } catch (err) {
      if (!isCurrentOperation(token, targetProjectId, targetChapterId)) return
      const message = err instanceof APIError ? err.message : '提交失败'
      setOperationError(message)
      notify(message, 'error')
      moveOperation(token, 'idle')
    }
  }

  async function refreshAfterJob(job: ActiveJob) {
    if (
      !isCurrentOperation(job.operationToken, job.projectId, job.chapterId) ||
      (operationRef.current !== 'generating' && operationRef.current !== 'refreshing')
    ) return
    moveOperation(job.operationToken, 'refreshing')
    setRefreshError('')
    try {
      const data = await api.getChapter(job.chapterId)
      if (!isCurrentOperation(job.operationToken, job.projectId, job.chapterId)) return
      if (data.id !== job.chapterId || data.project_id !== job.projectId) {
        setRefreshError('生成已完成，但刷新结果与当前章节不匹配，请重试。')
        return
      }
      if (data.status === 'writing') {
        setRefreshError('生成已完成，但最新正文尚未就绪，请重试刷新。')
        return
      }
      applyChapter(data)
      setActiveJob(null)
      setJobError('')
      setRefreshError('')
      moveOperation(job.operationToken, 'idle')
    } catch (err) {
      if (!isCurrentOperation(job.operationToken, job.projectId, job.chapterId)) return
      setRefreshError(
        err instanceof APIError ? `生成已完成，刷新失败：${err.message}` : '生成已完成，刷新失败。',
      )
    }
  }

  function onJobStatus(job: ActiveJob, status: JobStatus) {
    if (!isCurrentOperation(job.operationToken, job.projectId, job.chapterId)) return
    if (status === 'succeeded') {
      void refreshAfterJob(job)
      return
    }
    if (status === 'failed' || status === 'canceled') {
      setActiveJob(null)
      setJobError(status === 'failed' ? '生成任务失败，可以调整后重试。' : '生成任务已取消。')
      moveOperation(job.operationToken, 'idle')
    }
  }

  function onJobPollError(job: ActiveJob, message: string) {
    if (!isCurrentOperation(job.operationToken, job.projectId, job.chapterId)) return
    setJobError(message)
  }

  useEffect(() => {
    const targetChapter = loadedChapter
    if (
      !targetChapter ||
      targetChapter.status !== 'writing' ||
      activeJob ||
      operation !== 'generating' ||
      chapterPollPaused
    ) return

    const epoch = loadEpochRef.current
    const token = operationTokenRef.current
    const targetProjectId = targetChapter.project_id
    const targetChapterId = targetChapter.id
    let stopped = false
    let timer: number | undefined

    async function tick() {
      try {
        const data = await api.getChapter(targetChapterId)
        if (
          stopped ||
          !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
          operationTokenRef.current !== token ||
          data.id !== targetChapterId ||
          data.project_id !== targetProjectId
        ) return
        if (data.status === 'writing') {
          timer = window.setTimeout(() => void tick(), CHAPTER_POLL_MS)
          return
        }
        moveOperation(token, 'refreshing')
        applyChapter(data)
        setChapterPollPaused(false)
        if (data.status === 'failed') {
          setJobError('生成任务失败，可以调整后重试。')
        } else {
          setJobError('')
        }
        moveOperation(token, 'idle')
      } catch (err) {
        if (
          stopped ||
          !isCurrentRequest(epoch, targetProjectId, targetChapterId) ||
          operationTokenRef.current !== token
        ) return
        setJobError(err instanceof APIError ? err.message : '无法读取章节生成状态')
        setChapterPollPaused(true)
      }
    }

    timer = window.setTimeout(() => void tick(), CHAPTER_POLL_MS)
    return () => {
      stopped = true
      if (timer !== undefined) window.clearTimeout(timer)
    }
  }, [loadedChapter, activeJob, operation, chapterPollPaused])

  async function navigateWithGuard(to: string, fromIntelDialog = false) {
    if (!to || leavePromptRef.current) return
    if (!leaveDirty) {
      setIntelOpen(false)
      navigate(to)
      return
    }

    const routeAtPrompt = { ...routeRef.current }
    if (fromIntelDialog) {
      setIntelOpen(false)
      await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
    }

    leavePromptRef.current = true
    let ok = false
    try {
      ok = await confirm({
        title: '离开会丢掉尚未保存的内容',
        body: dirty && generationDraftDirty
          ? '当前持久修改以及本次生成模型或补充都还没有保存或使用。'
          : generationDraftDirty
            ? '本次生成模型或补充尚未使用。'
            : '当前对标题、概括、正文或摘要的调整还没有保存。',
        confirmText: '离开',
        danger: true,
      })
    } finally {
      leavePromptRef.current = false
    }

    const routeUnchanged =
      routeRef.current.projectId === routeAtPrompt.projectId &&
      routeRef.current.chapterId === routeAtPrompt.chapterId
    if (!routeUnchanged) return
    if (!ok) {
      if (fromIntelDialog) setIntelOpen(true)
      return
    }
    setIntelOpen(false)
    navigate(to)
  }

  async function goTo(nextId?: string) {
    if (!nextId) return
    await navigateWithGuard('/p/' + projectId + '/chapter/' + nextId)
  }

  const briefCount = [...brief].length
  const noteCount = [...note].length
  const bodyCount = [...body].length
  const hasExistingBody = Boolean(loadedChapter?.body.trim() || body.trim())
  const navigationLocked =
    operation === 'saving' || operation === 'submitting' || operation === 'refreshing'
  const selectedCard = cards.find((card) => card.id === cardId)
  const operationLabel: Record<Operation, string> = {
    idle: dirty ? '有未保存修改' : generationDraftDirty ? '有本次生成草稿' : '可以编辑',
    saving: '正在保存',
    submitting: '正在提交生成任务',
    generating: '正在生成正文',
    refreshing: '正在刷新最新正文',
  }

  return (
    <div className="page page-wide chapter-page">
      <Crumb projectId={projectId} current={loadedChapter ? `第 ${loadedChapter.seq} 章` : '章节'} />
      <div className="page-head chapter-page-head">
        <div className="chapter-head-copy">
          <p className="kicker">CHAPTER</p>
          <h1>{loadedChapter ? `第 ${loadedChapter.seq} 章 ${title || loadedChapter.title}` : '章节'}</h1>
          <p className="sub">对照风格卡和前情写这一章。手改正文和摘要会留给下一章。</p>
          {loadedChapter ? (
            <nav className="chapter-head-nav" aria-label="章节导航">
              <button
                className="btn secondary sm"
                type="button"
                disabled={!intel.prev || loadingChapters || navigationLocked}
                onClick={() => void goTo(intel.prev?.id)}
              >
                <ArrowLeft size={15} />上一章
              </button>
              <button
                className="btn secondary sm"
                type="button"
                disabled={navigationLocked}
                onClick={() => void navigateWithGuard('/p/' + projectId + '/write')}
              >
                <BookOpenText size={15} />回章程
              </button>
              <button
                className="btn secondary sm"
                type="button"
                disabled={!intel.next || loadingChapters || navigationLocked}
                onClick={() => void goTo(intel.next?.id)}
              >
                下一章<ArrowRight size={15} />
              </button>
            </nav>
          ) : null}
          {loadingChapters && loadedChapter ? <p className="muted chapter-neighbor-state">正在读取相邻章节…</p> : null}
          {chaptersError && loadedChapter ? (
            <div className="chapter-neighbor-state">
              <span className="error">{chaptersError}</span>
              <button className="btn secondary sm" type="button" onClick={() => void loadNeighbors()}>
                重试
              </button>
            </div>
          ) : null}
        </div>
        <div className="status-grid">
          <div className="ambient-picker-wrap">
            <button
              className={'btn secondary sm' + (ambientType ? ' ambient-active' : '')}
              type="button"
              onClick={() => setShowAmbientPicker((v) => !v)}
              title="环境白噪音发生器"
            >
              {ambientType === 'rain' ? <CloudRain size={14} color="var(--gold-hi)" /> : ambientType === 'breeze' ? <Wind size={14} color="var(--jade-hi)" /> : ambientType === 'cosmic' ? <Sparkles size={14} color="var(--violet-hi)" /> : <Music size={14} />}
              <span>{ambientType === 'rain' ? '夜雨' : ambientType === 'breeze' ? '松涛' : ambientType === 'cosmic' ? '星海' : '白噪音'}</span>
            </button>
            {showAmbientPicker && (
              <div className="ambient-dropdown">
                <div className="ambient-options">
                  <button type="button" className={ambientType === 'rain' ? 'active' : ''} onClick={() => toggleAmbient('rain')}>
                    <CloudRain size={14} /> 🌧️ 夜雨打窗
                  </button>
                  <button type="button" className={ambientType === 'breeze' ? 'active' : ''} onClick={() => toggleAmbient('breeze')}>
                    <Wind size={14} /> 🌲 古刹松涛
                  </button>
                  <button type="button" className={ambientType === 'cosmic' ? 'active' : ''} onClick={() => toggleAmbient('cosmic')}>
                    <Sparkles size={14} /> 🌌 星海空灵
                  </button>
                  {ambientType && (
                    <button type="button" className="mute" onClick={() => { ambient.stop(); setAmbientType(null) }}>
                      <VolumeX size={14} /> 关闭声音
                    </button>
                  )}
                </div>
                {ambientType && (
                  <div className="ambient-vol-slider">
                    <Volume2 size={13} color="var(--ink-soft)" />
                    <input
                      type="range"
                      min={0}
                      max={1}
                      step={0.05}
                      value={ambientVol}
                      onChange={(e) => {
                        const v = Number(e.target.value)
                        setAmbientVol(v)
                        ambient.setVolume(v)
                      }}
                    />
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="status-chip">
            <span>字数</span>
            <strong>{bodyCount}</strong>
          </div>
          <div className="status-chip">
            <span>目标</span>
            <strong>{target}</strong>
          </div>
        </div>
      </div>

      {pageError && !loadedChapter ? (
        <div className="empty-state">
          <h2>章节加载失败</h2>
          <p className="error">{pageError}</p>
          <button className="btn secondary" type="button" onClick={() => void load()}>重试</button>
        </div>
      ) : !loadedChapter ? (
        <div className="chapter-workspace">
          <Skeleton h={72} />
          <Skeleton h={360} />
          <Skeleton h={280} />
        </div>
      ) : zenMode ? (
        <div className="zen-mode-overlay">
          <div className="zen-floating-bar">
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
              <span className="chapter-seq-badge">第 {loadedChapter?.seq} 章</span>
              <span style={{ fontWeight: 700, color: 'var(--ink)' }}>{title}</span>
              <span style={{ fontSize: '0.82rem', color: 'var(--ink-faint)', fontFamily: 'var(--mono)' }}>
                {bodyCount} 字 · 预计阅读 {Math.ceil(bodyCount / 350)} 分钟
              </span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <button
                className={'btn secondary sm' + (ambientType ? ' ambient-active' : '')}
                type="button"
                onClick={() => setShowAmbientPicker((v) => !v)}
              >
                <Music size={14} />
                <span>{ambientType === 'rain' ? '夜雨' : ambientType === 'breeze' ? '松涛' : ambientType === 'cosmic' ? '星海' : '白噪音'}</span>
              </button>
              <button className="btn sm" type="button" onClick={(e) => void onSave(e)} disabled={editingLocked || !dirty}>
                <Save size={14} />{operation === 'saving' ? '保存中…' : '保存'}
              </button>
              <button className="btn secondary sm" type="button" onClick={() => setZenMode(false)} title="退出禅模式 (Esc)">
                <Minimize2 size={14} /> 退出 (Esc)
              </button>
            </div>
          </div>

          <div className="zen-canvas">
            <textarea
              ref={bodyTextareaRef}
              className="zen-textarea"
              value={body}
              placeholder="落笔成篇，静候灵感流淌…"
              readOnly={editingLocked}
              onSelect={handleBodySelect}
              onMouseUp={handleBodySelect}
              onKeyUp={handleBodySelect}
              onChange={(e) => setBody(e.target.value)}
              autoFocus
            />
          </div>

          <InlineAIMenu
            selectedText={selectedText}
            card={intel.card}
            position={selectionPos}
            open={inlineAIOpen}
            onReplace={handleInlineReplace}
            onAppend={handleInlineAppend}
            onClose={() => setInlineAIOpen(false)}
          />
        </div>
      ) : (
        <form className="chapter-workspace" onSubmit={onSave}>
          <div className="chapter-command-bar">
            <div className="chapter-command-status" aria-live="polite">
              <span className={`chapter-operation ${operation}`}>{operationLabel[operation]}</span>
              <span className="muted">
                {cardId ? '风格卡已选' : '缺风格卡'} · {brief.trim() ? '章概括已填' : '缺章概括'} · 目标 {target}
              </span>
              {operationError ? <span className="error">{operationError}</span> : null}
            </div>
            <div className="chapter-command-actions">
              <button
                className="btn secondary"
                type="button"
                onClick={() => setRhythmOpen(true)}
                title="行文节奏与长短句起伏波形透视仪"
              >
                <Activity size={15} />节奏透视
              </button>
              <button
                className="btn secondary"
                type="button"
                onClick={() => setBranchOpen(true)}
                title="AI 剧情破局走向推演与智能续写"
                style={{ color: 'var(--gold-hi)', borderColor: 'rgba(212, 175, 55, 0.4)' }}
              >
                <GitFork size={15} />灵感推演
              </button>
              <button
                className="btn secondary"
                type="button"
                onClick={() => setAudioOpen(true)}
                disabled={!body.trim()}
                title="自然语音沉浸朗读本章"
              >
                <Headphones size={15} />沉浸朗读
              </button>
              <button
                className="btn secondary"
                type="button"
                onClick={() => setZenMode(true)}
                title="全屏极简沉浸禅模式 (Esc退出)"
              >
                <Maximize2 size={15} />禅模式
              </button>
              <button
                ref={intelOpenerRef}
                className="btn secondary intel-open"
                type="button"
                aria-haspopup="dialog"
                aria-expanded={intelOpen}
                onClick={() => setIntelOpen(true)}
              >
                <PanelRightOpen size={16} />风格卡 / 前情
              </button>
              <button className="btn" type="submit" disabled={editingLocked || !dirty}>
                <Save size={16} />{operation === 'saving' ? '保存中…' : '保存'}
              </button>
              <button className="btn secondary" type="button" onClick={() => void onWrite()} disabled={editingLocked}>
                <WandSparkles size={16} />
                {operation === 'submitting'
                  ? '提交中…'
                  : operation === 'generating'
                    ? '生成中…'
                    : operation === 'refreshing'
                      ? '刷新正文…'
                      : hasExistingBody
                        ? '重写这一章'
                        : '写这一章'}
              </button>
            </div>
          </div>

          <section className="chapter-workbench-main">
            <details
              ref={setupRef}
              className="chapter-setup work-zone"
              open={setupOpen}
              onToggle={(event) => setSetupOpen(event.currentTarget.open)}
            >
              <summary>
                <span>本章设定</span>
                <span className="chapter-readiness">
                  <span data-ready={Boolean(cardId)}>
                    {cardId ? selectedCard?.name ?? intel.card?.name ?? '风格卡已选' : '未选风格卡'}
                  </span>
                  <span data-ready={Boolean(brief.trim())}>{brief.trim() ? '章概括已填' : '章概括为空'}</span>
                  <span>目标 {target}</span>
                </span>
              </summary>
              <div className="stack">
                <div className="row">
                  <label style={{ flex: 1 }}>
                    标题
                    <input
                      type="text"
                      value={title}
                      maxLength={40}
                      disabled={editingLocked}
                      onChange={(e) => setTitle(e.target.value)}
                    />
                  </label>
                  <label>
                    风格卡
                    <select ref={cardSelectRef} value={cardId} disabled={editingLocked} onChange={(e) => {
                      const next = e.target.value
                      setCardId(next)
                      void loadCard(next)
                    }}>
                      <option value="">未选</option>
                      {cardId && !cards.some((card) => card.id === cardId) ? (
                        <option value={cardId}>当前风格卡</option>
                      ) : null}
                      {cards.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name} · v{c.current_version}
                        </option>
                      ))}
                    </select>
                    {loadingCards ? <span className="muted">正在读取牌库…</span> : null}
                    {cardsError ? (
                      <span className="error">
                        {cardsError}{' '}
                        <button className="btn secondary sm" type="button" onClick={() => void loadCards()}>
                          重试
                        </button>
                      </span>
                    ) : null}
                  </label>
                  <label>
                    本次生成模型（留空使用默认）
                    <input
                      type="text"
                      value={model}
                      disabled={editingLocked}
                      onChange={(e) => setModel(e.target.value)}
                    />
                  </label>
                </div>
                <label>
                  章概括（{briefCount}/200）
                  <textarea
                    ref={briefRef}
                    value={brief}
                    maxLength={200}
                    rows={3}
                    disabled={editingLocked}
                    onChange={(e) => setBrief(e.target.value)}
                  />
                </label>
                <label>
                  目标字数 {target}
                  <input
                    type="range"
                    min={2000}
                    max={3500}
                    step={100}
                    value={target}
                    disabled={editingLocked}
                    onChange={(e) => setTarget(Number(e.target.value))}
                  />
                </label>
                <label>
                  这一次补充（{noteCount}/200）
                  <textarea
                    value={note}
                    maxLength={200}
                    rows={2}
                    disabled={editingLocked}
                    placeholder="只作用于下一次生成，例如：少写对白 / 收在门外"
                    onChange={(e) => setNote(e.target.value)}
                  />
                </label>
              </div>
            </details>
            <label className="chapter-body-field">
              正文（{bodyCount} 字）
              <textarea
                ref={bodyTextareaRef}
                className="chapter-body"
                value={body}
                rows={18}
                readOnly={editingLocked}
                onSelect={handleBodySelect}
                onMouseUp={handleBodySelect}
                onKeyUp={handleBodySelect}
                onChange={(e) => setBody(e.target.value)}
              />
            </label>
            {loadedChapter ? (
              <ScenePanel
                projectId={projectId}
                chapterId={loadedChapter.id}
                savedBody={loadedChapter.body}
                dirty={body !== loadedChapter.body}
                version={loadedChapter.updated_at}
                bodyRef={bodyTextareaRef}
              />
            ) : null}
            <label>
              摘要（留给下一章）
              <textarea
                value={summary}
                rows={3}
                disabled={editingLocked}
                onChange={(e) => setSummary(e.target.value)}
              />
            </label>
            {activeJob ? (
              <JobProgress
                key={`${activeJob.id}:${jobPollKey}`}
                jobId={activeJob.id}
                projectId={activeJob.projectId}
                autoNavigate={false}
                onStatus={(status) => onJobStatus(activeJob, status)}
                onPollError={(message) => onJobPollError(activeJob, message)}
              />
            ) : null}
            {jobError ? (
              <div className="stack">
                <p className="error">{jobError}</p>
                {activeJob && operation === 'generating' ? (
                  <button
                    className="btn secondary sm"
                    type="button"
                    onClick={() => {
                      setJobError('')
                      setJobPollKey((value) => value + 1)
                    }}
                  >
                    重试任务进度
                  </button>
                ) : null}
                {!activeJob && operation === 'generating' && chapterPollPaused ? (
                  <button
                    className="btn secondary sm"
                    type="button"
                    onClick={() => {
                      setJobError('')
                      setChapterPollPaused(false)
                    }}
                  >
                    重试生成状态
                  </button>
                ) : null}
              </div>
            ) : null}
            {refreshError && activeJob ? (
              <div className="stack">
                <p className="error">{refreshError}</p>
                <button
                  className="btn secondary sm"
                  type="button"
                  onClick={() => void refreshAfterJob(activeJob)}
                >
                  重试刷新正文
                </button>
              </div>
            ) : null}
          </section>

          <ChapterIntelPanel
            projectId={projectId}
            intel={intel}
            loadingCard={loadingCard}
            loadingPrev={loadingPrev}
            loadingBible={loadingBible}
            onRetryCard={() => void loadCard(cardId)}
            onRetryPrev={() => void loadPrev(intel.prev)}
            onRetryBible={() => void loadBible()}
            onNavigate={(to) => navigateWithGuard(to)}
            variant="aside"
          />

          <ChapterIntelPanel
            projectId={projectId}
            intel={intel}
            loadingCard={loadingCard}
            loadingPrev={loadingPrev}
            loadingBible={loadingBible}
            onRetryCard={() => void loadCard(cardId)}
            onRetryPrev={() => void loadPrev(intel.prev)}
            onRetryBible={() => void loadBible()}
            variant="drawer"
            open={intelOpen}
            onClose={() => setIntelOpen(false)}
            openerRef={intelOpenerRef}
            onNavigate={(to) => navigateWithGuard(to, true)}
          />

          <InlineAIMenu
            selectedText={selectedText}
            card={intel.card}
            position={selectionPos}
            open={inlineAIOpen}
            onReplace={handleInlineReplace}
            onAppend={handleInlineAppend}
            onClose={() => setInlineAIOpen(false)}
          />

          <RhythmVisualizer
            body={body}
            card={intel.card}
            open={rhythmOpen}
            onClose={() => setRhythmOpen(false)}
          />

          <BranchSimulationDrawer
            chapterId={idOfChapter}
            projectId={projectId}
            currentText={body}
            isOpen={branchOpen}
            onClose={() => setBranchOpen(false)}
            onApplyContinuation={(continued) => {
              setBody((prev) => {
                const trimmed = prev.trimEnd()
                return trimmed ? `${trimmed}\n\n${continued}` : continued
              })
            }}
          />

          {audioOpen && (
            <AudioNarrationBar
              title={loadedChapter ? `第 ${loadedChapter.seq} 章 ${title || loadedChapter.title}` : title}
              text={body}
              onClose={() => setAudioOpen(false)}
            />
          )}
        </form>
      )}
    </div>
  )
}

