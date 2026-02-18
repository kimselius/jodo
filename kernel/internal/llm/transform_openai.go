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

// OpenAIProvider implements the Provider interface for OpenAI's API.
type OpenAIProvider struct {
	apiKey string
	client *http.Client
}

func NewOpenAIProvider(cfg config.ProviderConfig) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAIProvider) Name() string         { return "openai" }
func (o *OpenAIProvider) SupportsTools() bool   { return true }
func (o *OpenAIProvider) SupportsEmbed() bool   { return true }

func (o *OpenAIProvider) BuildRequest(req *JodoRequest, model string) (*ProviderHTTPRequest, error) {
	// Build messages: system goes as first message
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
				// Transform tool calls: arguments must be a JSON string
				tcs := make([]map[string]interface{}, len(m.ToolCalls))
				for j, tc := range m.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Arguments)
					tcs[j] = map[string]interface{}{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Name,
							"arguments": string(argsJSON),
						},
					}
				}
				am["tool_calls"] = tcs
			}
			msgs = append(msgs, am)

		case "tool_result":
			// OpenAI uses role "tool" with tool_call_id
			content := m.Content
			if m.IsError {
				content = "Error: " + content
			}
			msgs = append(msgs, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": m.ToolCallID,
				"content":      content,
			})
		}
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": msgs,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	// Transform tools
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

		switch req.ToolChoice {
		case "auto", "":
			body["tool_choice"] = "auto"
		case "required":
			body["tool_choice"] = "required"
		case "none":
			body["tool_choice"] = "none"
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai marshal: %w", err)
	}

	return &ProviderHTTPRequest{
		URL: "https://api.openai.com/v1/chat/completions",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + o.apiKey,
		},
		Body: jsonBody,
	}, nil
}

func (o *OpenAIProvider) ParseResponse(statusCode int, body []byte) (*ProviderHTTPResponse, error) {
	if statusCode != 200 {
		return nil, fmt.Errorf("openai %d: %s", statusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("openai parse: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices returned")
	}

	choice := result.Choices[0]
	var toolCalls []ToolCall

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]interface{}
		// Arguments can be a JSON string or already parsed — handle both
		if len(tc.Function.Arguments) > 0 {
			json.Unmarshal(tc.Function.Arguments, &args)
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return &ProviderHTTPResponse{
		Content:   choice.Message.Content,
		ToolCalls: toolCalls,
		Done:      choice.FinishReason != "tool_calls",
		TokensIn:  result.Usage.PromptTokens,
		TokensOut: result.Usage.CompletionTokens,
	}, nil
}

func (o *OpenAIProvider) Embed(ctx context.Context, model string, text string) ([]float32, int, error) {
	if model == "" || model == "gpt-4o-mini" {
		model = "text-embedding-3-small"
	}

	body := map[string]interface{}{
		"model":      model,
		"input":      text,
		"dimensions": 1024,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("openai embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("openai embed %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, 0, fmt.Errorf("openai embed parse: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, 0, fmt.Errorf("openai: no embedding returned")
	}

	return result.Data[0].Embedding, result.Usage.PromptTokens, nil
}
