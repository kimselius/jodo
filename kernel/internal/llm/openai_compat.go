package llm

import (
	"encoding/json"
	"fmt"
)

// openaiCompatOpts controls the minor differences between OpenAI and Ollama
// when building the shared message/tool format.
type openaiCompatOpts struct {
	ArgsAsJSON        bool // true for OpenAI (JSON string), false for Ollama (object)
	IncludeToolCallID bool // true for OpenAI, false for Ollama
}

// buildOpenAICompatMessages transforms JodoMessages into the OpenAI-compatible
// message format used by both OpenAI and Ollama.
func buildOpenAICompatMessages(req *JodoRequest, opts openaiCompatOpts) []interface{} {
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
					fn := map[string]interface{}{
						"name": tc.Name,
					}
					if opts.ArgsAsJSON {
						argsJSON, _ := json.Marshal(tc.Arguments)
						fn["arguments"] = string(argsJSON)
					} else {
						fn["arguments"] = tc.Arguments
					}
					entry := map[string]interface{}{
						"function": fn,
					}
					if opts.ArgsAsJSON {
						entry["id"] = tc.ID
						entry["type"] = "function"
					}
					tcs[j] = entry
				}
				am["tool_calls"] = tcs
			}
			msgs = append(msgs, am)

		case "tool_result":
			content := m.Content
			if m.IsError {
				content = "Error: " + content
			}
			msg := map[string]interface{}{
				"role":    "tool",
				"content": content,
			}
			if opts.IncludeToolCallID {
				msg["tool_call_id"] = m.ToolCallID
			}
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

// buildOpenAICompatTools converts JodoRequest tools to the OpenAI function format
// used by both OpenAI and Ollama. Returns nil if no tools or tool_choice is "none".
func buildOpenAICompatTools(req *JodoRequest) []map[string]interface{} {
	if len(req.Tools) == 0 || req.ToolChoice == "none" {
		return nil
	}
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
	return tools
}

// parseToolCallArgs parses function call arguments that may be a JSON string
// or a JSON object (Ollama returns either form).
func parseToolCallArgs(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var args map[string]interface{}
	if raw[0] == '"' {
		// Double-encoded: JSON string containing JSON
		var jsonStr string
		json.Unmarshal(raw, &jsonStr)
		json.Unmarshal([]byte(jsonStr), &args)
	} else {
		json.Unmarshal(raw, &args)
	}
	return args
}

// generateToolCallID returns the existing ID or generates one from a prefix and index.
func generateToolCallID(existingID, prefix string, index int) string {
	if existingID != "" {
		return existingID
	}
	return fmt.Sprintf("%s_%d", prefix, index)
}
