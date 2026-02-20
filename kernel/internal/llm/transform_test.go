package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- OpenAI-compat message building ---

func TestBuildMessagesUserOnly(t *testing.T) {
	req := &JodoRequest{
		System: "You are helpful.",
		Messages: []JodoMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{ArgsAsJSON: true, IncludeToolCallID: true})

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system+user), got %d", len(msgs))
	}

	sys := msgs[0].(map[string]interface{})
	if sys["role"] != "system" || sys["content"] != "You are helpful." {
		t.Errorf("unexpected system message: %v", sys)
	}

	user := msgs[1].(map[string]interface{})
	if user["role"] != "user" || user["content"] != "Hello" {
		t.Errorf("unexpected user message: %v", user)
	}
}

func TestBuildMessagesNoSystem(t *testing.T) {
	req := &JodoRequest{
		Messages: []JodoMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (no system), got %d", len(msgs))
	}
}

func TestBuildMessagesAssistantWithToolCalls(t *testing.T) {
	req := &JodoRequest{
		Messages: []JodoMessage{
			{Role: "user", Content: "search for X"},
			{
				Role:    "assistant",
				Content: "Let me search.",
				ToolCalls: []ToolCall{
					{ID: "tc_1", Name: "search", Arguments: map[string]interface{}{"query": "X"}},
				},
			},
		},
	}

	// OpenAI mode: ArgsAsJSON=true, IncludeToolCallID=true
	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{ArgsAsJSON: true, IncludeToolCallID: true})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	am := msgs[1].(map[string]interface{})
	tcs := am["tool_calls"].([]map[string]interface{})
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tcs))
	}

	tc := tcs[0]
	if tc["id"] != "tc_1" {
		t.Errorf("expected tool call id tc_1, got %v", tc["id"])
	}
	if tc["type"] != "function" {
		t.Errorf("expected type function, got %v", tc["type"])
	}

	fn := tc["function"].(map[string]interface{})
	argsStr, ok := fn["arguments"].(string)
	if !ok {
		t.Fatalf("expected args as JSON string, got %T", fn["arguments"])
	}
	if !strings.Contains(argsStr, "X") {
		t.Errorf("args string should contain 'X': %s", argsStr)
	}
}

func TestBuildMessagesOllamaMode(t *testing.T) {
	req := &JodoRequest{
		Messages: []JodoMessage{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "tc_1", Name: "read", Arguments: map[string]interface{}{"path": "/tmp"}},
				},
			},
		},
	}

	// Ollama mode: ArgsAsJSON=false, IncludeToolCallID=false
	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{ArgsAsJSON: false, IncludeToolCallID: false})
	am := msgs[0].(map[string]interface{})
	tcs := am["tool_calls"].([]map[string]interface{})

	tc := tcs[0]
	// Should NOT have id or type in Ollama mode
	if _, has := tc["id"]; has {
		t.Error("Ollama mode should not include tool call id")
	}
	if _, has := tc["type"]; has {
		t.Error("Ollama mode should not include tool call type")
	}

	fn := tc["function"].(map[string]interface{})
	// Args should be map, not string
	if _, ok := fn["arguments"].(map[string]interface{}); !ok {
		t.Errorf("Ollama mode should have args as object, got %T", fn["arguments"])
	}
}

func TestBuildMessagesToolResult(t *testing.T) {
	req := &JodoRequest{
		Messages: []JodoMessage{
			{Role: "tool_result", Content: "result data", ToolCallID: "tc_1"},
		},
	}

	// OpenAI mode
	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{IncludeToolCallID: true})
	msg := msgs[0].(map[string]interface{})

	if msg["role"] != "tool" {
		t.Errorf("expected role tool, got %v", msg["role"])
	}
	if msg["tool_call_id"] != "tc_1" {
		t.Errorf("expected tool_call_id tc_1, got %v", msg["tool_call_id"])
	}
}

func TestBuildMessagesToolResultError(t *testing.T) {
	req := &JodoRequest{
		Messages: []JodoMessage{
			{Role: "tool_result", Content: "not found", ToolCallID: "tc_1", IsError: true},
		},
	}

	msgs := buildOpenAICompatMessages(req, openaiCompatOpts{IncludeToolCallID: true})
	msg := msgs[0].(map[string]interface{})

	content := msg["content"].(string)
	if !strings.HasPrefix(content, "Error: ") {
		t.Errorf("expected error prefix, got: %s", content)
	}
}

// --- Tool building ---

