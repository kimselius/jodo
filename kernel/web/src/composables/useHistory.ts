import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { CommitEntry } from '@/types/history'

export function useHistory() {
  const commits = ref<CommitEntry[]>([])
  const loading = ref(true)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      const data = await api.getHistory()
      commits.value = data.commits || []
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load history'
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return { commits, loading, error, load }
}
