import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import { onWSEvent } from '@/composables/useWebSocket'

export interface InboxMessage {
  id: number
  source: string
  target: string
  message: string
  galla?: number
  created_at: string
}

export function useInbox() {
  const messages = ref<InboxMessage[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getInbox()
      messages.value = data.messages
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load inbox'
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    try {
      const data = await api.getInbox()
      messages.value = data.messages
      error.value = null
    } catch { /* next WS event will retry */ }
  }

  const unsub = onWSEvent((event) => {
    if (event.type === 'inbox') reload()
  })

  onMounted(load)
  onUnmounted(unsub)

  return { messages, loading, error, load }
}
