<script setup lang="ts">
import { ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Textarea from '@/components/ui/Textarea.vue'

const props = defineProps<{
  initialTitle?: string
  initialContent?: string
  initialPriority?: number
  submitLabel?: string
}>()

const emit = defineEmits<{
  submit: [data: { title: string; content: string; priority: number }]
  cancel: []
}>()

const title = ref(props.initialTitle || '')
const content = ref(props.initialContent || '')
const priority = ref(props.initialPriority ?? 0)
const submitting = ref(false)

async function handleSubmit() {
  if (!title.value.trim() || submitting.value) return
  submitting.value = true
  try {
    emit('submit', {
      title: title.value.trim(),
      content: content.value.trim(),
      priority: priority.value,
    })
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="rounded-lg border border-border bg-card p-3 space-y-3">
    <div>
      <label class="text-xs font-medium text-muted-foreground mb-1 block">Title</label>
      <Input v-model="title" placeholder="Brief title..." />
    </div>

    <div>
      <label class="text-xs font-medium text-muted-foreground mb-1 block">Content</label>
      <Textarea v-model="content" placeholder="Describe the task or brief..." class="min-h-[100px]" />
    </div>

    <div>
      <label class="text-xs font-medium text-muted-foreground mb-1 block">Priority (0 = normal)</label>
      <Input v-model="priority" type="number" class="w-24" />
    </div>

    <div class="flex items-center gap-2 pt-1">
      <Button size="sm" :disabled="!title.trim() || submitting" @click="handleSubmit">
        {{ submitLabel || 'Create' }}
      </Button>
      <Button variant="ghost" size="sm" @click="$emit('cancel')">Cancel</Button>
    </div>
  </div>
</template>
