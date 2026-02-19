package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// LibraryItem represents a human-created brief for Jodo.
type LibraryItem struct {
	ID        int              `json:"id"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	Status    string           `json:"status"`
	Priority  int              `json:"priority"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Comments  []LibraryComment `json:"comments"`
}

// LibraryComment is a comment on a library item from the human or Jodo.
type LibraryComment struct {
	ID        int       `json:"id"`
	ItemID    int       `json:"item_id"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// handleLibraryList returns all library items with their comments.
// GET /api/library?status=new
func (s *Server) handleLibraryList(c *gin.Context) {
	query := `SELECT id, title, content, status, priority, created_at, updated_at FROM library_items WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status := c.Query("status"); status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY priority DESC, created_at DESC`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	items := []LibraryItem{}
	for rows.Next() {
		var item LibraryItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &item.Status, &item.Priority, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		item.Comments = []LibraryComment{} // initialize empty
		items = append(items, item)
	}

	// Load comments for all items
	if len(items) > 0 {
		ids := make([]int, len(items))
		idMap := make(map[int]int) // item.ID → index in items
		for i, item := range items {
			ids[i] = item.ID
			idMap[item.ID] = i
		}

		commentRows, err := s.DB.Query(
			`SELECT id, item_id, source, message, created_at FROM library_comments
			 WHERE item_id = ANY($1) ORDER BY created_at ASC`,
			pq.Array(ids),
		)
		if err == nil {
			defer commentRows.Close()
			for commentRows.Next() {
				var c LibraryComment
				if err := commentRows.Scan(&c.ID, &c.ItemID, &c.Source, &c.Message, &c.CreatedAt); err != nil {
					continue
				}
				if idx, ok := idMap[c.ItemID]; ok {
					items[idx].Comments = append(items[idx].Comments, c)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// handleLibraryCreate creates a new library item.
// POST /api/library  {"title": "...", "content": "...", "priority": 0}
func (s *Server) handleLibraryCreate(c *gin.Context) {
	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Priority int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title required"})
		return
	}

	var item LibraryItem
	err := s.DB.QueryRow(
		`INSERT INTO library_items (title, content, priority) VALUES ($1, $2, $3) RETURNING id, status, created_at, updated_at`,
		req.Title, req.Content, req.Priority,
	).Scan(&item.ID, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}

	item.Title = req.Title
	item.Content = req.Content
	item.Priority = req.Priority
	item.Comments = []LibraryComment{}

	if s.WS != nil {
		s.WS.Broadcast("library", gin.H{"action": "created", "item_id": item.ID})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "item": item})
}

// handleLibraryGet returns a single library item with comments.
// GET /api/library/:id
func (s *Server) handleLibraryGet(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var item LibraryItem
	err = s.DB.QueryRow(
		`SELECT id, title, content, status, priority, created_at, updated_at FROM library_items WHERE id = $1`,
		id,
	).Scan(&item.ID, &item.Title, &item.Content, &item.Status, &item.Priority, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	// Load comments
	item.Comments = []LibraryComment{}
	rows, err := s.DB.Query(
		`SELECT id, item_id, source, message, created_at FROM library_comments WHERE item_id = $1 ORDER BY created_at ASC`,
		id,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var comment LibraryComment
			if err := rows.Scan(&comment.ID, &comment.ItemID, &comment.Source, &comment.Message, &comment.CreatedAt); err != nil {
				continue
			}
			item.Comments = append(item.Comments, comment)
		}
	}

	c.JSON(http.StatusOK, gin.H{"item": item})
}

// handleLibraryUpdate updates a library item's content (human edits).
// PUT /api/library/:id  {"title": "...", "content": "...", "priority": 0}
func (s *Server) handleLibraryUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Title    *string `json:"title"`
		Content  *string `json:"content"`
		Priority *int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Build dynamic update
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if req.Title != nil {
		setClauses = append(setClauses, "title = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Content != nil {
		setClauses = append(setClauses, "content = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Content)
		argIdx++
	}
	if req.Priority != nil {
		setClauses = append(setClauses, "priority = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Priority)
		argIdx++
	}

	args = append(args, id)
	query := "UPDATE library_items SET " + joinStrings(setClauses, ", ") + " WHERE id = $" + strconv.Itoa(argIdx)

	result, err := s.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if s.WS != nil {
		s.WS.Broadcast("library", gin.H{"action": "updated", "item_id": id})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleLibraryPatch updates a library item's status (Jodo updates).
// PATCH /api/library/:id  {"status": "in_progress"}
func (s *Server) handleLibraryPatch(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status required"})
		return
	}

	// Validate status
	validStatuses := map[string]bool{"new": true, "in_progress": true, "done": true, "blocked": true, "archived": true}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	result, err := s.DB.Exec(
		`UPDATE library_items SET status = $1, updated_at = NOW() WHERE id = $2`,
		req.Status, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if s.WS != nil {
		s.WS.Broadcast("library", gin.H{"action": "status_changed", "item_id": id, "status": req.Status})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleLibraryDelete deletes a library item and its comments.
// DELETE /api/library/:id
func (s *Server) handleLibraryDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	result, err := s.DB.Exec(`DELETE FROM library_items WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if s.WS != nil {
		s.WS.Broadcast("library", gin.H{"action": "deleted", "item_id": id})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleLibraryCommentPost adds a comment to a library item.
// POST /api/library/:id/comments  {"source": "human", "message": "..."}
func (s *Server) handleLibraryCommentPost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Source  string `json:"source"`
		Message string `json:"message"`
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

	// Verify item exists
	var exists bool
	s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM library_items WHERE id = $1)`, id).Scan(&exists)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	var comment LibraryComment
	err = s.DB.QueryRow(
		`INSERT INTO library_comments (item_id, source, message) VALUES ($1, $2, $3) RETURNING id, created_at`,
		id, req.Source, req.Message,
	).Scan(&comment.ID, &comment.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "comment failed"})
		return
	}

	comment.ItemID = id
	comment.Source = req.Source
	comment.Message = req.Message

	// Update item's updated_at
	s.DB.Exec(`UPDATE library_items SET updated_at = NOW() WHERE id = $1`, id)

	if s.WS != nil {
		s.WS.Broadcast("library", gin.H{"action": "comment_added", "item_id": id, "comment": comment})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "comment": comment})
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
