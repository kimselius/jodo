import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { GrowthEvent } from '@/types/growth'

export function useGrowth() {
  const events = ref<GrowthEvent[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getGrowth(100)
      events.value = data.events
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load growth log'
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return { events, loading, error, load }
}
