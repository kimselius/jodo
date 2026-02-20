<script setup lang="ts">
import { ref, computed } from 'vue'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import { api } from '@/lib/api'

interface EnabledModel {
  model_key: string
  model_name: string
  input_cost_per_1m: number
  output_cost_per_1m: number
  capabilities: string[]
  quality: number
  prefer_loaded?: boolean
}

interface DiscoveredModel {
  model_key?: string
  model_name?: string
  input_cost_per_1m?: number
  output_cost_per_1m?: number
  capabilities?: string[]
  quality?: number
  description?: string
  tier?: string
  recommended?: boolean | {
    model_key: string
    model_name: string
    capabilities: string[]
    quality: number
    description: string
    input_cost_per_1m?: number
    output_cost_per_1m?: number
  }
  name?: string
  family?: string
  parameter_size?: string
  quantization?: string
  size_bytes?: number
  vram_estimate?: number
  supports_tools?: boolean | null
  has_thinking?: boolean
}

const ALL_CAPABILITIES = ['code', 'plan', 'chat', 'embed'] as const

const props = withDefaults(defineProps<{
  providerName: string
  enabledModels: EnabledModel[]
  setupMode?: boolean
  baseUrl?: string
  apiKey?: string
}>(), {
  setupMode: false,
  baseUrl: '',
  apiKey: '',
})

const emit = defineEmits<{
  enable: [model: EnabledModel]
  disable: [modelKey: string]
  updateCapabilities: [modelKey: string, capabilities: string[]]
  updatePreferLoaded: [modelKey: string, preferLoaded: boolean]
}>()

const discovering = ref(false)
const discovered = ref<DiscoveredModel[]>([])
const discoveryError = ref('')
const hasDiscovered = ref(false)

const isOllama = computed(() => props.providerName === 'ollama')

const tierOrder = ['flagship', 'reasoning', 'mid', 'budget', 'embed']
const tierLabels: Record<string, string> = {
  flagship: 'Flagship',
  reasoning: 'Reasoning',
  mid: 'Standard',
  budget: 'Budget',
  embed: 'Embedding',
}

const groupedModels = computed(() => {
  if (discovered.value.length === 0) return []

  if (isOllama.value) {
    return [{ tier: '', label: '', models: [...discovered.value] }]
  }

  const groups: Record<string, DiscoveredModel[]> = {}
  for (const m of discovered.value) {
    const tier = m.tier || 'mid'
    if (!groups[tier]) groups[tier] = []
    groups[tier].push(m)
  }

  return tierOrder
    .filter(t => groups[t]?.length)
    .map(t => ({
      tier: t,
      label: tierLabels[t] || t,
      models: groups[t].sort((a, b) => (b.quality ?? 0) - (a.quality ?? 0)),
    }))
})

async function discover() {
  discovering.value = true
  discoveryError.value = ''
  try {
    const res = props.setupMode
      ? await api.setupDiscoverModels(props.providerName, props.baseUrl || undefined, props.apiKey || undefined)
      : await api.discoverModels(props.providerName)
    discovered.value = res.models as DiscoveredModel[]
    if (res.error) discoveryError.value = res.error
    hasDiscovered.value = true
  } catch (e) {
    discoveryError.value = e instanceof Error ? e.message : 'Discovery failed'
  } finally {
    discovering.value = false
  }
}

function getModelKey(model: DiscoveredModel): string {
  return model.model_key || model.name || ''
}

function getModelName(model: DiscoveredModel): string {
  return model.model_name || model.name || model.model_key || ''
}

function isEnabled(model: DiscoveredModel): boolean {
  const key = getModelKey(model)
  return props.enabledModels.some(m => m.model_key === key)
}

function getEnabledModel(modelKey: string): EnabledModel | undefined {
  return props.enabledModels.find(m => m.model_key === modelKey)
}

function getDescription(model: DiscoveredModel): string {
  if (model.description) return model.description
  if (typeof model.recommended === 'object' && model.recommended?.description) {
    return model.recommended.description
  }
  return ''
}

