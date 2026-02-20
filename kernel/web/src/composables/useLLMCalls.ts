import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { LLMCallSummary, LLMCallDetail } from '@/types/llmcalls'

export function useLLMCalls() {
  const calls = ref<LLMCallSummary[]>([])
  const total = ref(0)
  const loading = ref(true)
  const error = ref<string | null>(null)
  const offset = ref(0)
  const limit = 50
  const intentFilter = ref('')

  async function load(reset = false) {
    if (reset) {
      offset.value = 0
      calls.value = []
    }
    loading.value = true
    try {
      const data = await api.getLLMCalls(limit, offset.value, intentFilter.value || undefined)
      if (reset || offset.value === 0) {
        calls.value = data.calls
      } else {
        calls.value = [...calls.value, ...data.calls]
      }
      total.value = data.total
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load LLM calls'
    } finally {
      loading.value = false
    }
  }

  function loadMore() {
    offset.value += limit
    load()
  }

  const hasMore = () => calls.value.length < total.value

  // Detail view
  const selectedCall = ref<LLMCallDetail | null>(null)
  const detailLoading = ref(false)

  async function loadDetail(id: number) {
    detailLoading.value = true
    try {
      selectedCall.value = await api.getLLMCallDetail(id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load call detail'
    } finally {
      detailLoading.value = false
    }
  }

  function toggleDetail(id: number) {
    if (selectedCall.value?.id === id) {
      selectedCall.value = null
      return
    }
    loadDetail(id)
  }

  function clearDetail() {
    selectedCall.value = null
  }

  onMounted(() => load())

  return {
    calls, total, loading, error, intentFilter,
    load, loadMore, hasMore,
    selectedCall, detailLoading, loadDetail, toggleDetail, clearDetail,
  }
}
