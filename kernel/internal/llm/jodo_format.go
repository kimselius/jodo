package llm

// JodoRequest is what Jodo sends to POST /api/think.
// The kernel translates this to whatever format the chosen provider needs.
type JodoRequest struct {
	Intent     string        `json:"intent"`
	System     string        `json:"system,omitempty"`
	Messages   []JodoMessage `json:"messages"`
	Tools      []ToolDef     `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"` // auto, none, required
	MaxTokens  int           `json:"max_tokens,omitempty"`
	MaxCost    float64       `json:"max_cost,omitempty"`
	ChainID    string        `json:"chain_id,omitempty"`
}

// JodoMessage is a message in Jodo Format. Roles: user, assistant, tool_result.
type JodoMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // present on assistant messages
	ToolCallID string     `json:"tool_call_id,omitempty"` // present on tool_result messages
	IsError    bool       `json:"is_error,omitempty"`     // present on tool_result messages
}

// ToolDef is a tool definition that Jodo sends to the kernel.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall is a tool invocation request from the model.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// JodoResponse is what the kernel returns to Jodo from POST /api/think.
type JodoResponse struct {
	Content         string                 `json:"content"`
	ToolCalls       []ToolCall             `json:"tool_calls,omitempty"`
	Done            bool                   `json:"done"`
	ModelUsed       string                 `json:"model_used"`
	Provider        string                 `json:"provider"`
	TokensIn        int                    `json:"tokens_in"`
	TokensOut       int                    `json:"tokens_out"`
	Cost            float64                `json:"cost"`
	TotalChainCost  float64                `json:"total_chain_cost"`
	BudgetRemaining map[string]interface{} `json:"budget_remaining"`
}

// ProviderHTTPRequest is the output of BuildRequest — the raw HTTP call the proxy makes.
type ProviderHTTPRequest struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

// ProviderHTTPResponse is the parsed output from ParseResponse.
type ProviderHTTPResponse struct {
	Content   string
	ToolCalls []ToolCall
	Done      bool
	TokensIn  int
	TokensOut int
}
