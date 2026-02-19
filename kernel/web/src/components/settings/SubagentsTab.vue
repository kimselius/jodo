<script setup lang="ts">
import { ref, watch } from 'vue'
import { api } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'

const props = defineProps<{
  subagent: { max_concurrent: number; max_timeout: number }
}>()

const emit = defineEmits<{ saved: [] }>()

const maxConcurrent = ref(props.subagent.max_concurrent)
const maxTimeout = ref(props.subagent.max_timeout)
const saving = ref(false)
const error = ref<string | null>(null)

watch(() => props.subagent, (v) => {
  maxConcurrent.value = v.max_concurrent
  maxTimeout.value = v.max_timeout
})

async function save() {
  saving.value = true
  error.value = null
  try {
    await api.updateSettingsSubagent({
      max_concurrent: Number(maxConcurrent.value),
      max_timeout: Number(maxTimeout.value),
    })
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <p class="text-sm text-muted-foreground">
      Configure how many subagents Jodo can run in parallel and their timeout limits.
      Changes apply on next seed restart or rebirth.
    </p>

    <div class="space-y-3">
      <div>
        <label class="text-sm font-medium mb-1 block">Max concurrent subagents</label>
        <Input v-model="maxConcurrent" type="number" class="w-32" />
        <p class="text-xs text-muted-foreground mt-1">Range: 1–10. More agents = more parallel work but higher resource usage.</p>
      </div>

      <div>
        <label class="text-sm font-medium mb-1 block">Max timeout (seconds)</label>
        <Input v-model="maxTimeout" type="number" class="w-32" />
        <p class="text-xs text-muted-foreground mt-1">Range: 60–3600. How long a subagent can run before being terminated.</p>
      </div>
    </div>

    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <Button size="sm" :disabled="saving" @click="save">
      {{ saving ? 'Saving...' : 'Save' }}
    </Button>
  </div>
</template>
