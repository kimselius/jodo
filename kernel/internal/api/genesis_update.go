package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var genesisWriteMu sync.Mutex

type identityUpdateRequest struct {
	Name    *string `json:"name,omitempty"`
	Purpose *string `json:"purpose,omitempty"`
}

func (s *Server) handleGenesisIdentityGet(c *gin.Context) {
	if s.ConfigStore != nil {
		genesis, err := s.ConfigStore.LoadGenesis()
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"name":    genesis.Identity.Name,
				"version": genesis.Identity.Version,
				"purpose": genesis.Purpose,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    s.Genesis.Identity.Name,
		"version": s.Genesis.Identity.Version,
		"purpose": s.Genesis.Purpose,
	})
}

func (s *Server) handleGenesisIdentityPut(c *gin.Context) {
	var req identityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == nil && req.Purpose == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	genesisWriteMu.Lock()
	defer genesisWriteMu.Unlock()

	// Write to DB
	if s.ConfigStore != nil {
		genesis, err := s.ConfigStore.LoadGenesis()
		if err == nil {
			if req.Name != nil && *req.Name != "" {
				genesis.Identity.Name = *req.Name
			}
			if req.Purpose != nil {
				genesis.Purpose = *req.Purpose
			}

			if err := s.ConfigStore.SaveGenesis(genesis); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "save genesis failed"})
				return
			}

			s.Genesis = genesis
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
	}

	// Fallback: update in-memory only
	if req.Name != nil && *req.Name != "" {
		s.Genesis.Identity.Name = *req.Name
	}
	if req.Purpose != nil {
		s.Genesis.Purpose = *req.Purpose
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
