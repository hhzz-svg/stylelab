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

export const DIMENSION_KEYS = [
  'sentence_rhythm',
  'narrative_perspective',
  'dialogue_density',
  'sensory_description',
  'scene_pacing',
  'emotional_expression',
  'rhetoric_preference',
  'lexical_texture',
  'tension_hook',
] as const

export type DimensionKey = (typeof DIMENSION_KEYS)[number]

export const DIMENSION_LABELS: Record<DimensionKey, string> = {
  sentence_rhythm: '句式节奏',
  narrative_perspective: '叙事视角',
  dialogue_density: '对白密度',
  sensory_description: '感官描写',
  scene_pacing: '场景节奏',
  emotional_expression: '情绪表达',
  rhetoric_preference: '修辞偏好',
  lexical_texture: '语汇质地',
  tension_hook: '张力钩子',
}

export type Dimension = {
  level: number
  summary: string
  techniques: string[]
}

export type ParentRef = {
  card_id: string
  version: number
  dims: string[]
  weights: Record<string, number>
}

export type StyleCard = {
  id: string
  project_id: string
  name: string
  kind: string
  version: number
  dimensions: Record<string, Dimension>
  prohibitions: string[]
  facts?: unknown
  lineage?: {
    parent_cards: ParentRef[]
    prompt_version: string
  } | null
}

export type CardSummary = {
  id: string
  name: string
  kind: string
  current_version: number
  updated_at: string
}

export type PersonaView = {
  strengths: string[]
  risks: string[]
  suggestions: string[]
}

export type AuditConflict = {
  dimension: string
  summary: string
}

export type AuditEdit = {
  dimension: string
  target_level: number
  reason: string
}

export type AuditReport = {
  id?: string
  card_id: string
  card_version: number
  personas: string[]
  by_persona: Record<string, PersonaView>
  conflicts: AuditConflict[]
  recommended_edits: AuditEdit[]
}

export type SampleChapter = {
  premise: string
  body: string
  facts: unknown
  card_id: string
  card_version: number
}
