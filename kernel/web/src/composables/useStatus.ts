import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import type { StatusResponse, BudgetResponse } from '@/types/status'

export function useStatus(intervalMs = 10_000) {
  const status = ref<StatusResponse | null>(null)
  const budget = ref<BudgetResponse | null>(null)
  const error = ref<string | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null

  async function refresh() {
    try {
      const [s, b] = await Promise.all([
        api.getStatus(),
        api.getBudget()
      ])
      status.value = s
      budget.value = b
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch status'
    }
  }

  onMounted(() => {
    refresh()
    timer = setInterval(refresh, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { status, budget, error, refresh }
}
