export interface SetupStatus {
  setup_complete: boolean
  jodo_mode: 'vps' | 'docker'
}

export interface SSHGenerateResponse {
  public_key: string
}

export interface SSHVerifyResponse {
  connected: boolean
  error?: string
}

export interface TestProviderResponse {
  valid: boolean
  error?: string
}

export interface ProviderSetup {
  name: string
  enabled: boolean
  api_key: string
  base_url: string
  monthly_budget: number
  emergency_reserve: number
  total_vram_bytes: number
  models: ModelSetup[]
}

export interface ModelSetup {
  model_key: string
  model_name: string
  input_cost_per_1m: number
  output_cost_per_1m: number
  capabilities: string[]
  quality: number
  vram_estimate_bytes: number
  supports_tools: boolean | null
}

export interface GenesisSetup {
  name: string
  purpose: string
  survival_instincts: string[]
  first_tasks: string[]
  hints: string[]
  capabilities_api: Record<string, string>
  capabilities_local: string[]
}

export interface ProviderInfo {
  name: string
  enabled: boolean
  has_api_key: boolean
  base_url: string
  monthly_budget: number
  emergency_reserve: number
  total_vram_bytes: number
  models: ModelInfo[]
}

export interface ModelInfo {
  model_key: string
  model_name: string
  input_cost_per_1m: number
  output_cost_per_1m: number
  capabilities: string[]
  quality: number
  enabled: boolean
  vram_estimate_bytes: number
  supports_tools: boolean | null
}

export interface SSHStatus {
  host: string
  user: string
  has_key: boolean
  brain_path: string
  jodo_mode: 'vps' | 'docker'
}

export interface ProvisionStep {
  name: string
  ok: boolean
  output: string
}

export interface ProvisionResult {
  success: boolean
  steps: ProvisionStep[]
}

export interface KernelSettings {
  health_check_interval: number
  max_restart_attempts: number
  log_level: string
  audit_log_path: string
  external_url: string
}

export interface RoutingConfig {
  strategy: string
  intent_preferences: Record<string, string[]>
}
