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
  chapter_id?: string
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

export const KIND_LABEL: Record<string, string> = {
  extracted: '抽离',
  fused: '融合',
  manual: '手调',
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

export type CardRef = {
  cardId: string
  version: number
}

export type BlendMode = 'balanced' | 'dominant' | 'custom'

export type FuseParentDraft = CardRef & {
  activeDims: Record<DimensionKey, boolean>
  weights: Record<DimensionKey, number>
}

export type FuseDraft = {
  name: string
  parents: FuseParentDraft[]
  mode: BlendMode
  dominantCardId?: string
  focusDimension: DimensionKey
  model: string
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

export type ChapterStatus = 'draft' | 'writing' | 'written' | 'failed'

export type ChapterSummary = {
  id: string
  seq: number
  title: string
  brief: string
  status: ChapterStatus
  rune_count: number
  has_summary: boolean
  card_id: string
  target_runes: number
  updated_at: string
}

export type Chapter = {
  id: string
  project_id: string
  card_id: string
  card_version: number
  seq: number
  title: string
  brief: string
  body: string
  summary: string
  status: ChapterStatus
  target_runes: number
  model: string
  created_at: string
  updated_at: string
}

export type BibleEntryKind = 'character' | 'setting' | 'thread'

export type BibleEntryStatus = 'active' | 'resolved'

export type BibleEntry = {
  id: string
  project_id: string
  kind: BibleEntryKind
  name: string
  content: string
  status: BibleEntryStatus
  origin: 'manual' | 'auto'
  source_seq: number
  created_at: string
  updated_at: string
}

export type BibleEntrySummary = {
  id: string
  kind: BibleEntryKind
  name: string
  status: BibleEntryStatus
  origin: 'manual' | 'auto'
  source_seq: number
  content_preview: string
  updated_at: string
}

export type GraphNodeKind = 'character' | 'faction' | 'artifact' | 'location'

export type GraphNode = {
  id: string
  project_id: string
  name: string
  kind: GraphNodeKind
  faction: string
  summary: string
  details: {
    realm?: string
    temperament?: string
    secrets?: string
    [key: string]: any
  }
  x: number
  y: number
  created_at: string
  updated_at: string
}

export type GraphEdge = {
  id: string
  project_id: string
  source_id: string
  target_id: string
  relation: string
  description: string
  strength: number
  created_at: string
  updated_at: string
}

export type GraphData = {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface OutlineChapterItem {
  title: string
  brief: string
  hook?: string
}

export interface OutlineVolumeItem {
  volume_index: number
  volume_title: string
  volume_brief: string
  chapters: OutlineChapterItem[]
}

export interface OutlineResponse {
  synopsis: string
  volumes: OutlineVolumeItem[]
}

export interface PlotBranch {
  id: string
  type: string
  title: string
  direction: string
  plot_points: string[]
  sample_opening: string
}

export interface BranchSimulateResponse {
  current_analysis: string
  branches: PlotBranch[]
}

export interface ContinuityIssue {
  severity: 'critical' | 'warning' | 'info'
  category: 'realm' | 'character' | 'foreshadow' | 'artifact'
  title: string
  description: string
  suggestion: string
  location: string
}

export interface ContinuityAuditResponse {
  score: number
  overall: string
  issues: ContinuityIssue[]
  foreshadows: string[]
}


