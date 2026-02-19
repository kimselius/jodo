package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleGrowthLog(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	milestones, err := s.Growth.Recent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": milestones,
	})
}
