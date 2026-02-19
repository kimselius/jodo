<script setup lang="ts">
import { ref, computed } from 'vue'
import Card from '@/components/ui/Card.vue'
import type { BudgetResponse } from '@/types/status'
import { api } from '@/lib/api'

const props = defineProps<{ budget: BudgetResponse }>()

const providers = computed(() => {
  return Object.entries(props.budget.providers).map(([name, data]) => ({
    name,
    ...data,
    pct: data.monthly_budget > 0
      ? Math.min(100, (data.spent_this_month / data.monthly_budget) * 100)
      : 0,
  }))
})

// Breakdown
const expanded = ref(false)
const breakdown = ref<Array<{ provider: string; model: string; intent: string; calls: number; tokens_in: number; tokens_out: number; cost: number }>>([])
const loadingBreakdown = ref(false)

async function toggleBreakdown() {
  expanded.value = !expanded.value
  if (expanded.value && breakdown.value.length === 0) {
    loadingBreakdown.value = true
    try {
      const res = await api.getBudgetBreakdown()
      breakdown.value = res.breakdown || []
    } catch {
      // silently fail — non-critical
    } finally {
      loadingBreakdown.value = false
    }
  }
}

// Group breakdown by provider
const groupedBreakdown = computed(() => {
  const groups: Record<string, typeof breakdown.value> = {}
  for (const row of breakdown.value) {
    if (!groups[row.provider]) groups[row.provider] = []
    groups[row.provider].push(row)
  }
  return groups
})

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}
</script>

<template>
  <Card class="p-4">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Budget</h3>
      <button
        @click="toggleBreakdown"
        class="text-[10px] text-muted-foreground hover:text-foreground transition-colors"
      >
        {{ expanded ? 'Hide details' : 'Show details' }}
      </button>
    </div>

    <div class="space-y-4">
      <div v-for="p in providers" :key="p.name">
        <div class="flex items-center justify-between text-sm mb-1">
          <span class="capitalize">{{ p.name }}</span>
          <span class="text-xs text-muted-foreground">
            ${{ p.spent_this_month.toFixed(2) }} / ${{ p.monthly_budget.toFixed(2) }}
          </span>
        </div>
        <div class="h-1.5 rounded-full bg-secondary overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500"
            :class="p.pct > 80 ? 'bg-destructive' : p.pct > 50 ? 'bg-warning' : 'bg-primary'"
            :style="{ width: `${p.pct}%` }"
          />
        </div>
        <div class="flex justify-between text-[10px] text-muted-foreground mt-0.5">
          <span>Reserve: ${{ p.emergency_reserve.toFixed(2) }}</span>
          <span>${{ p.remaining.toFixed(2) }} left</span>
        </div>

        <!-- Per-model breakdown for this provider -->
        <div v-if="expanded && groupedBreakdown[p.name]?.length" class="mt-2 space-y-1">
          <div
            v-for="row in groupedBreakdown[p.name]"
            :key="`${row.model}-${row.intent}`"
            class="flex items-center justify-between text-[10px] bg-muted rounded px-2 py-1"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-mono text-foreground truncate">{{ row.model }}</span>
              <span class="text-muted-foreground">{{ row.intent }}</span>
            </div>
            <div class="flex items-center gap-3 flex-shrink-0">
              <span class="text-muted-foreground">{{ row.calls }} calls</span>
              <span class="text-muted-foreground">{{ formatTokens(row.tokens_in) }}in / {{ formatTokens(row.tokens_out) }}out</span>
              <span class="font-medium">${{ row.cost.toFixed(4) }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-if="expanded && loadingBreakdown" class="text-[10px] text-muted-foreground text-center py-2">
        Loading breakdown...
      </div>

      <div class="border-t border-border pt-3 flex items-center justify-between text-xs text-muted-foreground">
        <span>Spent today</span>
        <span>${{ budget.total_spent_today.toFixed(2) }}</span>
      </div>
    </div>
  </Card>
</template>
