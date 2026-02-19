package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatMessage struct {
	ID        int       `json:"id"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Galla     *int      `json:"galla,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// handleChatPost adds a message to the conversation.
// POST /api/chat  {"message": "hello", "source": "human"}
func (s *Server) handleChatPost(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
		Source  string `json:"source"`
		Galla   *int   `json:"galla,omitempty"`
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

	var id int
	err := s.DB.QueryRow(
		`INSERT INTO chat_messages (source, message, galla) VALUES ($1, $2, $3) RETURNING id`,
		req.Source, req.Message, req.Galla,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// handleChatGet retrieves messages.
// GET /api/chat?last=10&source=human&since_id=42
func (s *Server) handleChatGet(c *gin.Context) {
	query := `SELECT id, source, message, galla, created_at FROM chat_messages WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if source := c.Query("source"); source != "" {
		query += ` AND source = $` + strconv.Itoa(argIdx)
		args = append(args, source)
		argIdx++
	}

	if sinceID := c.Query("since_id"); sinceID != "" {
		query += ` AND id > $` + strconv.Itoa(argIdx)
		args = append(args, sinceID)
		argIdx++
	}

	query += ` ORDER BY id ASC`

	if last := c.Query("last"); last != "" {
		n, err := strconv.Atoi(last)
		if err == nil && n > 0 {
			// Wrap in subquery to get last N in chronological order
			query = `SELECT * FROM (` + query + ` ) sub ORDER BY id DESC LIMIT ` + strconv.Itoa(n)
			query = `SELECT * FROM (` + query + `) sub2 ORDER BY id ASC`
		}
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	messages := []ChatMessage{}
	for rows.Next() {
		var m ChatMessage
		var galla sql.NullInt32
		if err := rows.Scan(&m.ID, &m.Source, &m.Message, &galla, &m.CreatedAt); err != nil {
			continue
		}
		if galla.Valid {
			g := int(galla.Int32)
			m.Galla = &g
		}
		messages = append(messages, m)
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}
