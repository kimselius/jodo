package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type gallaEntry struct {
	ID           int        `json:"id"`
	Galla        int        `json:"galla"`
	Plan         *string    `json:"plan"`
	Summary      *string    `json:"summary"`
	ActionsCount int        `json:"actions_count"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

func (s *Server) handleGallaPost(c *gin.Context) {
	var req struct {
		Galla        int     `json:"galla"`
		Plan         *string `json:"plan"`
		Summary      *string `json:"summary"`
		ActionsCount *int    `json:"actions_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Plan != nil {
		// Planning phase — insert or update plan
		_, err := s.DB.Exec(`
			INSERT INTO galla_log (galla, plan)
			VALUES ($1, $2)
			ON CONFLICT (galla) DO UPDATE SET plan = $2
		`, req.Galla, *req.Plan)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if req.Summary != nil {
		// Execution phase — update summary and mark complete
		actionsCount := 0
		if req.ActionsCount != nil {
			actionsCount = *req.ActionsCount
		}
		_, err := s.DB.Exec(`
			INSERT INTO galla_log (galla, summary, actions_count, completed_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (galla) DO UPDATE SET
				summary = $2,
				actions_count = $3,
				completed_at = NOW()
		`, req.Galla, *req.Summary, actionsCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Broadcast to connected frontends so Growth page auto-updates
	s.WS.Broadcast("growth", gin.H{"galla": req.Galla})

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleGallaGet(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	rows, err := s.DB.Query(`
		SELECT id, galla, plan, summary, actions_count, started_at, completed_at
		FROM galla_log
		ORDER BY galla DESC
		LIMIT $1
	`, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	entries := []gallaEntry{}
	for rows.Next() {
		var e gallaEntry
		if err := rows.Scan(&e.ID, &e.Galla, &e.Plan, &e.Summary, &e.ActionsCount, &e.StartedAt, &e.CompletedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		entries = append(entries, e)
	}

	c.JSON(http.StatusOK, gin.H{"gallas": entries})
}
