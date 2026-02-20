import { ref } from 'vue'
import { api } from '@/lib/api'

export interface MemorySearchResult {
  id: string
  content: string
  similarity: number
  tags: string[]
  created_at: string
}

export function useMemorySearch() {
  const query = ref('')
  const results = ref<MemorySearchResult[]>([])
  const searching = ref(false)
  const searchError = ref<string | null>(null)

  async function search() {
    if (!query.value.trim()) {
      results.value = []
      return
    }
    searching.value = true
    try {
      const data = await api.searchMemories(query.value.trim())
      results.value = data.results
      searchError.value = null
    } catch (e) {
      searchError.value = e instanceof Error ? e.message : 'Search failed'
    } finally {
      searching.value = false
    }
  }

  function clearSearch() {
    query.value = ''
    results.value = []
  }

  return { query, results, searching, searchError, search, clearSearch }
}
