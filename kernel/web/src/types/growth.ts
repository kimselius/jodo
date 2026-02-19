export interface GrowthEvent {
  id: number
  event: string
  note: string
  git_hash?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface GallaEntry {
  id: number
  galla: number
  plan: string | null
  summary: string | null
  actions_count: number
  started_at: string
  completed_at: string | null
}
