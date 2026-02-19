export interface ChatMessage {
  id: number
  source: 'human' | 'jodo' | string
  message: string
  galla?: number | null
  read_at?: string | null
  created_at: string
}
