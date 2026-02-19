import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import { onWSEvent } from '@/composables/useWebSocket'
import type { GallaEntry } from '@/types/growth'

export function useGallas() {
  const gallas = ref<GallaEntry[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getGallas(100)
      // API returns DESC — reverse to ascending (oldest first, newest at bottom)
      gallas.value = data.gallas.reverse()
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load gallas'
    } finally {
      loading.value = false
    }
  }

  // Silent reload (no loading spinner) for WS-triggered updates
  async function reload() {
    try {
      const data = await api.getGallas(100)
      gallas.value = data.gallas.reverse()
      error.value = null
    } catch { /* ignore — next WS event will retry */ }
  }

  const unsub = onWSEvent((event) => {
    if (event.type === 'growth') reload()
  })

  onMounted(load)
  onUnmounted(unsub)

  return { gallas, loading, error, load }
}
