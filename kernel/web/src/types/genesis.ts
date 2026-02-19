export interface Genesis {
  identity: {
    name: string
    version: number
  }
  purpose: string
  survival_instincts: string[]
  capabilities: {
    kernel_api: Record<string, string>
    local: string[]
  }
  first_tasks: string[]
  hints: string[]
}

export interface IdentityUpdate {
  name?: string
  purpose?: string
}
