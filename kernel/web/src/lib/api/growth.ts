import type { GrowthEvent, GallaEntry } from '@/types/growth'
import { request } from './request'

export const growthApi = {
  getGrowth(limit = 50) {
    return request<{ events: GrowthEvent[] }>(`/api/growth?limit=${limit}`)
  },

  getGallas(limit = 50) {
    return request<{ gallas: GallaEntry[] }>(`/api/galla?limit=${limit}`)
  },
}
