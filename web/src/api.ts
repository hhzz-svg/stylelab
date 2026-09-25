import type {
  AnalysisWeights,
  APIErrorBody,
  Asset,
  AuditReport,
  BibleEntry,
  BibleEntrySummary,
  CardSummary,
  Chapter,
  ChapterSummary,
  Cooccurrence,
  GraphAnalysis,
  GraphData,
  GraphEdge,
  GraphNode,
  Job,
  LLMKey,
  Lineage,
  Me,
  OutlineChapterItem,
  ParentRef,
  Project,
  SampleChapter,
  StudioLatest,
  StyleCard,
} from './types'

export class APIError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

/** 网络层失败（断网、超时、DNS），status 固定 0，区别于服务端错误。 */
export class NetworkError extends Error {
  status = 0
  code = 'network'

  constructor(message: string) {
    super(message)
  }
}

export type ToastAction = { label: string; to: string }

/** 向全局 toast 栈广播一条消息。零依赖：UI 层通过监听事件渲染。 */
export function notify(
  message: string,
  kind: 'success' | 'error' | 'info' = 'info',
  action?: ToastAction,
) {
  window.dispatchEvent(new CustomEvent('app:toast', { detail: { message, kind, action } }))
}

const DEFAULT_TIMEOUT_MS = 30_000

/** Both APIError and NetworkError carry a message worth showing -- a bare
 *  `err instanceof APIError` check swallows "请求超时，请重试". */
export function errMessage(err: unknown, fallback: string): string {
  if (err instanceof APIError || err instanceof NetworkError) return err.message
  return fallback
}

async function parseError(res: Response): Promise<APIError> {
  let code = 'invalid'
  let message = res.statusText || 'request failed'
  try {
    const body = (await res.json()) as APIErrorBody
    if (body.error?.code) code = body.error.code
    if (body.error?.message) message = body.error.message
  } catch {
    // keep defaults
  }
  return new APIError(res.status, code, message)
}

/** Reads the download name out of a Content-Disposition header, preferring the
 *  RFC 5987 `filename*` form so a CJK project name survives. */
function filenameFromDisposition(header: string | null): string | undefined {
  if (!header) return undefined
  const extended = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (extended) {
    try {
      return decodeURIComponent(extended[1].trim())
    } catch {
      // fall through to the ASCII form
    }
  }
  const plain = /filename="?([^";]+)"?/i.exec(header)
  return plain ? plain[1].trim() : undefined
}

async function request<T>(path: string, init: RequestInit = {}, timeoutMs = DEFAULT_TIMEOUT_MS): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  let res: Response
  try {
    res = await fetch(path, {
      ...init,
      headers,
      credentials: 'include',
      signal: controller.signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new NetworkError('请求超时，请重试')
    }
    throw new NetworkError('无法连接服务，请检查网络')
  } finally {
    clearTimeout(timer)
  }
  if (res.status === 204) {
    return undefined as T
  }
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/api/auth/')) {
      // 会话过期：交给 App 统一软跳转登录页（保留当前路由，登录后回来）
      window.dispatchEvent(new CustomEvent('app:unauth'))
    }
    throw await parseError(res)
  }
  return (await res.json()) as T
}

