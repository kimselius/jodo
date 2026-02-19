import { ref } from 'vue'
import { api } from '@/lib/api'
import type { ProviderInfo, KernelSettings, RoutingConfig, SSHStatus } from '@/types/setup'
import type { Genesis } from '@/types/genesis'

export function useSettings() {
  const providers = ref<ProviderInfo[]>([])
  const genesis = ref<Genesis | null>(null)
  const routing = ref<RoutingConfig | null>(null)
  const kernel = ref<KernelSettings | null>(null)
  const ssh = ref<SSHStatus | null>(null)
  const subagent = ref<{ max_concurrent: number; max_timeout: number } | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)
  const saved = ref(false)

  async function loadProviders() {
    try {
      const res = await api.getSettingsProviders()
      providers.value = res.providers || []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load providers'
    }
  }

  async function loadGenesis() {
    try {
      genesis.value = await api.getSettingsGenesis()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load genesis'
    }
  }

  async function loadRouting() {
    try {
      routing.value = await api.getSettingsRouting()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load routing'
    }
  }

  async function loadKernel() {
    try {
      kernel.value = await api.getSettingsKernel()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load kernel settings'
    }
  }

  async function loadSSH() {
    try {
      ssh.value = await api.getSettingsSSH()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load SSH status'
    }
  }

  async function loadSubagent() {
    try {
      subagent.value = await api.getSettingsSubagent()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load subagent settings'
    }
  }

  async function loadAll() {
    loading.value = true
    error.value = null
    await Promise.all([loadProviders(), loadGenesis(), loadRouting(), loadKernel(), loadSSH(), loadSubagent()])
    loading.value = false
  }

  function showSaved() {
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  }

  return {
    providers,
    genesis,
    routing,
    kernel,
    ssh,
    subagent,
    loading,
    saving,
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
  }
}
