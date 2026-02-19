<script setup lang="ts">
import { ref, watch } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import { api } from '@/lib/api'
import type { RoutingConfig } from '@/types/setup'

const props = defineProps<{
  routing: RoutingConfig
}>()

const emit = defineEmits<{ saved: [] }>()

const form = ref<RoutingConfig>({
  strategy: 'best_affordable',
  intent_preferences: {},
})
const saving = ref(false)
const error = ref<string | null>(null)

watch(() => props.routing, (r) => {
  if (r) {
    form.value = JSON.parse(JSON.stringify(r))
  }
}, { immediate: true })

const intents = ['code', 'chat', 'embed', 'quick', 'repair']

async function save() {
  saving.value = true
  error.value = null
  try {
    await api.updateSettingsRouting(form.value)
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
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">Routing Strategy</label>
        <Input v-model="form.strategy" placeholder="best_affordable" />
        <p class="text-xs text-muted-foreground mt-1">How the kernel picks which provider to use for each request.</p>
      </div>
    </Card>

    <Card class="p-4 space-y-4">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Intent Preferences</h3>
      <p class="text-xs text-muted-foreground">
        Provider preference order for each intent type (comma-separated).
      </p>

      <div v-for="intent in intents" :key="intent" class="flex items-center gap-3">
        <label class="text-sm font-medium w-16 text-right">{{ intent }}</label>
        <Input
          :model-value="(form.intent_preferences?.[intent] || []).join(', ')"
          @update:model-value="(v: string) => {
            if (!form.intent_preferences) form.intent_preferences = {}
            form.intent_preferences[intent] = v.split(',').map(s => s.trim()).filter(Boolean)
          }"
          placeholder="ollama, claude, openai"
          class="flex-1"
        />
      </div>
    </Card>

    <div class="flex justify-end">
      <Button @click="save" :disabled="saving">
        {{ saving ? 'Saving...' : 'Save Routing' }}
      </Button>
    </div>
  </div>
</template>
