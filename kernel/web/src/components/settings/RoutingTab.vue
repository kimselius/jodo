<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import draggable from 'vuedraggable'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import { api } from '@/lib/api'
import type { RoutingConfig, ProviderInfo } from '@/types/setup'

const props = defineProps<{
  routing: RoutingConfig
  providers: ProviderInfo[]
}>()

const emit = defineEmits<{ saved: [] }>()

const form = ref<RoutingConfig>({
  intent_preferences: {},
})
const saving = ref(false)
const error = ref<string | null>(null)

watch(() => props.routing, (r) => {
  if (r) {
    form.value = JSON.parse(JSON.stringify(r))
  }
}, { immediate: true })

const intents = ['code', 'plan', 'chat', 'embed']

// Build master list of all enabled model@provider refs
const availableModels = computed(() => {
  const models: { ref: string; modelKey: string; provider: string; quality: number; capabilities: string[] }[] = []
  for (const p of props.providers) {
    if (!p.enabled) continue
    for (const m of p.models) {
      if (!m.enabled) continue
      models.push({
        ref: `${m.model_key}@${p.name}`,
        modelKey: m.model_key,
        provider: p.name,
        quality: m.quality,
        capabilities: m.capabilities,
      })
    }
  }
  return models
})

// Get draggable list for an intent
function getIntentList(intent: string) {
  return (form.value.intent_preferences?.[intent] || []).map(ref => ({
    id: ref,
    ref,
    ...parseRef(ref),
  }))
}

function setIntentList(intent: string, list: { id: string; ref: string }[]) {
  if (!form.value.intent_preferences) form.value.intent_preferences = {}
  form.value.intent_preferences[intent] = list.map(item => item.ref)
}

function parseRef(ref: string) {
  const parts = ref.split('@')
  if (parts.length === 2) {
    return { modelKey: parts[0], provider: parts[1], isModelRef: true }
  }
  return { modelKey: '', provider: ref, isModelRef: false }
}

// Models available to add for a specific intent (not already in the list)
function addableModels(intent: string) {
  const current = new Set(form.value.intent_preferences?.[intent] || [])
  return availableModels.value.filter(m => {
    if (current.has(m.ref)) return false
    // Filter by capability match
    return m.capabilities.includes(intent)
  })
}

function addModel(intent: string, modelRef: string) {
  if (!form.value.intent_preferences) form.value.intent_preferences = {}
  if (!form.value.intent_preferences[intent]) form.value.intent_preferences[intent] = []
  if (!form.value.intent_preferences[intent].includes(modelRef)) {
    form.value.intent_preferences[intent].push(modelRef)
  }
}

function removeModel(intent: string, modelRef: string) {
  if (!form.value.intent_preferences?.[intent]) return
  form.value.intent_preferences[intent] = form.value.intent_preferences[intent].filter(r => r !== modelRef)
}

function providerColor(name: string): 'default' | 'success' | 'warning' | 'accent' {
  switch (name) {
    case 'ollama': return 'success'
    case 'claude': return 'accent'
    case 'openai': return 'warning'
    default: return 'default'
  }
}

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

    <Card v-for="intent in intents" :key="intent" class="p-4 space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold capitalize">{{ intent }}</h3>
        <span class="text-xs text-muted-foreground">
          {{ (form.intent_preferences?.[intent] || []).length }} model{{ (form.intent_preferences?.[intent] || []).length !== 1 ? 's' : '' }}
        </span>
      </div>

      <draggable
        :model-value="getIntentList(intent)"
        @update:model-value="(list: { id: string; ref: string }[]) => setIntentList(intent, list)"
        item-key="id"
        handle=".drag-handle"
        :animation="200"
        class="space-y-1"
      >
        <template #item="{ element, index }">
          <div class="flex items-center gap-2 bg-muted rounded-md px-2 py-1.5 group">
            <span class="drag-handle cursor-grab text-muted-foreground hover:text-foreground text-xs select-none">
              &#x2630;
            </span>
            <span class="text-xs text-muted-foreground w-4 text-right">{{ index + 1 }}.</span>
            <span class="text-sm font-mono flex-1 truncate">{{ element.modelKey || element.ref }}</span>
            <Badge v-if="element.isModelRef" :variant="providerColor(element.provider)" class="text-[10px]">
              {{ element.provider }}
            </Badge>
            <button
              @click="removeModel(intent, element.ref)"
              class="text-muted-foreground hover:text-destructive text-xs opacity-0 group-hover:opacity-100 transition-opacity"
              title="Remove"
            >
              &#x2715;
            </button>
          </div>
        </template>
      </draggable>

      <div v-if="addableModels(intent).length > 0">
        <select
          class="w-full h-8 rounded-md border border-input bg-background px-2 text-xs text-muted-foreground"
          @change="(e) => { const v = (e.target as HTMLSelectElement).value; if (v) { addModel(intent, v); (e.target as HTMLSelectElement).value = '' } }"
        >
          <option value="">+ Add model...</option>
          <option v-for="m in addableModels(intent)" :key="m.ref" :value="m.ref">
            {{ m.modelKey }} @ {{ m.provider }} (q{{ m.quality }})
          </option>
        </select>
      </div>

      <p v-else-if="(form.intent_preferences?.[intent] || []).length === 0" class="text-xs text-muted-foreground">
        No models configured. Enable models in the Providers tab first.
      </p>
    </Card>

    <div class="flex justify-end">
      <Button @click="save" :disabled="saving">
        {{ saving ? 'Saving...' : 'Save Routing' }}
      </Button>
    </div>
  </div>
</template>
