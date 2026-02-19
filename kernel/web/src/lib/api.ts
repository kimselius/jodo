import type { ChatMessage } from '@/types/chat'
import type { StatusResponse, BudgetResponse } from '@/types/status'
import type { Genesis, IdentityUpdate } from '@/types/genesis'
import type { CommitEntry } from '@/types/history'
import type { MemoryEntry } from '@/types/memory'
import type { GrowthEvent } from '@/types/growth'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  // Chat
  getMessages(params?: Record<string, string>) {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return request<{ messages: ChatMessage[] }>(`/api/chat${qs}`)
  },

  sendMessage(message: string) {
    return request<{ ok: boolean; id: number }>('/api/chat', {
      method: 'POST',
      body: JSON.stringify({ message, source: 'human' }),
    })
  },

  ackMessages(upToId: number) {
    return request<{ ok: boolean; marked: number }>('/api/chat/ack', {
      method: 'POST',
      body: JSON.stringify({ up_to_id: upToId }),
    })
  },

  // Status
  getStatus() {
    return request<StatusResponse>('/api/status')
  },

  getBudget() {
    return request<BudgetResponse>('/api/budget')
  },

  // Genesis
  getGenesis() {
    return request<Genesis>('/api/genesis')
  },

  updateIdentity(update: IdentityUpdate) {
    return request<{ ok: boolean }>('/api/genesis/identity', {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  // History
  getHistory() {
    return request<{ commits: CommitEntry[] }>('/api/history')
  },

  // Memories
  getMemories(limit = 50, offset = 0) {
    return request<{ memories: MemoryEntry[]; total: number }>(
      `/api/memories?limit=${limit}&offset=${offset}`
    )
  },

  // Growth
  getGrowth(limit = 50) {
    return request<{ events: GrowthEvent[] }>(`/api/growth?limit=${limit}`)
  },

  // Lifecycle
  restart() {
    return request<{ status: string }>('/api/restart', { method: 'POST' })
  },
}
