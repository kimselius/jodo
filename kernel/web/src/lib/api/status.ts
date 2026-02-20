import type { StatusResponse, BudgetResponse } from '@/types/status'
import { request } from './request'

export const statusApi = {
  getStatus() {
    return request<StatusResponse>('/api/status')
  },

  getBudget() {
    return request<BudgetResponse>('/api/budget')
  },

  getBudgetBreakdown() {
    return request<{ breakdown: Array<{ provider: string; model: string; intent: string; calls: number; tokens_in: number; tokens_out: number; cost: number }> }>('/api/budget/breakdown')
  },
}
