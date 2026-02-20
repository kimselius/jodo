<script setup lang="ts">
import { ref } from 'vue'
import ProviderCard from '@/components/shared/ProviderCard.vue'
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

// Track new API key input per provider (never pre-populated from backend)
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
      total_vram_bytes: p.total_vram_bytes || 0,
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

async function handlePreferLoadedUpdate(providerName: string, modelKey: string, preferLoaded: boolean) {
  try {
    await api.updateModel(providerName, modelKey, { prefer_loaded: preferLoaded })
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to update prefer loaded'
  }
}
</script>

<template>
  <div class="space-y-4">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <ProviderCard
      v-for="p in providers"
      :key="p.name"
      :name="p.name"
      v-model:enabled="p.enabled"
      v-model:base-url="p.base_url"
      v-model:api-key-value="newApiKey[p.name]"
      :has-existing-key="p.has_api_key"
      v-model:monthly-budget="p.monthly_budget"
      v-model:emergency-reserve="p.emergency_reserve"
      v-model:total-vram-bytes="p.total_vram_bytes"
      :enabled-models="enabledModelsFor(p)"
      :show-save="true"
      :testing="testing === p.name"
      :saving="saving === p.name"
      :test-result="testResult[p.name] ?? null"
      @test="testProvider(p)"
      @save="updateProvider(p)"
      @model-enable="(model) => handleModelEnable(p.name, model)"
      @model-disable="(key) => handleModelDisable(p.name, key)"
      @update-capabilities="(key, caps) => handleCapabilityUpdate(p.name, key, caps)"
      @update-prefer-loaded="(key, val) => handlePreferLoadedUpdate(p.name, key, val)"
    />
  </div>
</template>
