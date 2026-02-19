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
}

interface DiscoveredModel {
  // Common
  model_key?: string
  model_name?: string
  input_cost_per_1m?: number
  output_cost_per_1m?: number
  capabilities?: string[]
  quality?: number
  description?: string
  recommended?: boolean | {
    model_key: string
    model_name: string
    capabilities: string[]
    quality: number
    description: string
    input_cost_per_1m?: number
    output_cost_per_1m?: number
  }
  // Ollama-specific
  name?: string
  family?: string
  parameter_size?: string
  quantization?: string
  size_bytes?: number
}

const props = defineProps<{
  providerName: string
  enabledModels: EnabledModel[]
}>()

const emit = defineEmits<{
  enable: [model: EnabledModel]
  disable: [modelKey: string]
}>()

const discovering = ref(false)
const discovered = ref<DiscoveredModel[]>([])
const discoveryError = ref('')
const hasDiscovered = ref(false)

const isOllama = computed(() => props.providerName === 'ollama')
const buttonLabel = computed(() => isOllama.value ? 'Discover Models' : 'Show Available Models')

async function discover() {
  discovering.value = true
  discoveryError.value = ''
  try {
    const res = await api.discoverModels(props.providerName)
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
    // Build model data from discovered info
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

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + ' GB'
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + ' MB'
  return bytes + ' B'
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
        {{ discovering ? 'Discovering...' : buttonLabel }}
      </Button>
      <span v-if="discoveryError" class="text-xs text-destructive">{{ discoveryError }}</span>
    </div>

    <div v-if="hasDiscovered && discovered.length === 0 && !discoveryError" class="text-xs text-muted-foreground">
      No models found.
    </div>

    <div v-if="discovered.length > 0" class="space-y-1.5">
      <div
        v-for="model in discovered"
        :key="getModelKey(model)"
        class="flex items-center gap-3 bg-muted rounded-md px-3 py-2 cursor-pointer hover:bg-muted/80 transition-colors"
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
          </div>
          <p v-if="getDescription(model)" class="text-xs text-muted-foreground mt-0.5 truncate">
            {{ getDescription(model) }}
          </p>
        </div>
        <div class="flex-shrink-0 text-right">
          <div v-if="model.size_bytes" class="text-[10px] text-muted-foreground">
            {{ formatSize(model.size_bytes) }}
          </div>
          <div v-if="(model.input_cost_per_1m ?? 0) > 0" class="text-[10px] text-muted-foreground">
            ${{ model.input_cost_per_1m }}/${{ model.output_cost_per_1m }} /1M
          </div>
        </div>
      </div>
    </div>

    <!-- Currently enabled models (always visible) -->
    <div v-if="enabledModels.length > 0 && !hasDiscovered" class="space-y-1">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Enabled Models</h4>
      <div
        v-for="m in enabledModels"
        :key="m.model_key"
        class="text-xs bg-muted rounded px-2 py-1.5 flex items-center justify-between"
      >
        <span>
          <span class="font-mono text-foreground">{{ m.model_key }}</span>
          <span class="text-muted-foreground ml-1">({{ m.capabilities.join(', ') }})</span>
        </span>
        <span class="text-muted-foreground">q{{ m.quality }}</span>
      </div>
    </div>
  </div>
</template>
