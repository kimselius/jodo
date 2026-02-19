<script setup lang="ts">
import { computed } from 'vue'
import Card from '@/components/ui/Card.vue'
import type { BudgetResponse } from '@/types/status'

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
</script>

<template>
  <Card class="p-4">
    <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">Budget</h3>

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
      </div>

      <div class="border-t border-border pt-3 flex items-center justify-between text-xs text-muted-foreground">
        <span>Spent today</span>
        <span>${{ budget.total_spent_today.toFixed(2) }}</span>
      </div>
    </div>
  </Card>
</template>
