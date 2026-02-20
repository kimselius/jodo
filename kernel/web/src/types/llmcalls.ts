export interface LLMCallSummary {
  id: number
  intent: string
  provider: string
  model: string
  tokens_in: number
  tokens_out: number
  cost: number
  duration_ms: number
  chain_id?: string
  error?: string
  created_at: string
}

export interface LLMCallDetail extends LLMCallSummary {
  request_system?: string
  request_messages: unknown[]
  request_tools?: unknown[]
  response_content?: string
  response_tool_calls?: unknown[]
  response_done: boolean
}
