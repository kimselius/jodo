import { request } from './request'

export const lifecycleApi = {
  restart() {
    return request<{ status: string }>('/api/restart', { method: 'POST' })
  },
}
