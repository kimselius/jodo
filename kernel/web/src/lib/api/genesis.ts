import type { Genesis, IdentityUpdate } from '@/types/genesis'
import { request } from './request'

export const genesisApi = {
  getGenesis() {
    return request<Genesis>('/api/genesis')
  },

  updateIdentity(update: IdentityUpdate) {
    return request<{ ok: boolean }>('/api/genesis/identity', {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },
}
