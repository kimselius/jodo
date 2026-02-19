package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleGenesis(c *gin.Context) {
	// Try loading from DB first, fall back to in-memory
	if s.ConfigStore != nil {
		genesis, err := s.ConfigStore.LoadGenesis()
		if err == nil {
			c.JSON(http.StatusOK, genesis)
			return
		}
	}
	c.JSON(http.StatusOK, s.Genesis)
}
