import type { LibraryItem, LibraryComment } from '@/types/library'
import { request } from './request'

export const libraryApi = {
  getLibrary(status?: string) {
    const qs = status ? `?status=${status}` : ''
    return request<{ items: LibraryItem[] }>(`/api/library${qs}`)
  },

  createItem(title: string, content: string, priority = 0) {
    return request<{ ok: boolean; item: LibraryItem }>('/api/library', {
      method: 'POST',
      body: JSON.stringify({ title, content, priority }),
    })
  },

  updateItem(id: number, update: { title?: string; content?: string; priority?: number }) {
    return request<{ ok: boolean }>(`/api/library/${id}`, {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  patchStatus(id: number, status: string) {
    return request<{ ok: boolean }>(`/api/library/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
  },

  deleteItem(id: number) {
    return request<{ ok: boolean }>(`/api/library/${id}`, { method: 'DELETE' })
  },

  addComment(id: number, message: string) {
    return request<{ ok: boolean; comment: LibraryComment }>(`/api/library/${id}/comments`, {
      method: 'POST',
      body: JSON.stringify({ source: 'human', message }),
    })
  },

  getInbox(limit = 200) {
    return request<{ messages: Array<{ id: number; source: string; target: string; message: string; galla?: number; created_at: string }> }>(
      `/api/inbox?limit=${limit}`
    )
  },
}
