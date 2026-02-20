<script setup lang="ts">
import { computed } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import ModelDiscovery from '@/components/settings/ModelDiscovery.vue'

export interface EnabledModel {
  model_key: string
  model_name: string
  input_cost_per_1m: number
  output_cost_per_1m: number
  capabilities: string[]
  quality: number
  prefer_loaded?: boolean
}

const enabled = defineModel<boolean>('enabled', { required: true })
const baseUrl = defineModel<string>('baseUrl', { required: true })
const apiKeyValue = defineModel<string>('apiKeyValue', { required: true })
const monthlyBudget = defineModel<number>('monthlyBudget', { required: true })
const emergencyReserve = defineModel<number>('emergencyReserve', { required: true })
const totalVramBytes = defineModel<number>('totalVramBytes', { required: true })

const props = withDefaults(defineProps<{
  name: string
  hasExistingKey?: boolean
  enabledModels: EnabledModel[]
  setupMode?: boolean
  saving?: boolean
  showSave?: boolean
}>(), {
  hasExistingKey: false,
  setupMode: false,
  saving: false,
  showSave: false,
})

const emit = defineEmits<{
  save: []
  modelEnable: [model: EnabledModel]
  modelDisable: [key: string]
  updateCapabilities: [key: string, caps: string[]]
  updatePreferLoaded: [key: string, val: boolean]
}>()

const isOllama = computed(() => props.name === 'ollama')

const vramGB = computed({
  get: () => totalVramBytes.value ? String(Math.round(totalVramBytes.value / 1073741824)) : '',
  set: (v: string) => { totalVramBytes.value = Number(v || 0) * 1073741824 },
})

const budgetStr = computed({
  get: () => monthlyBudget.value ? String(monthlyBudget.value) : '',
  set: (v: string) => { monthlyBudget.value = Number(v || 0) },
})

const reserveStr = computed({
  get: () => emergencyReserve.value ? String(emergencyReserve.value) : '',
  set: (v: string) => { emergencyReserve.value = Number(v || 0) },
})
</script>

<template>
  <Card class="p-4 space-y-4">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <h3 class="text-sm font-semibold capitalize">{{ name }}</h3>
        <span v-if="isOllama && setupMode" class="text-[10px] uppercase tracking-wider text-muted-foreground bg-muted px-1.5 py-0.5 rounded">Free</span>
      </div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input type="checkbox" v-model="enabled" class="rounded border-input" />
        <span class="text-xs text-muted-foreground">Enabled</span>
      </label>
    </div>

    <template v-if="enabled">
      <!-- Ollama: base URL + VRAM -->
      <div v-if="isOllama" class="space-y-4">
        <div>
          <label class="text-sm font-medium mb-1.5 block">Base URL</label>
          <Input v-model="baseUrl" :placeholder="setupMode ? 'http://host.docker.internal:11434' : ''" />
          <p v-if="setupMode" class="text-xs text-muted-foreground mt-1">
            If Ollama runs on the same machine, use <code>http://host.docker.internal:11434</code>
          </p>
        </div>
        <div>
          <label class="text-sm font-medium mb-1.5 block">Total GPU VRAM (GB)</label>
          <Input v-model="vramGB" type="number" placeholder="e.g. 48" step="1" />
          <p class="text-xs text-muted-foreground mt-1">
            Total VRAM across all GPUs. Used to prevent loading models that don't fit. Leave empty to disable.
          </p>
        </div>
      </div>

      <!-- Cloud providers: API key + budget -->
      <template v-else>
        <div>
          <label class="text-sm font-medium mb-1.5 block">
            API Key
            <span v-if="hasExistingKey" class="text-xs text-muted-foreground font-normal ml-1">(configured)</span>
          </label>
          <Input
            v-model="apiKeyValue"
            type="password"
            :placeholder="hasExistingKey ? 'Enter new key to change' : 'sk-...'"
          />
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="text-sm font-medium mb-1.5 block">Monthly Budget ($)</label>
            <Input v-model="budgetStr" type="number" step="1" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1.5 block">Emergency Reserve ($)</label>
            <Input v-model="reserveStr" type="number" step="0.5" />
          </div>
        </div>
      </template>

      <!-- Model Discovery -->
      <ModelDiscovery
        :provider-name="name"
        :enabled-models="enabledModels"
        :setup-mode="setupMode"
        :base-url="setupMode ? baseUrl : ''"
        :api-key="setupMode ? apiKeyValue : ''"
        @enable="(model) => emit('modelEnable', model)"
        @disable="(key) => emit('modelDisable', key)"
        @update-capabilities="(key, caps) => emit('updateCapabilities', key, caps)"
        @update-prefer-loaded="(key, val) => emit('updatePreferLoaded', key, val)"
      />

      <!-- Save button -->
      <div v-if="showSave" class="flex justify-end">
        <Button
          size="sm"
          @click="emit('save')"
          :disabled="saving"
        >
          {{ saving ? 'Saving...' : 'Save' }}
        </Button>
      </div>
    </template>
  </Card>
</template>
