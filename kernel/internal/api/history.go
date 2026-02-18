package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleHistory(c *gin.Context) {
	commits, err := s.Git.Log(30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"commits": commits,
	})
}
