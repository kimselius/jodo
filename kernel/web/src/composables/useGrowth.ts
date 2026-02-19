import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import { onWSEvent } from '@/composables/useWebSocket'
import type { GrowthEvent } from '@/types/growth'

export function useGrowth() {
  const events = ref<GrowthEvent[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getGrowth(100)
      // API returns DESC — reverse to ascending (oldest first, newest at bottom)
      events.value = data.events.reverse()
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load growth log'
    } finally {
      loading.value = false
    }
  }

  // Silent reload for WS-triggered updates
  async function reload() {
    try {
      const data = await api.getGrowth(100)
      events.value = data.events.reverse()
      error.value = null
    } catch { /* ignore — next WS event will retry */ }
  }

  const unsub = onWSEvent((event) => {
    if (event.type === 'growth') reload()
  })

  onMounted(load)
  onUnmounted(unsub)

  return { events, loading, error, load }
}
