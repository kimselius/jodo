import type { ChatMessage } from '@/types/chat'
import { request } from './request'

export const chatApi = {
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
}
