<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import draggable from 'vuedraggable'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import type { ProviderSetup } from '@/types/setup'

const props = defineProps<{
  providers: ProviderSetup[]
  routing: Record<string, string[]>
}>()

defineEmits<{
  next: []
  back: []
}>()

const intents = ['code', 'chat', 'embed', 'reasoning', 'tools', 'quick', 'repair']

const intentLabels: Record<string, string> = {
  code: 'Code',
  chat: 'Chat',
  embed: 'Embedding',
  reasoning: 'Reasoning',
  tools: 'Tool Use',
  quick: 'Quick Tasks',
  repair: 'Repair',
}

const intentDescriptions: Record<string, string> = {
  code: 'Writing code, building features, debugging',
  chat: 'Conversations with the human, general responses',
  embed: 'Text embeddings for search and memory',
  reasoning: 'Complex reasoning, analysis, planning',
  tools: 'Tool-calling tasks (read/write/execute)',
  quick: 'Fast, simple tasks where speed matters',
  repair: 'Self-repair when something breaks',
}

// All enabled models across all providers
const allModels = computed(() => {
  const models: { ref: string; modelKey: string; provider: string; quality: number; capabilities: string[] }[] = []
  for (const p of props.providers) {
    if (!p.enabled) continue
    for (const m of p.models) {
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

// Which intents have at least one model
const activeIntents = computed(() =>
  intents.filter(intent =>
    allModels.value.some(m => m.capabilities.includes(intent))
  )
)

function getIntentList(intent: string) {
  return (props.routing[intent] || []).map(ref => ({
    id: ref,
    ref,
    ...parseRef(ref),
  }))
}

function setIntentList(intent: string, list: { id: string; ref: string }[]) {
  props.routing[intent] = list.map(item => item.ref)
}

function parseRef(ref: string) {
  const parts = ref.split('@')
  if (parts.length === 2) {
    return { modelKey: parts[0], provider: parts[1] }
  }
  return { modelKey: ref, provider: '' }
}

function addableModels(intent: string) {
  const current = new Set(props.routing[intent] || [])
  return allModels.value.filter(m => {
    if (current.has(m.ref)) return false
    return m.capabilities.includes(intent)
  })
}

function addModel(intent: string, modelRef: string) {
  if (!props.routing[intent]) props.routing[intent] = []
  if (!props.routing[intent].includes(modelRef)) {
    props.routing[intent].push(modelRef)
  }
}

function removeModel(intent: string, modelRef: string) {
  if (!props.routing[intent]) return
  props.routing[intent] = props.routing[intent].filter(r => r !== modelRef)
}

function providerColor(name: string): 'default' | 'success' | 'warning' | 'accent' {
  switch (name) {
    case 'ollama': return 'success'
    case 'claude': return 'accent'
    case 'openai': return 'warning'
    default: return 'default'
  }
}

function getModelQuality(ref: string): number {
  const m = allModels.value.find(m => m.ref === ref)
  return m?.quality ?? 0
}

// Auto-populate routing from provider models on first load
watch(allModels, (models) => {
  if (models.length === 0) return
  for (const intent of intents) {
    // Only auto-populate if empty
    if (props.routing[intent]?.length) continue
    const matching = models
      .filter(m => m.capabilities.includes(intent))
      .sort((a, b) => b.quality - a.quality)
    if (matching.length > 0) {
      props.routing[intent] = matching.map(m => m.ref)
    }
  }
}, { immediate: true })
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">Model Routing</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Set the priority order for each task type. Jodo tries models top-to-bottom — drag to reorder, or remove models you don't want for a given task.
      </p>
    </div>

    <Card v-for="intent in activeIntents" :key="intent" class="p-4 space-y-3">
      <div>
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold">{{ intentLabels[intent] || intent }}</h3>
          <span class="text-xs text-muted-foreground">
            {{ (routing[intent] || []).length }} model{{ (routing[intent] || []).length !== 1 ? 's' : '' }}
          </span>
        </div>
        <p class="text-xs text-muted-foreground mt-0.5">{{ intentDescriptions[intent] }}</p>
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
            <span class="text-sm font-mono flex-1 truncate">{{ element.modelKey }}</span>
            <Badge :variant="providerColor(element.provider)" class="text-[10px]">
              {{ element.provider }}
            </Badge>
            <span class="text-[10px] text-muted-foreground">q{{ getModelQuality(element.ref) }}</span>
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

      <p v-else-if="(routing[intent] || []).length === 0" class="text-xs text-muted-foreground italic">
        No models with this capability. Go back and set capabilities on your models.
      </p>
    </Card>

    <div v-if="activeIntents.length === 0" class="text-sm text-muted-foreground text-center py-8">
      No models configured yet. Go back to the Providers step and enable some models.
    </div>

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')">Next</Button>
    </div>
  </div>
</template>