export const api = {
  me: () => request<Me>('/api/me'),

  register: (email: string, password: string) =>
    request<{ user_id: string }>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  login: (email: string, password: string) =>
    request<{ user_id: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  logout: () => request<void>('/api/auth/logout', { method: 'POST' }),

  listProjects: () => request<{ projects: Project[] }>('/api/projects'),

  createProject: (name: string) =>
    request<Project>('/api/projects', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  getProject: (id: string) => request<Project>(`/api/projects/${id}`),

  deleteProject: (id: string) => request<void>(`/api/projects/${id}`, { method: 'DELETE' }),

  listAssets: (projectId: string) =>
    request<{ assets: Asset[] }>(`/api/projects/${projectId}/assets`),

  uploadAsset: async (projectId: string, file: File) => {
    const body = new FormData()
    body.append('file', file)
    return request<Asset>(`/api/projects/${projectId}/assets`, {
      method: 'POST',
      body,
    })
  },

  extract: (projectId: string, assetIds: string[], name: string, model: string) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/extract`, {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, name, model }),
    }),

  getJob: (id: string) => request<Job>(`/api/jobs/${id}`),

  listCards: (projectId: string) =>
    request<{ cards: CardSummary[] }>(`/api/projects/${projectId}/cards`),

  createCard: (
    projectId: string,
    body: {
      name: string
      kind?: string
      dimensions: Record<string, { level: number; summary: string; techniques: string[] }>
      prohibitions: string[]
    },
  ) =>
    request<StyleCard>(`/api/projects/${projectId}/cards`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  getCard: (id: string) => request<StyleCard>(`/api/cards/${id}`),

  getCardVersion: (id: string, n: number) =>
    request<StyleCard>(`/api/cards/${id}/versions/${n}`),

  saveCardVersion: (
    id: string,
    body: {
      name?: string
      levels: Record<string, number>
      rewrite_summaries: boolean
      model: string
    },
  ) =>
    request<StyleCard>(`/api/cards/${id}/versions`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  exportCard: async (id: string) => {
    const res = await fetch(`/api/cards/${id}/export`, { credentials: 'include' })
    if (!res.ok) {
      throw await parseError(res)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'simulation_profile.json'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },

  fuse: (projectId: string, name: string, parents: ParentRef[], model: string) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/fuse`, {
      method: 'POST',
      body: JSON.stringify({ name, parents, model }),
    }),

  auditCard: (id: string, model: string) =>
    request<{ job_id: string }>(`/api/cards/${id}/audit`, {
      method: 'POST',
      body: JSON.stringify({ model }),
    }),

  getAudit: (id: string) => request<AuditReport>(`/api/audits/${id}`),

  sampleCard: (id: string, premise: string, targetRunes: number, model: string) =>
    request<{ job_id: string }>(`/api/cards/${id}/sample`, {
      method: 'POST',
      body: JSON.stringify({ premise, target_runes: targetRunes, model }),
    }),

  getSample: (id: string) => request<SampleChapter>(`/api/samples/${id}`),

  listChapters: (projectId: string) =>
    request<{ chapters: ChapterSummary[] }>(`/api/projects/${projectId}/chapters`),

  createChapter: (projectId: string, title: string, brief: string, cardId: string) =>
    request<Chapter>(`/api/projects/${projectId}/chapters`, {
      method: 'POST',
      body: JSON.stringify({ title, brief, card_id: cardId }),
    }),

  getChapter: (id: string) => request<Chapter>(`/api/chapters/${id}`),

  patchChapter: (
    id: string,
    body: {
      title?: string
      brief?: string
      body?: string
      summary?: string
      card_id?: string
      target_runes?: number
    },
  ) =>
    request<Chapter>(`/api/chapters/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  deleteChapter: (id: string) => request<void>(`/api/chapters/${id}`, { method: 'DELETE' }),

  writeChapter: (id: string, model: string, targetRunes: number, note: string) =>
    request<{ job_id: string }>(`/api/chapters/${id}/write`, {
      method: 'POST',
      body: JSON.stringify({ model, target_runes: targetRunes, note }),
    }),

  listBible: (projectId: string) =>
    request<{ entries: BibleEntrySummary[] }>(`/api/projects/${projectId}/bible`),

  getBible: (id: string) => request<BibleEntry>(`/api/bible/${id}`),

  createBible: (
    projectId: string,
    kind: string,
    name: string,
    content: string,
    status: string,
  ) =>
    request<BibleEntry>(`/api/projects/${projectId}/bible`, {
      method: 'POST',
      body: JSON.stringify({ kind, name, content, status }),
    }),

  patchBible: (
    id: string,
    body: { kind?: string; name?: string; content?: string; status?: string },
  ) =>
    request<BibleEntry>(`/api/bible/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  deleteBible: (id: string) => request<void>(`/api/bible/${id}`, { method: 'DELETE' }),

  syncBible: (projectId: string, model: string) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/bible/sync`, {
      method: 'POST',
      body: JSON.stringify({ model }),
    }),

  listLLMKeys: () => request<{ keys: LLMKey[] }>('/api/me/llm-keys'),

  putLLMKey: (provider: string, apiKey: string, baseURL: string) =>
    request<{ provider: string; last4: string }>('/api/me/llm-keys', {
      method: 'PUT',
      body: JSON.stringify({ provider, api_key: apiKey, base_url: baseURL }),
    }),

  deleteLLMKey: (provider: string) =>
    request<void>(`/api/me/llm-keys/${provider}`, { method: 'DELETE' }),

  getGraph: (projectId: string) =>
    request<GraphData>(`/api/projects/${projectId}/graph`),

  saveGraphNode: (projectId: string, node: Partial<GraphNode>) =>
    request<{ id: string; ok: boolean }>(`/api/projects/${projectId}/graph/nodes`, {
      method: 'POST',
      body: JSON.stringify(node),
    }),

  deleteGraphNode: (projectId: string, nodeId: string) =>
    request<{ ok: boolean }>(`/api/projects/${projectId}/graph/nodes/${nodeId}`, {
      method: 'DELETE',
    }),

  saveGraphEdge: (projectId: string, edge: Partial<GraphEdge>) =>
    request<{ id: string; ok: boolean }>(`/api/projects/${projectId}/graph/edges`, {
      method: 'POST',
      body: JSON.stringify(edge),
    }),

  deleteGraphEdge: (projectId: string, edgeId: string) =>
    request<{ ok: boolean }>(`/api/projects/${projectId}/graph/edges/${edgeId}`, {
      method: 'DELETE',
    }),

  graphLineage: (projectId: string) =>
    request<Lineage>(`/api/projects/${projectId}/graph/lineage`),

  graphAnalysis: (projectId: string, weights: AnalysisWeights = 'graph') =>
    request<GraphAnalysis>(`/api/projects/${projectId}/graph/analysis?weights=${weights}`),

  graphCooccurrence: (projectId: string) =>
    request<Cooccurrence>(`/api/projects/${projectId}/graph/cooccurrence`),

  extractGraph: (projectId: string, model?: string) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/graph/extract`, {
      method: 'POST',
      body: JSON.stringify({ model }),
    }),

  generateOutline: (
    projectId: string,
    params: {
      premise: string
      genre?: string
      target_chapters?: number
      volume_count?: number
      model?: string
      card_id?: string
    },
  ) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/outline/generate`, {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  importOutline: (
    projectId: string,
    params: {
      chapters: OutlineChapterItem[]
      replace_existing?: boolean
      card_id?: string
    },
  ) =>
    request<{ ok: boolean; inserted_count: number; protected_count: number }>(
      `/api/projects/${projectId}/outline/import`,
      { method: 'POST', body: JSON.stringify(params) },
    ),

  branchSimulate: (chapterId: string, currentText: string, model?: string) =>
    request<{ job_id: string }>(`/api/chapters/${chapterId}/branch-simulate`, {
      method: 'POST',
      body: JSON.stringify({ current_text: currentText, model }),
    }),

  continueChapter: (
    chapterId: string,
    params: {
      current_text: string
      instruction: string
      target_runes?: number
      model?: string
    },
  ) =>
    request<{ job_id: string }>(`/api/chapters/${chapterId}/continue`, {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  continuityAudit: (projectId: string, model?: string) =>
    request<{ job_id: string }>(`/api/projects/${projectId}/continuity-audit`, {
      method: 'POST',
      body: JSON.stringify({ model }),
    }),

  /** Newest successful outline/audit for a project, or branch result for a
   *  chapter. `latest` is null when there has been none. */
  projectStudioLatest: <T>(projectId: string, kind: 'outline_generate' | 'continuity_audit') =>
    request<{ latest: StudioLatest<T> | null }>(`/api/projects/${projectId}/studio/latest?kind=${kind}`),

  chapterStudioLatest: <T>(chapterId: string) =>
    request<{ latest: StudioLatest<T> | null }>(`/api/chapters/${chapterId}/studio/latest?kind=branch_simulate`),

  downloadNovel: async (projectId: string, format: 'txt' | 'md' = 'txt') => {
    const res = await fetch(`/api/projects/${projectId}/export?format=${format}`, {
      credentials: 'include',
    })
    if (!res.ok) {
      if (res.status === 401) window.dispatchEvent(new CustomEvent('app:unauth'))
      throw await parseError(res)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    // The server names the file after the project; fall back only if it did not.
    a.download = filenameFromDisposition(res.headers.get('Content-Disposition')) ?? `novel.${format}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },
}


