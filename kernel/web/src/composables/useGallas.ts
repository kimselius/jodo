import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { GallaEntry } from '@/types/growth'

export function useGallas() {
  const gallas = ref<GallaEntry[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getGallas(100)
      gallas.value = data.gallas
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load gallas'
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return { gallas, loading, error, load }
}