func TestBuildToolsEmpty(t *testing.T) {
	req := &JodoRequest{}
	tools := buildOpenAICompatTools(req)
	if tools != nil {
		t.Error("expected nil tools for empty request")
	}
}

func TestBuildToolsNone(t *testing.T) {
	req := &JodoRequest{
		Tools: []ToolDef{{Name: "search", Description: "search things"}},
		ToolChoice: "none",
	}
	tools := buildOpenAICompatTools(req)
	if tools != nil {
		t.Error("expected nil tools when tool_choice is none")
	}
}

func TestBuildTools(t *testing.T) {
	req := &JodoRequest{
		Tools: []ToolDef{
			{Name: "search", Description: "search things", Parameters: map[string]interface{}{"type": "object"}},
		},
	}
	tools := buildOpenAICompatTools(req)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["type"] != "function" {
		t.Errorf("expected type function, got %v", tools[0]["type"])
	}
	fn := tools[0]["function"].(map[string]interface{})
	if fn["name"] != "search" {
		t.Errorf("expected name search, got %v", fn["name"])
	}
}

// --- Tool call arg parsing ---

func TestParseToolCallArgsObject(t *testing.T) {
	raw := json.RawMessage(`{"query": "hello"}`)
	args := parseToolCallArgs(raw)
	if args["query"] != "hello" {
		t.Errorf("expected query=hello, got %v", args["query"])
	}
}

func TestParseToolCallArgsDoubleEncoded(t *testing.T) {
	// OpenAI sometimes returns args as a JSON string
	raw := json.RawMessage(`"{\"query\": \"hello\"}"`)
	args := parseToolCallArgs(raw)
	if args["query"] != "hello" {
		t.Errorf("expected query=hello from double-encoded, got %v", args["query"])
	}
}

func TestParseToolCallArgsEmpty(t *testing.T) {
	args := parseToolCallArgs(nil)
	if args != nil {
		t.Errorf("expected nil for empty args, got %v", args)
	}
}

// --- ID generation ---

func TestGenerateToolCallIDExisting(t *testing.T) {
	id := generateToolCallID("existing_id", "prefix", 0)
	if id != "existing_id" {
		t.Errorf("expected existing_id, got %s", id)
	}
}

func TestGenerateToolCallIDGenerated(t *testing.T) {
	id := generateToolCallID("", "ollama_tc", 3)
	if id != "ollama_tc_3" {
		t.Errorf("expected ollama_tc_3, got %s", id)
	}
}

// --- Provider BuildRequest/ParseResponse ---

func TestClaudeBuildRequestSystemMessage(t *testing.T) {
	prov := &ClaudeProvider{apiKey: "test-key"}
	req := &JodoRequest{
		System:    "Be helpful",
		Messages:  []JodoMessage{{Role: "user", Content: "Hi"}},
		MaxTokens: 100,
	}

	provReq, err := prov.BuildRequest(req, "claude-3-haiku")
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	if provReq.URL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("unexpected URL: %s", provReq.URL)
	}
	if provReq.Headers["x-api-key"] != "test-key" {
		t.Errorf("missing api key header")
	}

	var body map[string]interface{}
	json.Unmarshal(provReq.Body, &body)

	if body["system"] != "Be helpful" {
		t.Errorf("expected system in body, got %v", body["system"])
	}
	if body["model"] != "claude-3-haiku" {
		t.Errorf("expected model claude-3-haiku, got %v", body["model"])
	}
}

