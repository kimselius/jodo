package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/memory"
)

func (s *Server) handleMemoryStore(c *gin.Context) {
	var req memory.StoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}

	resp, err := s.Memory.Store(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast to WebSocket clients
	if s.WS != nil {
		s.WS.Broadcast("memory", gin.H{
			"content": req.Content,
			"tags":    req.Tags,
			"source":  req.Source,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleMemorySearch(c *gin.Context) {
	var req memory.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}

	resp, err := s.Searcher.Search(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
