package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Entry is a single audit log line.
type Entry struct {
	Timestamp string      `json:"ts"`
	Event     string      `json:"event"`            // "think_request", "think_response", "think_error", "memory_store", etc.
	Intent    string      `json:"intent,omitempty"`
	Provider  string      `json:"provider,omitempty"`
	Model     string      `json:"model,omitempty"`
	TokensIn  int         `json:"tokens_in,omitempty"`
	TokensOut int         `json:"tokens_out,omitempty"`
	Cost      float64     `json:"cost,omitempty"`
	Duration  string      `json:"duration,omitempty"`
	Data      interface{} `json:"data,omitempty"` // request/response payload
	Error     string      `json:"error,omitempty"`
}

// Logger writes JSONL audit entries to a file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewLogger creates an audit logger writing to the given path.
// Creates the file if it doesn't exist, appends if it does.
func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}

	l := &Logger{
		file: f,
		enc:  json.NewEncoder(f),
	}

	l.Log(Entry{
		Event: "audit_start",
		Data:  map[string]string{"path": path},
	})

	log.Printf("[audit] logging to %s", path)
	return l, nil
}

// Log writes an entry to the audit log.
func (l *Logger) Log(e Entry) {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.enc.Encode(e); err != nil {
		log.Printf("[audit] write error: %v", err)
	}
}

// Close flushes and closes the audit log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// ThinkRequest is the data logged for an incoming think request.
type ThinkRequest struct {
	Intent     string      `json:"intent"`
	System     string      `json:"system,omitempty"`
	Messages   interface{} `json:"messages"`
	Tools      interface{} `json:"tools,omitempty"`
	ToolChoice string      `json:"tool_choice,omitempty"`
	MaxTokens  int         `json:"max_tokens"`
	ChainID    string      `json:"chain_id,omitempty"`
	MaxCost    float64     `json:"max_cost,omitempty"`
}

// ThinkResponse is the data logged for a think response.
type ThinkResponse struct {
	Content   string      `json:"content"`
	ToolCalls interface{} `json:"tool_calls,omitempty"`
	Done      bool        `json:"done"`
}
