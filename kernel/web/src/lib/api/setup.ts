import type {
  SetupStatus, SSHGenerateResponse, SSHVerifyResponse,
  TestProviderResponse, ProviderSetup, GenesisSetup,
  ProvisionResult,
} from '@/types/setup'
import { request } from './request'

export const setupApi = {
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

  setupRouting(intentPreferences: Record<string, string[]>) {
    return request<{ ok: boolean }>('/api/setup/routing', {
      method: 'POST',
      body: JSON.stringify({ intent_preferences: intentPreferences }),
    })
  },

  setupDiscoverModels(provider: string, baseUrl?: string, apiKey?: string) {
    return request<{ models: unknown[]; error?: string }>('/api/setup/discover', {
      method: 'POST',
      body: JSON.stringify({ provider, base_url: baseUrl, api_key: apiKey }),
    })
  },
}
