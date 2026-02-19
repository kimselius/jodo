import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { MemoryEntry } from '@/types/memory'

export function useMemories() {
  const memories = ref<MemoryEntry[]>([])
  const total = ref(0)
  const loading = ref(true)
  const error = ref<string | null>(null)
  const offset = ref(0)
  const limit = 50

  async function load(reset = false) {
    if (reset) {
      offset.value = 0
      memories.value = []
    }
    loading.value = true
    try {
      const data = await api.getMemories(limit, offset.value)
      if (reset || offset.value === 0) {
        memories.value = data.memories
      } else {
        memories.value = [...memories.value, ...data.memories]
      }
      total.value = data.total
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load memories'
    } finally {
      loading.value = false
    }
  }

  function loadMore() {
    offset.value += limit
    load()
  }

  const hasMore = () => memories.value.length < total.value

  onMounted(() => load())

  return { memories, total, loading, error, load, loadMore, hasMore }
}
