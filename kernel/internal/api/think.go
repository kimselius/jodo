package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/llm"
)

func (s *Server) handleThink(c *gin.Context) {
	var req llm.JodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages required"})
		return
	}

	resp, err := s.LLM.Think(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