function isRecommended(model: DiscoveredModel): boolean {
  if (model.recommended === true) return true
  if (typeof model.recommended === 'object' && model.recommended) return true
  return false
}

function toggle(model: DiscoveredModel) {
  const key = getModelKey(model)
  if (isEnabled(model)) {
    emit('disable', key)
  } else {
    const rec = typeof model.recommended === 'object' ? model.recommended : null
    const enabledModel: EnabledModel = {
      model_key: key,
      model_name: getModelName(model),
      input_cost_per_1m: model.input_cost_per_1m ?? rec?.input_cost_per_1m ?? 0,
      output_cost_per_1m: model.output_cost_per_1m ?? rec?.output_cost_per_1m ?? 0,
      capabilities: model.capabilities ?? rec?.capabilities ?? ['chat'],
      quality: model.quality ?? rec?.quality ?? 50,
    }
    emit('enable', enabledModel)
  }
}

function toggleCapability(modelKey: string, cap: string) {
  const model = getEnabledModel(modelKey)
  if (!model) return
  const caps = [...model.capabilities]
  const idx = caps.indexOf(cap)
  if (idx >= 0) {
    if (caps.length <= 1) return
    caps.splice(idx, 1)
  } else {
    caps.push(cap)
  }
  emit('updateCapabilities', modelKey, caps)
}

function hasCap(modelKey: string, cap: string): boolean {
  const model = getEnabledModel(modelKey)
  return model?.capabilities.includes(cap) ?? false
}

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + ' GB'
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + ' MB'
  return bytes + ' B'
}

function formatCost(input: number, output: number): string {
  if (input === 0 && output === 0) return 'Free'
  return `$${input}/$${output} /1M`
}

