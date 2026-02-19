package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/config"
)

// GET /api/settings/providers
func (s *Server) handleSettingsProvidersGet(c *gin.Context) {
	providers, err := s.ConfigStore.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// PUT /api/settings/providers/:name
func (s *Server) handleSettingsProviderPut(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		Enabled          *bool    `json:"enabled"`
		APIKey           string   `json:"api_key"`
		BaseURL          *string  `json:"base_url"`
		MonthlyBudget    *float64 `json:"monthly_budget"`
		EmergencyReserve *float64 `json:"emergency_reserve"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Load current provider to merge partial updates
	providers, err := s.ConfigStore.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var current *config.ProviderInfo
	for _, p := range providers {
		if p.Name == name {
			current = &p
			break
		}
	}
	if current == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	baseURL := current.BaseURL
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}
	budget := current.MonthlyBudget
	if req.MonthlyBudget != nil {
		budget = *req.MonthlyBudget
	}
	reserve := current.EmergencyReserve
	if req.EmergencyReserve != nil {
		reserve = *req.EmergencyReserve
	}

	if err := s.ConfigStore.SaveProvider(name, enabled, req.APIKey, baseURL, budget, reserve); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Hot-reload the LLM proxy with new config
	s.reloadProxy()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/settings/providers/:name/models
func (s *Server) handleSettingsModelAdd(c *gin.Context) {
	name := c.Param("name")

	var req modelSetupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := s.ConfigStore.SaveModel(name, req.ModelKey, req.ModelName, req.InputCostPer1M, req.OutputCostPer1M, req.Capabilities, req.Quality); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.reloadProxy()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/settings/providers/:name/models/:key
func (s *Server) handleSettingsModelDelete(c *gin.Context) {
	name := c.Param("name")
	key := c.Param("key")

	if err := s.ConfigStore.DeleteModel(name, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.reloadProxy()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/settings/genesis
func (s *Server) handleSettingsGenesisGet(c *gin.Context) {
	genesis, err := s.ConfigStore.LoadGenesis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, genesis)
}

// PUT /api/settings/genesis
func (s *Server) handleSettingsGenesisPut(c *gin.Context) {
	var req genesisSetupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	genesis := req.toGenesis()
	if err := s.ConfigStore.SaveGenesis(genesis); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update in-memory genesis
	s.Genesis = genesis

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/settings/routing
func (s *Server) handleSettingsRoutingGet(c *gin.Context) {
	rc := s.ConfigStore.GetRoutingConfig()
	c.JSON(http.StatusOK, rc)
}

// PUT /api/settings/routing
func (s *Server) handleSettingsRoutingPut(c *gin.Context) {
	var req config.RoutingConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := s.ConfigStore.SaveRoutingConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.reloadProxy()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/settings/kernel
func (s *Server) handleSettingsKernelGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"health_check_interval": s.ConfigStore.GetConfigInt("kernel.health_check_interval", 10),
		"max_restart_attempts":  s.ConfigStore.GetConfigInt("kernel.max_restart_attempts", 3),
		"log_level":             s.ConfigStore.GetConfig("kernel.log_level"),
		"audit_log_path":        s.ConfigStore.GetConfig("kernel.audit_log_path"),
		"external_url":          s.ConfigStore.GetConfig("kernel.external_url"),
	})
}

// PUT /api/settings/kernel
func (s *Server) handleSettingsKernelPut(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	for key, val := range req {
		dbKey := "kernel." + key
		s.ConfigStore.SetConfig(dbKey, fmt.Sprintf("%v", val))
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "note": "Some changes may require a kernel restart"})
}

// GET /api/settings/ssh
func (s *Server) handleSettingsSSHGet(c *gin.Context) {
	host := s.ConfigStore.GetConfig("jodo.host")
	user := s.ConfigStore.GetConfig("jodo.ssh_user")

	_, err := s.ConfigStore.GetSecret("ssh_private_key")
	hasKey := err == nil

	c.JSON(http.StatusOK, gin.H{
		"host":    host,
		"user":    user,
		"has_key": hasKey,
	})
}

// reloadProxy reloads the LLM proxy with updated config from DB.
func (s *Server) reloadProxy() {
	if s.LLM == nil || s.Config == nil {
		return
	}

	dbCfg := s.Config.Database
	cfg, err := s.ConfigStore.LoadFullConfig(dbCfg)
	if err != nil {
		return
	}

	s.LLM.Reconfigure(cfg.Providers, cfg.Routing)
	s.Config.Providers = cfg.Providers
	s.Config.Routing = cfg.Routing

	// Also update routing intent preferences
	ipJSON := s.ConfigStore.GetConfig("routing.intent_preferences")
	if ipJSON != "" {
		json.Unmarshal([]byte(ipJSON), &s.Config.Routing.IntentPreferences)
	}
}
