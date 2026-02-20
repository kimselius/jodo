package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type llmCallSummary struct {
	ID         int       `json:"id"`
	Intent     string    `json:"intent"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	Cost       float64   `json:"cost"`
	DurationMs int       `json:"duration_ms"`
	ChainID    *string   `json:"chain_id,omitempty"`
	Error      *string   `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type llmCallDetail struct {
	llmCallSummary
	RequestSystem     *string         `json:"request_system,omitempty"`
	RequestMessages   json.RawMessage `json:"request_messages"`
	RequestTools      json.RawMessage `json:"request_tools,omitempty"`
	ResponseContent   *string         `json:"response_content,omitempty"`
	ResponseToolCalls json.RawMessage `json:"response_tool_calls,omitempty"`
	ResponseDone      bool            `json:"response_done"`
}

// GET /api/llm-calls?limit=50&offset=0&intent=chat
func (s *Server) handleLLMCallsList(c *gin.Context) {
	limit := 50
	offset := 0

	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	intent := c.Query("intent")

	var rows *sql.Rows
	var err error
	if intent != "" {
		rows, err = s.DB.Query(
			`SELECT id, intent, provider, model, tokens_in, tokens_out, cost, duration_ms, chain_id, error, created_at
			 FROM llm_calls WHERE intent = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			intent, limit, offset,
		)
	} else {
		rows, err = s.DB.Query(
			`SELECT id, intent, provider, model, tokens_in, tokens_out, cost, duration_ms, chain_id, error, created_at
			 FROM llm_calls ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	calls := []llmCallSummary{}
	for rows.Next() {
		var call llmCallSummary
		if err := rows.Scan(&call.ID, &call.Intent, &call.Provider, &call.Model,
			&call.TokensIn, &call.TokensOut, &call.Cost, &call.DurationMs,
			&call.ChainID, &call.Error, &call.CreatedAt); err != nil {
			continue
		}
		calls = append(calls, call)
	}

	var total int
	if intent != "" {
		s.DB.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE intent = $1`, intent).Scan(&total)
	} else {
		s.DB.QueryRow(`SELECT COUNT(*) FROM llm_calls`).Scan(&total)
	}

	c.JSON(http.StatusOK, gin.H{"calls": calls, "total": total})
}

// GET /api/llm-calls/:id
func (s *Server) handleLLMCallDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var call llmCallDetail
	err = s.DB.QueryRow(
		`SELECT id, intent, provider, model, tokens_in, tokens_out, cost, duration_ms,
		        chain_id, request_system, request_messages, request_tools,
		        response_content, response_tool_calls, COALESCE(response_done, false), error, created_at
		 FROM llm_calls WHERE id = $1`, id,
	).Scan(
		&call.ID, &call.Intent, &call.Provider, &call.Model,
		&call.TokensIn, &call.TokensOut, &call.Cost, &call.DurationMs,
		&call.ChainID, &call.RequestSystem, &call.RequestMessages, &call.RequestTools,
		&call.ResponseContent, &call.ResponseToolCalls, &call.ResponseDone,
		&call.Error, &call.CreatedAt,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, call)
}
