import type {
  SetupStatus, SSHGenerateResponse, SSHVerifyResponse,
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

  /** Save config for a named setup step. Called on each "Next" click. */
  setupSaveStep(step: string, data: Record<string, unknown>) {
    return request<{ ok: boolean }>(`/api/setup/step/${step}`, {
      method: 'POST',
      body: JSON.stringify(data),
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
}
