import type {
  ProviderSetup, ProviderInfo, GenesisSetup,
  SSHStatus, KernelSettings, RoutingConfig,
} from '@/types/setup'
import type { Genesis } from '@/types/genesis'
import { request } from './request'

export const settingsApi = {
  getProviders() {
    return request<{ providers: ProviderInfo[] }>('/api/settings/providers')
  },

  updateProvider(name: string, update: Partial<ProviderSetup>) {
    return request<{ ok: boolean }>(`/api/settings/providers/${name}`, {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  addModel(providerName: string, model: ProviderSetup['models'][0]) {
    return request<{ ok: boolean }>(`/api/settings/providers/${providerName}/models`, {
      method: 'POST',
      body: JSON.stringify(model),
    })
  },

  updateModel(providerName: string, modelKey: string, update: Record<string, unknown>) {
    return request<{ ok: boolean }>(`/api/settings/providers/${providerName}/models/${modelKey}`, {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  deleteModel(providerName: string, modelKey: string) {
    return request<{ ok: boolean }>(`/api/settings/providers/${providerName}/models/${modelKey}`, {
      method: 'DELETE',
    })
  },

  discoverModels(providerName: string) {
    return request<{ models: unknown[]; error?: string }>(`/api/settings/providers/${providerName}/discover`)
  },

  getGenesis() {
    return request<Genesis>('/api/settings/genesis')
  },

  updateGenesis(genesis: GenesisSetup) {
    return request<{ ok: boolean }>('/api/settings/genesis', {
      method: 'PUT',
      body: JSON.stringify(genesis),
    })
  },

  getRouting() {
    return request<RoutingConfig>('/api/settings/routing')
  },

  updateRouting(routing: RoutingConfig) {
    return request<{ ok: boolean }>('/api/settings/routing', {
      method: 'PUT',
      body: JSON.stringify(routing),
    })
  },

  getKernel() {
    return request<KernelSettings>('/api/settings/kernel')
  },

  updateKernel(settings: Partial<KernelSettings>) {
    return request<{ ok: boolean }>('/api/settings/kernel', {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },

  getSSH() {
    return request<SSHStatus>('/api/settings/ssh')
  },

  getSubagent() {
    return request<{ max_concurrent: number; max_timeout: number }>('/api/settings/subagent')
  },

  updateSubagent(settings: { max_concurrent?: number; max_timeout?: number }) {
    return request<{ ok: boolean }>('/api/settings/subagent', {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },

  getVRAMStatus() {
    return request<{ enabled: boolean; total_vram_bytes: number; used_vram_bytes?: number; free_vram_bytes?: number; loaded_models?: Array<{ name: string; size_vram: number }> }>('/api/settings/vram')
  },
}
