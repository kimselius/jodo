export interface GrowthEvent {
  id: number
  event: string
  note: string
  git_hash?: string
  metadata?: Record<string, unknown>
  created_at: string
}
