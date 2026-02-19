export interface StatusResponse {
  kernel: {
    status: string
    uptime_seconds: number
    version: string
  }
  jodo: {
    status: 'running' | 'starting' | 'unhealthy' | 'dead' | 'rebirthing' | string
    pid: number
    galla: number
    phase: 'booting' | 'thinking' | 'sleeping' | string
    uptime_seconds: number
    last_health_check: string
    health_check_ok: boolean
    restarts_today: number
    current_git_tag: string
    current_git_hash: string
  }
  database: {
    status: string
    memories_stored: number
  }
}

export interface ProviderBudget {
  monthly_budget: number
  spent_this_month: number
  remaining: number
  emergency_reserve: number
  available_for_normal_use: number
}

export interface BudgetResponse {
  providers: Record<string, ProviderBudget>
  total_spent_today: number
  budget_resets: string
}
