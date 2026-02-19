package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleHeartbeat(c *gin.Context) {
	var req struct {
		Phase string `json:"phase"`
		Galla int    `json:"galla"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	s.Process.SetHeartbeat(req.Galla, req.Phase)

	// A heartbeat from seed.py means Jodo is alive — set status to running
	s.Process.SetStatus("running")

	// Broadcast to WebSocket clients
	if s.WS != nil {
		s.WS.Broadcast("heartbeat", gin.H{
			"phase": req.Phase,
			"galla": req.Galla,
		})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
