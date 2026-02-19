<script setup lang="ts">
import { computed } from 'vue'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'
import type { GrowthEvent } from '@/types/growth'

const props = defineProps<{ event: GrowthEvent }>()

const eventVariant = computed(() => {
  switch (props.event.event) {
    case 'boot': return 'success'
    case 'rebirth': return 'destructive'
    case 'restart': return 'warning'
    case 'rollback': return 'warning'
    case 'stable_tag': return 'success'
    case 'app_down': return 'destructive'
    default: return 'secondary'
  }
})

const eventColor = computed(() => {
  switch (props.event.event) {
    case 'boot': return 'bg-success'
    case 'rebirth': return 'bg-destructive'
    case 'restart': case 'rollback': return 'bg-warning'
    case 'stable_tag': return 'bg-success'
    case 'app_down': return 'bg-destructive'
    default: return 'bg-border'
  }
})
</script>

<template>
  <div class="flex gap-3">
    <!-- Timeline dot -->
    <div class="flex flex-col items-center">
      <div :class="['h-2.5 w-2.5 rounded-full mt-1.5 shrink-0', eventColor]" />
      <div class="w-px flex-1 bg-border" />
    </div>

    <!-- Content -->
    <div class="pb-4 min-w-0">
      <div class="flex items-center gap-2 flex-wrap">
        <Badge :variant="eventVariant" class="text-[10px]">{{ event.event }}</Badge>
        <code v-if="event.git_hash" class="text-[10px] text-muted-foreground font-mono">
          {{ event.git_hash.slice(0, 7) }}
        </code>
        <span class="text-[10px] text-muted-foreground">{{ formatTime(event.created_at) }}</span>
      </div>
      <p class="text-sm mt-0.5 break-words">{{ event.note }}</p>
    </div>
  </div>
</template>
