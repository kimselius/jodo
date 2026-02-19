package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatMessage struct {
	ID        int        `json:"id"`
	Source    string     `json:"source"`
	Message   string     `json:"message"`
	Galla     *int       `json:"galla,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ChatHub manages SSE subscribers for real-time chat updates.
type ChatHub struct {
	mu      sync.RWMutex
	clients map[chan ChatMessage]struct{}
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		clients: make(map[chan ChatMessage]struct{}),
	}
}

func (h *ChatHub) Subscribe() chan ChatMessage {
	ch := make(chan ChatMessage, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *ChatHub) Unsubscribe(ch chan ChatMessage) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *ChatHub) Broadcast(msg ChatMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// Slow client, drop message
		}
	}
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

	var msg ChatMessage
	err := s.DB.QueryRow(
		`INSERT INTO chat_messages (source, message, galla) VALUES ($1, $2, $3) RETURNING id, created_at`,
		req.Source, req.Message, req.Galla,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store message"})
		return
	}

	msg.Source = req.Source
	msg.Message = req.Message
	msg.Galla = req.Galla

	// Broadcast to SSE subscribers
	if s.ChatHub != nil {
		s.ChatHub.Broadcast(msg)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": msg.ID})
}

// handleChatGet retrieves messages.
// GET /api/chat?last=10&source=human&since_id=42&unread=true
func (s *Server) handleChatGet(c *gin.Context) {
	query := `SELECT id, source, message, galla, read_at, created_at FROM chat_messages WHERE 1=1`
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

	if c.Query("unread") == "true" {
		query += ` AND read_at IS NULL`
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
		var readAt sql.NullTime
		if err := rows.Scan(&m.ID, &m.Source, &m.Message, &galla, &readAt, &m.CreatedAt); err != nil {
			continue
		}
		if galla.Valid {
			g := int(galla.Int32)
			m.Galla = &g
		}
		if readAt.Valid {
			m.ReadAt = &readAt.Time
		}
		messages = append(messages, m)
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// handleChatAck marks messages as read up to a given ID.
// POST /api/chat/ack  {"up_to_id": 42}
func (s *Server) handleChatAck(c *gin.Context) {
	var req struct {
		UpToID int `json:"up_to_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UpToID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "up_to_id required (positive integer)"})
		return
	}

	result, err := s.DB.Exec(
		`UPDATE chat_messages SET read_at = NOW() WHERE id <= $1 AND read_at IS NULL`,
		req.UpToID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark as read"})
		return
	}

	count, _ := result.RowsAffected()
	c.JSON(http.StatusOK, gin.H{"ok": true, "marked": count})
}

// handleChatStream is an SSE endpoint. The browser opens this once and
// receives every new message in real time — no polling needed.
// GET /api/chat/stream
func (s *Server) handleChatStream(c *gin.Context) {
	if s.ChatHub == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "chat hub not initialized"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ch := s.ChatHub.Subscribe()
	defer s.ChatHub.Unsubscribe(ch)

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
