package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/audit"
)

func (s *Server) handleCommit(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Message == "" {
		req.Message = "auto-commit via API"
	}

	resp, err := s.Git.CommitWithMessage(req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast to WebSocket clients
	if s.WS != nil {
		s.WS.Broadcast("timeline", gin.H{"message": req.Message, "commit": resp})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleRestart(c *gin.Context) {
	prevStatus := s.Process.GetStatus()

	go s.Process.RestartJodo()

	c.JSON(http.StatusOK, gin.H{
		"status":       "restarting",
		"previous_pid": prevStatus.PID,
	})
}

func (s *Server) handleLog(c *gin.Context) {
	var req struct {
		Event   string `json:"event"`
		Message string `json:"message"`
		Galla   int    `json:"galla"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Write to kernel's standard log
	log.Printf("[jodo-remote|g%d] %s", req.Galla, req.Message)

	// Write to audit log if available
	if s.Audit != nil {
		s.Audit.Log(audit.Entry{
			Event: req.Event,
			Data: map[string]interface{}{
				"message": req.Message,
				"galla":   req.Galla,
			},
		})
	}

	// Also record in growth log
	s.Growth.Log("jodo_log", req.Message, "", map[string]interface{}{"galla": req.Galla})

	// Broadcast to connected frontends
	s.WS.Broadcast("growth", gin.H{"event": req.Event, "galla": req.Galla})

	c.JSON(http.StatusOK, gin.H{"status": "logged"})
}

func (s *Server) handleRollback(c *gin.Context) {
	var req struct {
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target required (git tag or commit hash)"})
		return
	}

	resp, err := s.Git.Rollback(req.Target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Restart after rollback
	go s.Process.RestartJodo()

	c.JSON(http.StatusOK, resp)
}
