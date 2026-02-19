import type { ChatMessage } from '@/types/chat'
import type { StatusResponse, BudgetResponse } from '@/types/status'
import type { Genesis, IdentityUpdate } from '@/types/genesis'
import type { CommitEntry } from '@/types/history'
import type { MemoryEntry } from '@/types/memory'
import type { GrowthEvent, GallaEntry } from '@/types/growth'
import type { LibraryItem, LibraryComment } from '@/types/library'
import type {
  SetupStatus, SSHGenerateResponse, SSHVerifyResponse,
  TestProviderResponse, ProviderSetup, GenesisSetup,
  ProviderInfo, SSHStatus, KernelSettings, RoutingConfig,
  ProvisionResult,
} from '@/types/setup'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  // Chat
  getMessages(params?: Record<string, string>) {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return request<{ messages: ChatMessage[] }>(`/api/chat${qs}`)
  },

  sendMessage(message: string) {
    return request<{ ok: boolean; id: number }>('/api/chat', {
      method: 'POST',
      body: JSON.stringify({ message, source: 'human' }),
    })
  },

  ackMessages(upToId: number) {
    return request<{ ok: boolean; marked: number }>('/api/chat/ack', {
      method: 'POST',
      body: JSON.stringify({ up_to_id: upToId }),
    })
  },

  // Status
  getStatus() {
    return request<StatusResponse>('/api/status')
  },

  getBudget() {
    return request<BudgetResponse>('/api/budget')
  },

  getBudgetBreakdown() {
    return request<{ breakdown: Array<{ provider: string; model: string; intent: string; calls: number; tokens_in: number; tokens_out: number; cost: number }> }>('/api/budget/breakdown')
  },

  // Genesis
  getGenesis() {
    return request<Genesis>('/api/genesis')
  },

  updateIdentity(update: IdentityUpdate) {
    return request<{ ok: boolean }>('/api/genesis/identity', {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  // History
  getHistory() {
    return request<{ commits: CommitEntry[] }>('/api/history')
  },

  // Memories
  getMemories(limit = 50, offset = 0) {
    return request<{ memories: MemoryEntry[]; total: number }>(
      `/api/memories?limit=${limit}&offset=${offset}`
    )
  },

  // Growth
  getGrowth(limit = 50) {
    return request<{ events: GrowthEvent[] }>(`/api/growth?limit=${limit}`)
  },

  getGallas(limit = 50) {
    return request<{ gallas: GallaEntry[] }>(`/api/galla?limit=${limit}`)
  },

  // Lifecycle
  restart() {
    return request<{ status: string }>('/api/restart', { method: 'POST' })
  },

  // Setup
  getSetupStatus() {
    return request<SetupStatus>('/api/setup/status')
  },

  setupSSHGenerate() {
    return request<SSHGenerateResponse>('/api/setup/ssh/generate', { method: 'POST' })
  },

  setupSSHVerify(host: string, sshUser: string) {
    return request<SSHVerifyResponse>('/api/setup/ssh/verify', {
      method: 'POST',
      body: JSON.stringify({ host, ssh_user: sshUser }),
    })
  },

  setupConfig(kernelUrl: string) {
    return request<{ ok: boolean }>('/api/setup/config', {
      method: 'POST',
      body: JSON.stringify({ kernel_url: kernelUrl }),
    })
  },

  setupProviders(providers: ProviderSetup[]) {
    return request<{ ok: boolean }>('/api/setup/providers', {
      method: 'POST',
      body: JSON.stringify({ providers }),
    })
  },

  setupGenesis(genesis: GenesisSetup) {
    return request<{ ok: boolean }>('/api/setup/genesis', {
      method: 'POST',
      body: JSON.stringify(genesis),
    })
  },

  setupTestProvider(provider: string, apiKey: string, baseUrl?: string) {
    return request<TestProviderResponse>('/api/setup/test-provider', {
      method: 'POST',
      body: JSON.stringify({ provider, api_key: apiKey, base_url: baseUrl }),
    })
  },

  setupBirth() {
    return request<{ ok: boolean; message: string }>('/api/setup/birth', { method: 'POST' })
  },

  setupProvision(brainPath: string) {
    return request<ProvisionResult>('/api/setup/provision', {
      method: 'POST',
      body: JSON.stringify({ brain_path: brainPath }),
    })
  },

  setupDiscoverModels(provider: string, baseUrl?: string, apiKey?: string) {
    return request<{ models: unknown[]; error?: string }>('/api/setup/discover', {
      method: 'POST',
      body: JSON.stringify({ provider, base_url: baseUrl, api_key: apiKey }),
    })
  },

  // Settings
  getSettingsProviders() {
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

  getSettingsGenesis() {
    return request<Genesis>('/api/settings/genesis')
  },

  updateSettingsGenesis(genesis: GenesisSetup) {
    return request<{ ok: boolean }>('/api/settings/genesis', {
      method: 'PUT',
      body: JSON.stringify(genesis),
    })
  },

  getSettingsRouting() {
    return request<RoutingConfig>('/api/settings/routing')
  },

  updateSettingsRouting(routing: RoutingConfig) {
    return request<{ ok: boolean }>('/api/settings/routing', {
      method: 'PUT',
      body: JSON.stringify(routing),
    })
  },

  getSettingsKernel() {
    return request<KernelSettings>('/api/settings/kernel')
  },

  updateSettingsKernel(settings: Partial<KernelSettings>) {
    return request<{ ok: boolean }>('/api/settings/kernel', {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },

  getSettingsSSH() {
    return request<SSHStatus>('/api/settings/ssh')
  },

  getSettingsSubagent() {
    return request<{ max_concurrent: number; max_timeout: number }>('/api/settings/subagent')
  },

  updateSettingsSubagent(settings: { max_concurrent?: number; max_timeout?: number }) {
    return request<{ ok: boolean }>('/api/settings/subagent', {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },

  // Library
  getLibrary(status?: string) {
    const qs = status ? `?status=${status}` : ''
    return request<{ items: LibraryItem[] }>(`/api/library${qs}`)
  },

  createLibraryItem(title: string, content: string, priority = 0) {
    return request<{ ok: boolean; item: LibraryItem }>('/api/library', {
      method: 'POST',
      body: JSON.stringify({ title, content, priority }),
    })
  },

  updateLibraryItem(id: number, update: { title?: string; content?: string; priority?: number }) {
    return request<{ ok: boolean }>(`/api/library/${id}`, {
      method: 'PUT',
      body: JSON.stringify(update),
    })
  },

  patchLibraryStatus(id: number, status: string) {
    return request<{ ok: boolean }>(`/api/library/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
  },

  deleteLibraryItem(id: number) {
    return request<{ ok: boolean }>(`/api/library/${id}`, { method: 'DELETE' })
  },

  addLibraryComment(id: number, message: string) {
    return request<{ ok: boolean; comment: LibraryComment }>(`/api/library/${id}/comments`, {
      method: 'POST',
      body: JSON.stringify({ source: 'human', message }),
    })
  },

  // Inbox
  getInbox(limit = 200) {
    return request<{ messages: Array<{ id: number; source: string; target: string; message: string; galla?: number; created_at: string }> }>(
      `/api/inbox?limit=${limit}`
    )
  },
}