function tierBadgeVariant(tier: string): string {
  switch (tier) {
    case 'flagship': return 'default'
    case 'reasoning': return 'accent'
    case 'mid': return 'secondary'
    case 'budget': return 'success'
    case 'embed': return 'warning'
    default: return 'secondary'
  }
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center gap-3">
      <Button
        size="sm"
        variant="secondary"
        @click="discover"
        :disabled="discovering"
      >
        {{ discovering ? 'Discovering...' : 'Discover Models' }}
      </Button>
      <span v-if="discoveryError" class="text-xs text-destructive">{{ discoveryError }}</span>
      <span v-if="hasDiscovered && discovered.length > 0" class="text-xs text-muted-foreground">
        {{ discovered.length }} models found
      </span>
    </div>

    <div v-if="hasDiscovered && discovered.length === 0 && !discoveryError" class="text-xs text-muted-foreground">
      No models found.
    </div>

    <!-- Tiered model list -->
    <div v-if="groupedModels.length > 0" class="space-y-4">
      <div v-for="group in groupedModels" :key="group.tier">
        <div v-if="group.label" class="flex items-center gap-2 mb-1.5">
          <Badge :variant="tierBadgeVariant(group.tier) as any">{{ group.label }}</Badge>
          <div class="flex-1 border-t border-border" />
        </div>

        <div class="space-y-1">
          <div
            v-for="model in group.models"
            :key="getModelKey(model)"
            class="bg-muted rounded-md px-3 py-2"
          >
            <div
              class="flex items-center gap-3 cursor-pointer hover:opacity-80 transition-opacity"
              @click="toggle(model)"
            >
              <input
                type="checkbox"
                :checked="isEnabled(model)"
                class="rounded border-input flex-shrink-0"
                @click.stop
                @change="toggle(model)"
              />
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-mono truncate">{{ getModelKey(model) }}</span>
                  <Badge v-if="isRecommended(model)" variant="success">recommended</Badge>
                  <span v-if="model.parameter_size" class="text-[10px] text-muted-foreground bg-background px-1.5 py-0.5 rounded">
                    {{ model.parameter_size }}
                  </span>
                  <span v-if="model.supports_tools === true" class="text-[10px] text-green-600 bg-green-500/10 px-1.5 py-0.5 rounded">
                    tools
                  </span>
                  <span v-else-if="model.supports_tools === false" class="text-[10px] text-amber-600 bg-amber-500/10 px-1.5 py-0.5 rounded">
                    no tools
                  </span>
                  <span v-if="model.has_thinking" class="text-[10px] text-blue-600 bg-blue-500/10 px-1.5 py-0.5 rounded">
                    thinking
                  </span>
                </div>
                <p v-if="getDescription(model)" class="text-xs text-muted-foreground truncate mt-0.5">
                  {{ getDescription(model) }}
                </p>
              </div>
              <div class="flex-shrink-0 text-right">
                <div v-if="model.quality" class="text-[10px] text-muted-foreground">
                  q{{ model.quality }}
                </div>
                <div v-if="model.vram_estimate" class="text-[10px] text-muted-foreground">
                  ~{{ formatSize(model.vram_estimate) }} VRAM
                </div>
                <div v-else-if="model.size_bytes" class="text-[10px] text-muted-foreground">
                  {{ formatSize(model.size_bytes) }}
                </div>
                <div class="text-[10px] text-muted-foreground">
                  {{ formatCost(model.input_cost_per_1m ?? 0, model.output_cost_per_1m ?? 0) }}
                </div>
              </div>
            </div>

            <!-- Capability chips (editable when model is enabled) -->
            <div v-if="isEnabled(model)" class="flex flex-wrap items-center gap-1.5 mt-2 ml-8" @click.stop>
              <button
                v-for="cap in ALL_CAPABILITIES"
                :key="cap"
                class="text-[10px] px-2 py-0.5 rounded-full border transition-colors"
                :class="hasCap(getModelKey(model), cap)
                  ? 'bg-primary/15 text-primary border-primary/30'
                  : 'bg-transparent text-muted-foreground border-border hover:border-muted-foreground'"
                @click="toggleCapability(getModelKey(model), cap)"
              >
                {{ cap }}
              </button>
              <label v-if="isOllama" class="flex items-center gap-1 ml-2 cursor-pointer" :title="'When checked, this model is preferred if already loaded in VRAM'">
                <input
                  type="checkbox"
                  :checked="getEnabledModel(getModelKey(model))?.prefer_loaded ?? false"
                  class="rounded border-input"
                  @change="emit('updatePreferLoaded', getModelKey(model), ($event.target as HTMLInputElement).checked)"
                />
                <span class="text-[10px] text-muted-foreground">use if loaded</span>
              </label>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Currently enabled models (when discovery not open) -->
    <div v-if="enabledModels.length > 0 && !hasDiscovered" class="space-y-2">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Selected Models</h4>
      <div
        v-for="m in enabledModels"
        :key="m.model_key"
        class="bg-muted rounded-md px-3 py-2"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-mono text-foreground">{{ m.model_key }}</span>
          <span class="text-[10px] text-muted-foreground">q{{ m.quality }}</span>
        </div>
        <div class="flex flex-wrap items-center gap-1.5 mt-1.5">
          <button
            v-for="cap in ALL_CAPABILITIES"
            :key="cap"
            class="text-[10px] px-2 py-0.5 rounded-full border transition-colors"
            :class="m.capabilities.includes(cap)
              ? 'bg-primary/15 text-primary border-primary/30'
              : 'bg-transparent text-muted-foreground border-border hover:border-muted-foreground'"
            @click="toggleCapability(m.model_key, cap)"
          >
            {{ cap }}
          </button>
          <label v-if="isOllama" class="flex items-center gap-1 ml-2 cursor-pointer" :title="'When checked, this model is preferred if already loaded in VRAM'">
            <input
              type="checkbox"
              :checked="m.prefer_loaded ?? false"
              class="rounded border-input"
              @change="emit('updatePreferLoaded', m.model_key, ($event.target as HTMLInputElement).checked)"
            />
            <span class="text-[10px] text-muted-foreground">use if loaded</span>
          </label>
        </div>
      </div>
    </div>

    <div v-if="enabledModels.length === 0 && !hasDiscovered" class="text-xs text-muted-foreground italic">
      No models selected. Click "Discover Models" to find available models.
    </div>
  </div>
</template>
