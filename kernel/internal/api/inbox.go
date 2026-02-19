package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// InboxMessage represents a logged intercom message.
type InboxMessage struct {
	ID        int       `json:"id"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Message   string    `json:"message"`
	Galla     *int      `json:"galla,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// handleInboxList returns recent inbox messages.
// GET /api/inbox?limit=100
func (s *Server) handleInboxList(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	rows, err := s.DB.Query(
		`SELECT id, source, target, message, galla, created_at FROM inbox_messages ORDER BY created_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	messages := []InboxMessage{}
	for rows.Next() {
		var m InboxMessage
		if err := rows.Scan(&m.ID, &m.Source, &m.Target, &m.Message, &m.Galla, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	// Reverse to ascending order for display
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// handleInboxPost logs an inbox message.
// POST /api/inbox  {"source": "kernel", "target": "jodo", "message": "...", "galla": 5}
func (s *Server) handleInboxPost(c *gin.Context) {
	var req struct {
		Source  string `json:"source"`
		Target  string `json:"target"`
		Message string `json:"message"`
		Galla   *int   `json:"galla"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message required"})
		return
	}
	if req.Source == "" {
		req.Source = "unknown"
	}
	if req.Target == "" {
		req.Target = "jodo"
	}

	var msg InboxMessage
	err := s.DB.QueryRow(
		`INSERT INTO inbox_messages (source, target, message, galla) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		req.Source, req.Target, req.Message, req.Galla,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	msg.Source = req.Source
	msg.Target = req.Target
	msg.Message = req.Message
	msg.Galla = req.Galla

	if s.WS != nil {
		s.WS.Broadcast("inbox", gin.H{
			"id":      msg.ID,
			"source":  msg.Source,
			"target":  msg.Target,
			"message": msg.Message,
		})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
