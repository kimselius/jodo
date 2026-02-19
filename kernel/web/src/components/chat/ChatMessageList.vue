<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import type { ChatMessage } from '@/types/chat'
import ChatMessageComp from './ChatMessage.vue'

const props = defineProps<{
  messages: ChatMessage[]
  loading: boolean
}>()

const container = ref<HTMLElement>()
const isAtBottom = ref(true)

// Show patience message when the last message is from the human
const waitingForJodo = computed(() => {
  if (props.messages.length === 0) return false
  const last = props.messages[props.messages.length - 1]
  return last.source === 'human'
})

function scrollToBottom() {
  if (container.value) {
    container.value.scrollTop = container.value.scrollHeight
  }
}

function onScroll() {
  if (!container.value) return
  const { scrollTop, scrollHeight, clientHeight } = container.value
  isAtBottom.value = scrollHeight - scrollTop - clientHeight < 50
}

watch(() => props.messages.length, () => {
  if (isAtBottom.value) {
    nextTick(scrollToBottom)
  }
})

onMounted(() => {
  nextTick(scrollToBottom)
})
</script>

<template>
  <div
    ref="container"
    class="flex-1 overflow-y-auto"
    @scroll="onScroll"
  >
    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <div class="flex items-center gap-2 text-muted-foreground text-sm">
        <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        Loading messages...
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="messages.length === 0"
      class="flex flex-col items-center justify-center h-full text-center px-4"
    >
      <div class="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 mb-4">
        <span class="text-3xl text-primary font-bold">J</span>
      </div>
      <h3 class="text-lg font-medium mb-1">Welcome</h3>
      <p class="text-sm text-muted-foreground max-w-sm">
        This is the beginning of your conversation with Jodo.
        Send a message to get started.
      </p>
    </div>

    <!-- Messages -->
    <div v-else class="flex flex-col gap-3 py-4">
      <ChatMessageComp
        v-for="msg in messages"
        :key="msg.id"
        :msg="msg"
      />

      <!-- Patience message when waiting for Jodo -->
      <div v-if="waitingForJodo" class="px-4">
        <div class="flex items-start gap-2 max-w-[85%]">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/15 mt-0.5">
            <span class="text-primary text-xs font-bold">J</span>
          </div>
          <p class="text-xs text-muted-foreground italic leading-relaxed pt-1">
            Jodo will respond in the next galla. Please be patient.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
