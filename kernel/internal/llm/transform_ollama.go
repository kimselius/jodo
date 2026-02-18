package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"jodo-kernel/internal/config"
)

// OllamaProvider implements the Provider interface for Ollama's local API.
type OllamaProvider struct {
	baseURL string
	client  *http.Client
}

func NewOllamaProvider(cfg config.ProviderConfig) *OllamaProvider {
	base := cfg.BaseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL: base,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OllamaProvider) Name() string         { return "ollama" }
func (o *OllamaProvider) SupportsTools() bool   { return true }
func (o *OllamaProvider) SupportsEmbed() bool   { return true }

func (o *OllamaProvider) BuildRequest(req *JodoRequest, model string) (*ProviderHTTPRequest, error) {
	// Ollama follows OpenAI-compatible format for messages and tools
	var msgs []interface{}

	if req.System != "" {
		msgs = append(msgs, map[string]interface{}{
			"role":    "system",
			"content": req.System,
		})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, map[string]interface{}{
				"role":    "user",
				"content": m.Content,
			})

		case "assistant":
			am := map[string]interface{}{
				"role":    "assistant",
				"content": m.Content,
			}
			if len(m.ToolCalls) > 0 {
				tcs := make([]map[string]interface{}, len(m.ToolCalls))
				for j, tc := range m.ToolCalls {
					tcs[j] = map[string]interface{}{
						"function": map[string]interface{}{
							"name":      tc.Name,
							"arguments": tc.Arguments, // Ollama accepts objects directly
						},
					}
				}
				am["tool_calls"] = tcs
			}
			msgs = append(msgs, am)

		case "tool_result":
			content := m.Content
			if m.IsError {
				content = "Error: " + content
			}
			msgs = append(msgs, map[string]interface{}{
				"role":    "tool",
				"content": content,
			})
		}
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": msgs,
		"stream":   false,
	}

	// Transform tools (same format as OpenAI)
	if len(req.Tools) > 0 {
		switch req.ToolChoice {
		case "none":
			// Don't send tools
		default:
			tools := make([]map[string]interface{}, len(req.Tools))
			for i, t := range req.Tools {
				tools[i] = map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  t.Parameters,
					},
				}
			}
			body["tools"] = tools
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama marshal: %w", err)
	}

	return &ProviderHTTPRequest{
		URL: o.baseURL + "/api/chat",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: jsonBody,
	}, nil
}

func (o *OllamaProvider) ParseResponse(statusCode int, body []byte) (*ProviderHTTPResponse, error) {
	if statusCode != 200 {
		return nil, fmt.Errorf("ollama %d: %s", statusCode, string(body))
	}

	var result struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		DoneReason      string `json:"done_reason"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ollama parse: %w", err)
	}

	var toolCalls []ToolCall
	for i, tc := range result.Message.ToolCalls {
		var args map[string]interface{}
		if len(tc.Function.Arguments) > 0 {
			// Ollama may return arguments as object or string — handle both
			if tc.Function.Arguments[0] == '"' {
				var jsonStr string
				json.Unmarshal(tc.Function.Arguments, &jsonStr)
				json.Unmarshal([]byte(jsonStr), &args)
			} else {
				json.Unmarshal(tc.Function.Arguments, &args)
			}
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        fmt.Sprintf("ollama_tc_%d", i),
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	done := len(toolCalls) == 0 // Ollama: if there are tool calls, we're not done

	return &ProviderHTTPResponse{
		Content:   result.Message.Content,
		ToolCalls: toolCalls,
		Done:      done,
		TokensIn:  result.PromptEvalCount,
		TokensOut: result.EvalCount,
	}, nil
}

func (o *OllamaProvider) Embed(ctx context.Context, model string, text string) ([]float32, int, error) {
	body := map[string]interface{}{
		"model": model,
		"input": text,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/embed", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("ollama embed %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, 0, fmt.Errorf("ollama embed parse: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, 0, fmt.Errorf("ollama: no embedding returned")
	}

	embedding := result.Embeddings[0]

	// Truncate to 1024 dims for Matryoshka-compatible models (e.g. qwen3-embedding:8b)
	const targetDim = 1024
	if len(embedding) > targetDim {
		embedding = embedding[:targetDim]
	}

	return embedding, 0, nil
}
