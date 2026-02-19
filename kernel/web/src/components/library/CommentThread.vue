<script setup lang="ts">
import { ref } from 'vue'
import type { LibraryComment } from '@/types/library'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'

defineProps<{
  comments: LibraryComment[]
}>()

const emit = defineEmits<{
  comment: [message: string]
}>()

const newComment = ref('')
const sending = ref(false)

async function submit() {
  const msg = newComment.value.trim()
  if (!msg || sending.value) return
  sending.value = true
  try {
    emit('comment', msg)
    newComment.value = ''
  } finally {
    sending.value = false
  }
}

function formatTime(ts: string) {
  const d = new Date(ts)
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="mt-3 border-t border-border pt-3">
    <p class="text-[10px] font-medium text-muted-foreground mb-2">Comments</p>

    <!-- Comment list -->
    <div v-if="comments.length > 0" class="space-y-2 mb-3">
      <div
        v-for="c in comments"
        :key="c.id"
        class="flex gap-2"
      >
        <Badge
          :variant="c.source === 'jodo' ? 'accent' : 'default'"
          class="shrink-0 mt-0.5"
        >
          {{ c.source }}
        </Badge>
        <div class="min-w-0 flex-1">
          <p class="text-sm text-foreground whitespace-pre-wrap">{{ c.message }}</p>
          <p class="text-[10px] text-muted-foreground mt-0.5">{{ formatTime(c.created_at) }}</p>
        </div>
      </div>
    </div>

    <!-- New comment input -->
    <div class="flex gap-2">
      <input
        v-model="newComment"
        placeholder="Add a comment..."
        class="flex h-8 flex-1 rounded-md border border-input bg-transparent px-3 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        @keydown.enter.prevent="submit"
      />
      <Button size="sm" :disabled="!newComment.trim() || sending" @click="submit">
        Send
      </Button>
    </div>
  </div>
</template>
