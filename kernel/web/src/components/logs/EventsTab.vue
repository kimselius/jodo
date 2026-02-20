<script setup lang="ts">
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import { useGrowth } from '@/composables/useGrowth'

const { events, loading, error, load } = useGrowth()

function fmtTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

type BadgeVariant = 'success' | 'destructive' | 'warning' | 'secondary'

function eventVariant(event: string): BadgeVariant {
  switch (event) {
    case 'boot': case 'stable_tag': return 'success'
    case 'rebirth': case 'app_down': return 'destructive'
    case 'restart': case 'rollback': return 'warning'
    default: return 'secondary'
  }
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-3">
      <p class="text-xs text-muted-foreground">{{ events.length }} events</p>
      <Button variant="ghost" size="sm" @click="load">Refresh</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <span class="text-sm text-muted-foreground">Loading...</span>
    </div>

    <div v-else-if="events.length === 0" class="text-center py-12">
      <p class="text-sm text-muted-foreground">No events yet.</p>
    </div>

    <div v-else>
      <!-- Table header -->
      <div class="grid grid-cols-[auto_auto_1fr_auto] gap-3 px-2 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider border-b border-border">
        <span class="w-28">When</span>
        <span class="w-20">Event</span>
        <span>Note</span>
        <span class="w-16 text-right">Commit</span>
      </div>

      <!-- Rows -->
      <div
        v-for="ev in events"
        :key="ev.id"
        class="grid grid-cols-[auto_auto_1fr_auto] gap-3 px-2 py-2 text-sm border-b border-border/50 hover:bg-secondary/30 transition-colors"
      >
        <span class="w-28 text-xs text-muted-foreground">{{ fmtTime(ev.created_at) }}</span>
        <span class="w-20">
          <Badge :variant="eventVariant(ev.event)" class="text-[10px]">{{ ev.event }}</Badge>
        </span>
        <p class="text-xs text-foreground break-words min-w-0">{{ ev.note }}</p>
        <code v-if="ev.git_hash" class="w-16 text-right text-[10px] text-muted-foreground font-mono">{{ ev.git_hash.slice(0, 7) }}</code>
        <span v-else class="w-16" />
      </div>
    </div>
  </div>
</template>
