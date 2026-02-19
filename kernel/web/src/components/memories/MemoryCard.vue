<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'
import type { MemoryEntry } from '@/types/memory'

defineProps<{ memory: MemoryEntry }>()
</script>

<template>
  <Card class="p-3">
    <p class="text-sm whitespace-pre-wrap break-words">{{ memory.content }}</p>

    <div class="flex items-center gap-2 mt-2 flex-wrap">
      <Badge
        v-for="tag in memory.tags"
        :key="tag"
        variant="secondary"
        class="text-[10px]"
      >
        {{ tag }}
      </Badge>
      <span v-if="memory.source" class="text-[10px] text-muted-foreground">
        via {{ memory.source }}
      </span>
      <span class="text-[10px] text-muted-foreground ml-auto shrink-0">
        {{ formatTime(memory.created_at) }}
      </span>
    </div>
  </Card>
</template>
