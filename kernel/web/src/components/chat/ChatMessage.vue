<script setup lang="ts">
import { computed } from 'vue'
import type { ChatMessage } from '@/types/chat'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'

const props = defineProps<{ msg: ChatMessage }>()

const isHuman = computed(() => props.msg.source === 'human')
const isJodo = computed(() => props.msg.source === 'jodo')
const isRead = computed(() => !!props.msg.read_at)
</script>

<template>
  <div :class="['flex gap-3 px-4', isHuman ? 'flex-row-reverse' : 'flex-row']">
    <!-- Avatar -->
    <div :class="[
      'flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold mt-1',
      isHuman ? 'bg-primary/20 text-primary' : 'bg-accent/20 text-accent'
    ]">
      {{ isHuman ? 'H' : 'J' }}
    </div>

    <!-- Bubble -->
    <div :class="['max-w-[75%] min-w-0', isHuman ? 'items-end' : 'items-start']">
      <div :class="[
        'rounded-2xl px-3.5 py-2 text-sm leading-relaxed',
        isHuman
          ? 'bg-primary text-primary-foreground rounded-tr-sm'
          : 'bg-secondary text-foreground rounded-tl-sm'
      ]">
        <p class="whitespace-pre-wrap break-words">{{ msg.message }}</p>
      </div>

      <!-- Meta row -->
      <div :class="['flex items-center gap-2 mt-1 px-1', isHuman ? 'justify-end' : 'justify-start']">
        <span class="text-[10px] text-muted-foreground">{{ formatTime(msg.created_at) }}</span>
        <Badge v-if="isJodo && msg.galla != null" variant="accent" class="text-[10px] px-1.5 py-0">
          g{{ msg.galla }}
        </Badge>
        <svg
          v-if="isJodo && isRead"
          class="h-3 w-3 text-primary"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      </div>
    </div>
  </div>
</template>
