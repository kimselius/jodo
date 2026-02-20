import type { LLMCallSummary, LLMCallDetail } from '@/types/llmcalls'
import { request } from './request'

export const llmApi = {
  getLLMCalls(limit = 50, offset = 0, intent?: string) {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (intent) params.set('intent', intent)
    return request<{ calls: LLMCallSummary[]; total: number }>(`/api/llm-calls?${params}`)
  },

  getLLMCallDetail(id: number) {
    return request<LLMCallDetail>(`/api/llm-calls/${id}`)
  },
}
