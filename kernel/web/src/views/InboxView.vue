<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useInbox } from '@/composables/useInbox'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'

const { messages, loading, error, load } = useInbox()
const bottom = ref<HTMLElement>()

// Auto-scroll when new messages arrive
watch(
  () => messages.value.length,
  async () => {
    await nextTick()
    bottom.value?.scrollIntoView({ behavior: 'smooth' })
  }
)

function sourceVariant(source: string) {
  if (source === 'kernel') return 'default'
  if (source === 'jodo') return 'accent'
  if (source.startsWith('subagent:')) return 'warning'
  return 'secondary'
}

function formatTime(ts: string) {
  const d = new Date(ts)
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h1 class="text-lg font-semibold">Inbox</h1>
          <p class="text-xs text-muted-foreground mt-0.5">Read-only intercom — kernel, Jodo, and subagent communications</p>
        </div>
        <Button variant="ghost" size="sm" @click="load">Refresh</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

      <div v-if="loading && messages.length === 0" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else-if="messages.length === 0" class="text-center py-12">
        <p class="text-sm text-muted-foreground">No intercom messages yet.</p>
      </div>

      <div v-else class="space-y-1.5">
        <div
          v-for="msg in messages"
          :key="msg.id"
          class="flex gap-2 rounded-md p-2 hover:bg-secondary/30 transition-colors"
        >
          <div class="flex flex-col items-center gap-1 shrink-0 pt-0.5">
            <Badge :variant="sourceVariant(msg.source)" class="text-[10px]">
              {{ msg.source }}
            </Badge>
            <svg v-if="msg.target" class="h-3 w-3 text-muted-foreground/50" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" />
            </svg>
            <Badge v-if="msg.target" variant="secondary" class="text-[10px]">
              {{ msg.target }}
            </Badge>
          </div>

          <div class="min-w-0 flex-1">
            <p class="text-sm text-foreground whitespace-pre-wrap break-words">{{ msg.message }}</p>
            <div class="flex items-center gap-2 mt-0.5">
              <span class="text-[10px] text-muted-foreground">{{ formatTime(msg.created_at) }}</span>
              <span v-if="msg.galla != null" class="text-[10px] text-muted-foreground">g{{ msg.galla }}</span>
            </div>
          </div>
        </div>
        <div ref="bottom" />
      </div>
    </div>
  </div>
</template>
