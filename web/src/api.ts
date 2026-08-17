import type {
  APIErrorBody,
  Asset,
  AuditReport,
  CardSummary,
  Job,
  LLMKey,
  Me,
  ParentRef,
  Project,
  SampleChapter,
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

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(path, {
    ...init,
    headers,
    credentials: 'include',
  })
  if (res.status === 204) {
    return undefined as T
  }
  if (!res.ok) {
    throw await parseError(res)
  }
  if (res.status === 202 || res.headers.get('content-type')?.includes('application/json')) {
    return (await res.json()) as T
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

  listLLMKeys: () => request<{ keys: LLMKey[] }>('/api/me/llm-keys'),

  putLLMKey: (provider: string, apiKey: string, baseURL: string) =>
    request<{ provider: string; last4: string }>('/api/me/llm-keys', {
      method: 'PUT',
      body: JSON.stringify({ provider, api_key: apiKey, base_url: baseURL }),
    }),

  deleteLLMKey: (provider: string) =>
    request<void>(`/api/me/llm-keys/${provider}`, { method: 'DELETE' }),
}
