package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleGenesis(c *gin.Context) {
	c.JSON(http.StatusOK, s.Genesis)
}
