import type { MemoryEntry } from '@/types/memory'
import { request } from './request'

export const memoryApi = {
  getMemories(limit = 50, offset = 0) {
    return request<{ memories: MemoryEntry[]; total: number }>(
      `/api/memories?limit=${limit}&offset=${offset}`
    )
  },

  searchMemories(query: string, limit = 10) {
    const params = new URLSearchParams({ q: query, limit: String(limit) })
    return request<{ results: Array<{ id: string; content: string; similarity: number; tags: string[]; created_at: string }>; cost: number }>(`/api/memories/search?${params}`)
  },
}
