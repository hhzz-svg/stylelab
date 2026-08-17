export type Me = {
  user_id: string
  email: string
}

export type Project = {
  id: string
  name: string
  created_at: string
}

export type Asset = {
  id: string
  filename: string
  sha256: string
  rune_count: number
  chapter_count: number
  created_at?: string
}

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

export type JobResult = {
  card_id?: string
  audit_id?: string
  sample_id?: string
}

export type Job = {
  id: string
  user_id: string
  project_id: string
  kind: string
  status: JobStatus
  progress: number
  stage: string
  payload?: unknown
  result?: JobResult | string | null
  error?: string
}

export type LLMKey = {
  provider: string
  base_url: string
  last4: string
}

export type APIErrorBody = {
  error?: {
    code?: string
    message?: string
  }
}
