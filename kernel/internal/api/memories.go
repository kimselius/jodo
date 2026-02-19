package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleMemoriesList(c *gin.Context) {
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

	entries, err := s.Memory.List(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	total, _ := s.Memory.Count()

	c.JSON(http.StatusOK, gin.H{
		"memories": entries,
		"total":    total,
	})
}
