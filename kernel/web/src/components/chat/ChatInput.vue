<script setup lang="ts">
import { ref, nextTick, watch, onMounted } from 'vue'
import Button from '@/components/ui/Button.vue'

const props = defineProps<{ sending: boolean }>()
const emit = defineEmits<{ send: [text: string] }>()

const text = ref('')
const textarea = ref<HTMLTextAreaElement>()

onMounted(() => textarea.value?.focus())

watch(() => props.sending, (now, prev) => {
  if (prev && !now) nextTick(() => textarea.value?.focus())
})

function handleSend() {
  if (!text.value.trim() || props.sending) return
  emit('send', text.value)
  text.value = ''
  nextTick(() => textarea.value?.focus())
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSend()
  }
}
</script>

<template>
  <div class="border-t border-border bg-card px-4 py-3">
    <div class="flex items-end gap-2">
      <textarea
        ref="textarea"
        v-model="text"
        @keydown="handleKeydown"
        placeholder="Message Jodo..."
        rows="1"
        :disabled="sending"
        class="flex-1 resize-none rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring min-h-[40px] max-h-[120px]"
        style="field-sizing: content"
      />
      <Button
        size="icon"
        :disabled="!text.trim() || sending"
        @click="handleSend"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 19V5m0 0l-7 7m7-7l7 7" />
        </svg>
      </Button>
    </div>
  </div>
</template>
