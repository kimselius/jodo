package api

import (
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

var genesisWriteMu sync.Mutex

type identityUpdateRequest struct {
	Name    *string `json:"name,omitempty"`
	Purpose *string `json:"purpose,omitempty"`
}

func (s *Server) handleGenesisIdentityGet(c *gin.Context) {
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

	// Read current file
	data, err := os.ReadFile(s.GenesisPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read genesis failed"})
		return
	}

	// Parse into a map to preserve all fields
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "parse genesis failed"})
		return
	}

	// Apply updates
	identity, _ := raw["identity"].(map[string]interface{})
	if identity == nil {
		identity = make(map[string]interface{})
	}

	if req.Name != nil && *req.Name != "" {
		identity["name"] = *req.Name
		s.Genesis.Identity.Name = *req.Name
	}
	if req.Purpose != nil {
		raw["purpose"] = *req.Purpose
		s.Genesis.Purpose = *req.Purpose
	}
	raw["identity"] = identity

	// Write back
	out, err := yaml.Marshal(raw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal genesis failed"})
		return
	}
	if err := os.WriteFile(s.GenesisPath, out, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write genesis failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
