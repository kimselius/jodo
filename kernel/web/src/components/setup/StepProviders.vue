<script setup lang="ts">
import { ref } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import ModelDiscovery from '@/components/settings/ModelDiscovery.vue'
import { api } from '@/lib/api'
import type { ProviderSetup } from '@/types/setup'

const props = defineProps<{
  providers: ProviderSetup[]
}>()

defineEmits<{
  next: []
  back: []
}>()

const testing = ref<string | null>(null)
const testResult = ref<Record<string, { valid: boolean; error?: string }>>({})

async function testProvider(provider: ProviderSetup) {
  testing.value = provider.name
  try {
    const res = await api.setupTestProvider(provider.name, provider.api_key, provider.base_url)
    testResult.value[provider.name] = res
  } catch (e) {
    testResult.value[provider.name] = {
      valid: false,
      error: e instanceof Error ? e.message : 'Test failed',
    }
  } finally {
    testing.value = null
  }
}

function hasAtLeastOneProvider(): boolean {
  return props.providers.some(p => p.enabled)
}

function handleModelEnable(provider: ProviderSetup, model: { model_key: string; model_name: string; input_cost_per_1m: number; output_cost_per_1m: number; capabilities: string[]; quality: number }) {
  const exists = provider.models.find(m => m.model_key === model.model_key)
  if (!exists) {
    provider.models.push({ ...model })
  }
}

function handleModelDisable(provider: ProviderSetup, modelKey: string) {
  const idx = provider.models.findIndex(m => m.model_key === modelKey)
  if (idx >= 0) {
    provider.models.splice(idx, 1)
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">LLM Providers</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Configure the AI models Jodo will use to think. Ollama (local, free) is enabled by default.
      </p>
    </div>

    <Card v-for="provider in providers" :key="provider.name" class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <h3 class="text-sm font-semibold capitalize">{{ provider.name }}</h3>
          <span v-if="provider.name === 'ollama'" class="text-[10px] uppercase tracking-wider text-muted-foreground bg-muted px-1.5 py-0.5 rounded">Free</span>
        </div>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            v-model="provider.enabled"
            class="rounded border-input"
          />
          <span class="text-xs text-muted-foreground">Enabled</span>
        </label>
      </div>

      <template v-if="provider.enabled">
        <!-- Ollama: base URL only -->
        <div v-if="provider.name === 'ollama'">
          <label class="text-sm font-medium mb-1.5 block">Base URL</label>
          <Input v-model="provider.base_url" placeholder="http://host.docker.internal:11434" />
          <p class="text-xs text-muted-foreground mt-1">
            If Ollama runs on the same machine, use <code>http://host.docker.internal:11434</code>
          </p>
        </div>

        <!-- Claude/OpenAI: API key + budget -->
        <template v-else>
          <div>
            <label class="text-sm font-medium mb-1.5 block">API Key</label>
            <Input v-model="provider.api_key" type="password" placeholder="sk-..." />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="text-sm font-medium mb-1.5 block">Monthly Budget ($)</label>
              <Input v-model.number="provider.monthly_budget" type="number" step="1" />
            </div>
            <div>
              <label class="text-sm font-medium mb-1.5 block">Emergency Reserve ($)</label>
              <Input v-model.number="provider.emergency_reserve" type="number" step="0.5" />
            </div>
          </div>
        </template>

        <!-- Test button -->
        <div class="flex items-center gap-3">
          <Button
            size="sm"
            variant="secondary"
            @click="testProvider(provider)"
            :disabled="testing === provider.name"
          >
            {{ testing === provider.name ? 'Testing...' : 'Test Connection' }}
          </Button>
          <span v-if="testResult[provider.name]?.valid" class="text-xs text-green-500">Connected</span>
          <span v-else-if="testResult[provider.name]?.error" class="text-xs text-destructive">
            {{ testResult[provider.name].error }}
          </span>
        </div>

        <!-- Model Discovery -->
        <ModelDiscovery
          :provider-name="provider.name"
          :enabled-models="provider.models"
          @enable="(model) => handleModelEnable(provider, model)"
          @disable="(key) => handleModelDisable(provider, key)"
        />
      </template>
    </Card>

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')" :disabled="!hasAtLeastOneProvider()">Next</Button>
    </div>
  </div>
</template>
