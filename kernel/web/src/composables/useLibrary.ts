import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import { onWSEvent } from '@/composables/useWebSocket'
import type { LibraryItem } from '@/types/library'

export function useLibrary() {
  const items = ref<LibraryItem[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getLibrary()
      items.value = data.items
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load library'
    } finally {
      loading.value = false
    }
  }

  // Silent reload for WS-triggered updates
  async function reload() {
    try {
      const data = await api.getLibrary()
      items.value = data.items
      error.value = null
    } catch { /* ignore — next WS event will retry */ }
  }

  async function create(title: string, content: string, priority = 0) {
    await api.createLibraryItem(title, content, priority)
    await reload()
  }

  async function update(id: number, updates: { title?: string; content?: string; priority?: number }) {
    await api.updateLibraryItem(id, updates)
    await reload()
  }

  async function remove(id: number) {
    await api.deleteLibraryItem(id)
    await reload()
  }

  async function comment(id: number, message: string) {
    await api.addLibraryComment(id, message)
    await reload()
  }

  async function patchStatus(id: number, status: string) {
    await api.patchLibraryStatus(id, status)
    await reload()
  }

  const unsub = onWSEvent((event) => {
    if (event.type === 'library') reload()
  })

  onMounted(load)
  onUnmounted(unsub)

  return { items, loading, error, load, create, update, remove, comment, patchStatus }
}
