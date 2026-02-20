package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"jodo-kernel/internal/config"
)

// ClaudeProvider implements the Provider interface for Anthropic's Claude API.
type ClaudeProvider struct {
	apiKey string
}

func NewClaudeProvider(cfg config.ProviderConfig) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey: cfg.APIKey,
	}
}

func (c *ClaudeProvider) Name() string       { return "claude" }
func (c *ClaudeProvider) SupportsEmbed() bool { return false }

func (c *ClaudeProvider) BuildRequest(req *JodoRequest, model string) (*ProviderHTTPRequest, error) {
	body := map[string]interface{}{
		"model":      model,
		"max_tokens": req.MaxTokens,
	}
	if req.System != "" {
		body["system"] = req.System
	}

	// Transform messages: Jodo Format → Claude format
	body["messages"] = claudeTransformMessages(req.Messages)

	// Transform tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]interface{}{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			}
		}

		switch req.ToolChoice {
		case "none":
			// Don't send tools at all
		default:
			body["tools"] = tools
		}

		switch req.ToolChoice {
		case "auto", "":
			body["tool_choice"] = map[string]string{"type": "auto"}
		case "required":
			body["tool_choice"] = map[string]string{"type": "any"}
		case "none":
			// tools already omitted above
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("claude marshal: %w", err)
	}

	return &ProviderHTTPRequest{
		URL: "https://api.anthropic.com/v1/messages",
		Headers: map[string]string{
			"Content-Type":     "application/json",
			"x-api-key":        c.apiKey,
			"anthropic-version": "2023-06-01", // required by Anthropic API
		},
		Body: jsonBody,
	}, nil
}

func (c *ClaudeProvider) ParseResponse(statusCode int, body []byte) (*ProviderHTTPResponse, error) {
	if statusCode != 200 {
		return nil, fmt.Errorf("claude %d: %s", statusCode, string(body))
	}

	var result struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("claude parse: %w", err)
	}

	var content string
	var toolCalls []ToolCall

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			var args map[string]interface{}
			if len(block.Input) > 0 {
				json.Unmarshal(block.Input, &args)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return &ProviderHTTPResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Done:      result.StopReason != "tool_use",
		TokensIn:  result.Usage.InputTokens,
		TokensOut: result.Usage.OutputTokens,
	}, nil
}

func (c *ClaudeProvider) Embed(_ context.Context, _ string, _ string) ([]float32, int, error) {
	return nil, 0, fmt.Errorf("claude does not support embeddings")
}

// claudeTransformMessages converts Jodo messages to Claude's message format.
// Key differences:
//   - Assistant messages with tool_calls → content becomes array of blocks
//   - Consecutive tool_result messages → grouped into a single user message with tool_result blocks
func claudeTransformMessages(messages []JodoMessage) []interface{} {
	var result []interface{}

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		switch msg.Role {
		case "user":
			result = append(result, map[string]interface{}{
				"role":    "user",
				"content": msg.Content,
			})

		case "assistant":
			if len(msg.ToolCalls) == 0 {
				// Plain text response
				result = append(result, map[string]interface{}{
					"role":    "assistant",
					"content": msg.Content,
				})
			} else {
				// Content as array of blocks: text block(s) + tool_use blocks
				var blocks []interface{}
				if msg.Content != "" {
					blocks = append(blocks, map[string]interface{}{
						"type": "text",
						"text": msg.Content,
					})
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, map[string]interface{}{
						"type":  "tool_use",
						"id":    tc.ID,
						"name":  tc.Name,
						"input": tc.Arguments,
					})
				}
				result = append(result, map[string]interface{}{
					"role":    "assistant",
					"content": blocks,
				})
			}

		case "tool_result":
			// Collect consecutive tool_result messages into one user message
			var toolResults []interface{}
			for ; i < len(messages) && messages[i].Role == "tool_result"; i++ {
				tr := messages[i]
				block := map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": tr.ToolCallID,
					"content":     tr.Content,
				}
				if tr.IsError {
					block["is_error"] = true
				}
				toolResults = append(toolResults, block)
			}
			i-- // compensate for outer loop increment

			result = append(result, map[string]interface{}{
				"role":    "user",
				"content": toolResults,
			})
		}
	}

	return result
}
