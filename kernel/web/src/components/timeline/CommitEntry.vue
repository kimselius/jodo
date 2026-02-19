<script setup lang="ts">
import type { CommitEntry } from '@/types/history'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'

defineProps<{ commit: CommitEntry }>()
</script>

<template>
  <div class="flex gap-3">
    <!-- Timeline dot -->
    <div class="flex flex-col items-center">
      <div :class="[
        'h-2.5 w-2.5 rounded-full mt-1.5 shrink-0',
        commit.tag ? 'bg-success' : 'bg-border'
      ]" />
      <div class="w-px flex-1 bg-border" />
    </div>

    <!-- Content -->
    <div class="pb-4 min-w-0">
      <div class="flex items-center gap-2 flex-wrap">
        <code class="text-xs text-muted-foreground font-mono">{{ commit.hash.slice(0, 7) }}</code>
        <Badge v-if="commit.tag" variant="success" class="text-[10px]">{{ commit.tag }}</Badge>
        <span class="text-[10px] text-muted-foreground">{{ formatTime(commit.timestamp) }}</span>
      </div>
      <p class="text-sm mt-0.5 break-words">{{ commit.message }}</p>
    </div>
  </div>
</template>
