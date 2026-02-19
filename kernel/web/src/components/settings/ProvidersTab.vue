<script setup lang="ts">
import { ref } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import ModelDiscovery from './ModelDiscovery.vue'
import { api } from '@/lib/api'
import type { ProviderInfo } from '@/types/setup'

const props = defineProps<{
  providers: ProviderInfo[]
}>()

const emit = defineEmits<{ saved: [] }>()

const saving = ref<string | null>(null)
const error = ref<string | null>(null)
const testing = ref<string | null>(null)
const testResult = ref<Record<string, { valid: boolean; error?: string }>>({})

// Track new API key input per provider (separate from display)
const newApiKey = ref<Record<string, string>>({})

async function updateProvider(p: ProviderInfo) {
  saving.value = p.name
  error.value = null
  try {
    const update: Record<string, unknown> = {
      enabled: p.enabled,
      base_url: p.base_url,
      monthly_budget: p.monthly_budget,
      emergency_reserve: p.emergency_reserve,
    }
    if (newApiKey.value[p.name]) {
      update.api_key = newApiKey.value[p.name]
    }
    await api.updateProvider(p.name, update)
    newApiKey.value[p.name] = ''
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Update failed'
  } finally {
    saving.value = null
  }
}

async function testProvider(p: ProviderInfo) {
  testing.value = p.name
  try {
    const res = await api.setupTestProvider(p.name, newApiKey.value[p.name] || '', p.base_url)
    testResult.value[p.name] = res
  } catch (e) {
    testResult.value[p.name] = { valid: false, error: e instanceof Error ? e.message : 'Test failed' }
  } finally {
    testing.value = null
  }
}

function enabledModelsFor(p: ProviderInfo) {
  return p.models.filter(m => m.enabled)
}

async function handleModelEnable(providerName: string, model: { model_key: string; model_name: string; input_cost_per_1m: number; output_cost_per_1m: number; capabilities: string[]; quality: number }) {
  try {
    await api.addModel(providerName, model)
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to enable model'
  }
}

async function handleModelDisable(providerName: string, modelKey: string) {
  try {
    await api.updateModel(providerName, modelKey, { enabled: false })
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to disable model'
  }
}

async function handleCapabilityUpdate(providerName: string, modelKey: string, capabilities: string[]) {
  try {
    await api.updateModel(providerName, modelKey, { capabilities })
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to update capabilities'
  }
}
</script>

<template>
  <div class="space-y-4">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <Card v-for="p in providers" :key="p.name" class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold capitalize">{{ p.name }}</h3>
        <label class="flex items-center gap-2 cursor-pointer">
          <input type="checkbox" v-model="p.enabled" class="rounded border-input" />
          <span class="text-xs text-muted-foreground">Enabled</span>
        </label>
      </div>

      <template v-if="p.enabled">
        <div v-if="p.name === 'ollama'">
          <label class="text-sm font-medium mb-1.5 block">Base URL</label>
          <Input v-model="p.base_url" />
        </div>

        <template v-else>
          <div>
            <label class="text-sm font-medium mb-1.5 block">
              API Key
              <span v-if="p.has_api_key" class="text-xs text-muted-foreground font-normal ml-1">(configured)</span>
            </label>
            <Input
              v-model="newApiKey[p.name]"
              type="password"
              :placeholder="p.has_api_key ? 'Enter new key to change' : 'sk-...'"
            />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="text-sm font-medium mb-1.5 block">Monthly Budget ($)</label>
              <Input v-model.number="p.monthly_budget" type="number" step="1" />
            </div>
            <div>
              <label class="text-sm font-medium mb-1.5 block">Emergency Reserve ($)</label>
              <Input v-model.number="p.emergency_reserve" type="number" step="0.5" />
            </div>
          </div>
        </template>

        <!-- Model Discovery -->
        <ModelDiscovery
          :provider-name="p.name"
          :enabled-models="enabledModelsFor(p)"
          @enable="(model) => handleModelEnable(p.name, model)"
          @disable="(key) => handleModelDisable(p.name, key)"
          @update-capabilities="(key, caps) => handleCapabilityUpdate(p.name, key, caps)"
        />

        <div class="flex items-center gap-3">
          <Button
            size="sm"
            variant="secondary"
            @click="testProvider(p)"
            :disabled="testing === p.name"
          >
            {{ testing === p.name ? 'Testing...' : 'Test' }}
          </Button>
          <span v-if="testResult[p.name]?.valid" class="text-xs text-green-500">OK</span>
          <span v-else-if="testResult[p.name]?.error" class="text-xs text-destructive">{{ testResult[p.name].error }}</span>

          <div class="flex-1" />

          <Button
            size="sm"
            @click="updateProvider(p)"
            :disabled="saving === p.name"
          >
            {{ saving === p.name ? 'Saving...' : 'Save' }}
          </Button>
        </div>
      </template>
    </Card>
  </div>
</template>
