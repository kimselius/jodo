<script setup lang="ts">
import Button from '@/components/ui/Button.vue'
import ProviderCard from '@/components/shared/ProviderCard.vue'
import type { ProviderSetup } from '@/types/setup'

const props = defineProps<{
  providers: ProviderSetup[]
}>()

defineEmits<{
  next: []
  back: []
}>()

function hasAtLeastOneModel(): boolean {
  return props.providers.some(p => p.enabled && p.models.length > 0)
}

function handleModelEnable(provider: ProviderSetup, model: { model_key: string; model_name: string; input_cost_per_1m: number; output_cost_per_1m: number; capabilities: string[]; quality: number }) {
  const exists = provider.models.find(m => m.model_key === model.model_key)
  if (!exists) {
    provider.models.push({
      ...model,
      vram_estimate_bytes: 0,
      supports_tools: null,
      prefer_loaded: false,
    })
  }
}

function handleModelDisable(provider: ProviderSetup, modelKey: string) {
  const idx = provider.models.findIndex(m => m.model_key === modelKey)
  if (idx >= 0) {
    provider.models.splice(idx, 1)
  }
}

function handleCapabilityUpdate(provider: ProviderSetup, modelKey: string, capabilities: string[]) {
  const model = provider.models.find(m => m.model_key === modelKey)
  if (model) {
    model.capabilities = capabilities
  }
}

function handlePreferLoadedUpdate(provider: ProviderSetup, modelKey: string, preferLoaded: boolean) {
  const model = provider.models.find(m => m.model_key === modelKey)
  if (model) {
    model.prefer_loaded = preferLoaded
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">LLM Providers</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Configure the AI models Jodo will use to think. Enable a provider, then discover and select models.
      </p>
    </div>

    <ProviderCard
      v-for="provider in providers"
      :key="provider.name"
      :name="provider.name"
      v-model:enabled="provider.enabled"
      v-model:base-url="provider.base_url"
      v-model:api-key-value="provider.api_key"
      v-model:monthly-budget="provider.monthly_budget"
      v-model:emergency-reserve="provider.emergency_reserve"
      v-model:total-vram-bytes="provider.total_vram_bytes"
      :enabled-models="provider.models"
      :setup-mode="true"
      @model-enable="(model) => handleModelEnable(provider, model)"
      @model-disable="(key) => handleModelDisable(provider, key)"
      @update-capabilities="(key, caps) => handleCapabilityUpdate(provider, key, caps)"
      @update-prefer-loaded="(key, val) => handlePreferLoadedUpdate(provider, key, val)"
    />

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')" :disabled="!hasAtLeastOneModel()">Next</Button>
    </div>
  </div>
</template>
