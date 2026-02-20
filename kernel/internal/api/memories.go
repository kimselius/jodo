package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/memory"
)

// GET /api/memories/search?q=...&limit=10&tags=foo,bar
func (s *Server) handleMemoriesSearchGet(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q parameter required"})
		return
	}

	limit := 10
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	var tags []string
	if t := c.Query("tags"); t != "" {
		tags = strings.Split(t, ",")
	}

	resp, err := s.Searcher.Search(c.Request.Context(), &memory.SearchRequest{
		Query: query,
		Limit: limit,
		Tags:  tags,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

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
