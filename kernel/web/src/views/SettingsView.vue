<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSettings } from '@/composables/useSettings'
import GenesisTab from '@/components/settings/GenesisTab.vue'
import ProvidersTab from '@/components/settings/ProvidersTab.vue'
import RoutingTab from '@/components/settings/RoutingTab.vue'
import KernelTab from '@/components/settings/KernelTab.vue'
import VPSTab from '@/components/settings/VPSTab.vue'
import SubagentsTab from '@/components/settings/SubagentsTab.vue'

const {
  providers,
  genesis,
  routing,
  kernel,
  ssh,
  subagent,
  loading,
  error,
  saved,
  loadAll,
  loadProviders,
  loadGenesis,
  loadRouting,
  loadKernel,
  loadSSH,
  loadSubagent,
  showSaved,
} = useSettings()

const activeTab = ref('genesis')

const tabs = [
  { key: 'genesis', label: 'Identity' },
  { key: 'providers', label: 'Providers' },
  { key: 'routing', label: 'Routing' },
  { key: 'kernel', label: 'Kernel' },
  { key: 'subagents', label: 'Subagents' },
  { key: 'vps', label: 'VPS' },
]

onMounted(() => loadAll())

function handleSaved(reloadFn?: () => Promise<void>) {
  showSaved()
  if (reloadFn) reloadFn()
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4 space-y-4">
      <h1 class="text-lg font-semibold">Settings</h1>

      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <p v-if="saved" class="text-sm text-green-500">Saved successfully.</p>

      <!-- Tabs -->
      <div class="flex gap-1 border-b border-border">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          @click="activeTab = tab.key"
          class="px-3 py-2 text-sm font-medium transition-colors relative"
          :class="[
            activeTab === tab.key
              ? 'text-foreground'
              : 'text-muted-foreground hover:text-foreground',
          ]"
        >
          {{ tab.label }}
          <span
            v-if="activeTab === tab.key"
            class="absolute bottom-0 left-0 right-0 h-0.5 bg-primary"
          />
        </button>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <template v-else>
        <GenesisTab
          v-if="activeTab === 'genesis' && genesis"
          :genesis="genesis"
          @saved="handleSaved(loadGenesis)"
        />

        <ProvidersTab
          v-if="activeTab === 'providers'"
          :providers="providers"
          @saved="handleSaved(loadProviders)"
        />

        <RoutingTab
          v-if="activeTab === 'routing' && routing"
          :routing="routing"
          :providers="providers"
          @saved="handleSaved(loadRouting)"
        />

        <KernelTab
          v-if="activeTab === 'kernel' && kernel"
          :kernel="kernel"
          @saved="handleSaved(loadKernel)"
        />

        <SubagentsTab
          v-if="activeTab === 'subagents' && subagent"
          :subagent="subagent"
          @saved="handleSaved(loadSubagent)"
        />

        <VPSTab
          v-if="activeTab === 'vps' && ssh"
          :ssh="ssh"
          @saved="handleSaved(loadSSH)"
        />
      </template>
    </div>
  </div>
</template>