func TestClaudeParseResponseText(t *testing.T) {
	prov := &ClaudeProvider{apiKey: "test"}
	body := `{
		"content": [{"type": "text", "text": "Hello there"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "Hello there" {
		t.Errorf("expected content 'Hello there', got %q", resp.Content)
	}
	if !resp.Done {
		t.Error("expected done=true for end_turn")
	}
	if resp.TokensIn != 10 || resp.TokensOut != 5 {
		t.Errorf("unexpected tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestClaudeParseResponseToolUse(t *testing.T) {
	prov := &ClaudeProvider{apiKey: "test"}
	body := `{
		"content": [
			{"type": "text", "text": "I'll search."},
			{"type": "tool_use", "id": "tu_1", "name": "search", "input": {"q": "test"}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 20, "output_tokens": 15}
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Done {
		t.Error("expected done=false for tool_use stop_reason")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("expected tool name search, got %s", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "tu_1" {
		t.Errorf("expected tool id tu_1, got %s", resp.ToolCalls[0].ID)
	}
}

func TestClaudeParseResponseError(t *testing.T) {
	prov := &ClaudeProvider{apiKey: "test"}
	_, err := prov.ParseResponse(400, []byte(`{"error": "bad request"}`))
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestOpenAIParseResponse(t *testing.T) {
	prov := &OpenAIProvider{apiKey: "test"}
	body := `{
		"choices": [{
			"message": {"content": "Hi"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2}
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "Hi" {
		t.Errorf("expected 'Hi', got %q", resp.Content)
	}
	if !resp.Done {
		t.Error("expected done=true for stop finish_reason")
	}
	if resp.TokensIn != 5 || resp.TokensOut != 2 {
		t.Errorf("unexpected tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestOpenAIParseResponseToolCalls(t *testing.T) {
	prov := &OpenAIProvider{apiKey: "test"}
	body := `{
		"choices": [{
			"message": {
				"content": "",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "get_weather", "arguments": "{\"city\": \"NYC\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 8}
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Done {
		t.Error("expected done=false for tool_calls finish_reason")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "get_weather" {
		t.Errorf("unexpected tool call: %+v", tc)
	}
	if tc.Arguments["city"] != "NYC" {
		t.Errorf("expected city=NYC, got %v", tc.Arguments["city"])
	}
}

func TestOllamaParseResponse(t *testing.T) {
	prov := &OllamaProvider{baseURL: "http://localhost:11434"}
	body := `{
		"message": {
			"content": "Answer",
			"tool_calls": []
		},
		"done_reason": "stop",
		"prompt_eval_count": 8,
		"eval_count": 3
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Content != "Answer" {
		t.Errorf("expected 'Answer', got %q", resp.Content)
	}
	if !resp.Done {
		t.Error("expected done=true with no tool calls")
	}
	if resp.TokensIn != 8 || resp.TokensOut != 3 {
		t.Errorf("unexpected tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestOllamaParseResponseToolCalls(t *testing.T) {
	prov := &OllamaProvider{baseURL: "http://localhost:11434"}
	body := `{
		"message": {
			"content": "",
			"tool_calls": [{
				"function": {"name": "bash", "arguments": {"cmd": "ls"}}
			}]
		},
		"done_reason": "stop",
		"prompt_eval_count": 5,
		"eval_count": 4
	}`

	resp, err := prov.ParseResponse(200, []byte(body))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if resp.Done {
		t.Error("expected done=false when tool calls present")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "bash" {
		t.Errorf("expected tool name bash, got %s", tc.Name)
	}
	if tc.ID != "ollama_tc_0" {
		t.Errorf("expected generated id ollama_tc_0, got %s", tc.ID)
	}
	if tc.Arguments["cmd"] != "ls" {
		t.Errorf("expected cmd=ls, got %v", tc.Arguments["cmd"])
	}
}

// --- Claude message transformation ---

func TestClaudeTransformConsecutiveToolResults(t *testing.T) {
	messages := []JodoMessage{
		{Role: "user", Content: "Do things"},
		{Role: "assistant", Content: "OK", ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "a", Arguments: map[string]interface{}{}},
			{ID: "tc_2", Name: "b", Arguments: map[string]interface{}{}},
		}},
		{Role: "tool_result", Content: "result1", ToolCallID: "tc_1"},
		{Role: "tool_result", Content: "result2", ToolCallID: "tc_2"},
	}

	result := claudeTransformMessages(messages)

	// Should be: user, assistant, user (with 2 tool_results grouped)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Third message should be user with grouped tool results
	grouped := result[2].(map[string]interface{})
	if grouped["role"] != "user" {
		t.Errorf("expected role user for grouped tool results, got %v", grouped["role"])
	}

	content := grouped["content"].([]interface{})
	if len(content) != 2 {
		t.Fatalf("expected 2 tool_result blocks, got %d", len(content))
	}

	tr1 := content[0].(map[string]interface{})
	if tr1["type"] != "tool_result" || tr1["tool_use_id"] != "tc_1" {
		t.Errorf("unexpected first tool result: %v", tr1)
	}
}

func TestClaudeTransformErrorToolResult(t *testing.T) {
	messages := []JodoMessage{
		{Role: "tool_result", Content: "failed", ToolCallID: "tc_1", IsError: true},
	}

	result := claudeTransformMessages(messages)
	grouped := result[0].(map[string]interface{})
	content := grouped["content"].([]interface{})
	tr := content[0].(map[string]interface{})

	if tr["is_error"] != true {
		t.Error("expected is_error=true on error tool result")
	}
}
