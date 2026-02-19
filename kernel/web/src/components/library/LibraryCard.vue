<script setup lang="ts">
import { computed } from 'vue'
import type { LibraryItem } from '@/types/library'
import Badge from '@/components/ui/Badge.vue'

const props = defineProps<{
  item: LibraryItem
  expanded: boolean
}>()

defineEmits<{
  toggle: []
  'update-status': [status: string]
}>()

const statusVariant = computed(() => {
  switch (props.item.status) {
    case 'new': return 'default'
    case 'in_progress': return 'accent'
    case 'done': return 'success'
    case 'blocked': return 'destructive'
    case 'archived': return 'secondary'
    default: return 'secondary'
  }
})

const statusLabel = computed(() => {
  switch (props.item.status) {
    case 'new': return 'New'
    case 'in_progress': return 'In Progress'
    case 'done': return 'Done'
    case 'blocked': return 'Blocked'
    case 'archived': return 'Archived'
    default: return props.item.status
  }
})

const timeAgo = computed(() => {
  const diff = Date.now() - new Date(props.item.updated_at).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
})
</script>

<template>
  <div
    class="rounded-lg border border-border bg-card text-card-foreground transition-colors hover:border-border/80"
    :class="{ 'ring-1 ring-primary/20': expanded }"
  >
    <!-- Header row — always visible -->
    <button
      class="flex w-full items-center gap-3 p-3 text-left"
      @click="$emit('toggle')"
    >
      <!-- Priority indicator -->
      <div
        v-if="item.priority > 0"
        class="flex h-5 w-5 shrink-0 items-center justify-center rounded text-[10px] font-bold"
        :class="item.priority >= 2 ? 'bg-warning/15 text-warning' : 'bg-muted text-muted-foreground'"
      >
        {{ item.priority }}
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium truncate">{{ item.title }}</span>
          <Badge :variant="statusVariant" class="shrink-0">{{ statusLabel }}</Badge>
        </div>
        <p v-if="!expanded && item.content" class="text-xs text-muted-foreground truncate mt-0.5">
          {{ item.content.slice(0, 120) }}
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <!-- Comment count -->
        <span v-if="item.comments.length > 0" class="flex items-center gap-1 text-xs text-muted-foreground">
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          {{ item.comments.length }}
        </span>
        <span class="text-[10px] text-muted-foreground">{{ timeAgo }}</span>
        <!-- Expand chevron -->
        <svg
          class="h-4 w-4 text-muted-foreground transition-transform"
          :class="{ 'rotate-180': expanded }"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </div>
    </button>

    <!-- Expanded content -->
    <div v-if="expanded" class="border-t border-border px-3 pb-3">
      <!-- Full content -->
      <div v-if="item.content" class="pt-3">
        <p class="text-sm text-foreground whitespace-pre-wrap">{{ item.content }}</p>
      </div>

      <!-- Status actions -->
      <div class="flex items-center gap-1.5 mt-3">
        <span class="text-[10px] text-muted-foreground mr-1">Status:</span>
        <button
          v-for="s in ['new', 'in_progress', 'done', 'blocked', 'archived']"
          :key="s"
          @click="$emit('update-status', s)"
          class="rounded px-2 py-0.5 text-[10px] font-medium transition-colors"
          :class="item.status === s
            ? 'bg-primary text-primary-foreground'
            : 'bg-secondary text-muted-foreground hover:text-foreground'"
        >
          {{ s === 'in_progress' ? 'In Progress' : s.charAt(0).toUpperCase() + s.slice(1) }}
        </button>
      </div>

      <!-- Comment thread -->
      <slot name="comments" />
    </div>
  </div>
</template>
