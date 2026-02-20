package api

import (
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
		TotalVRAMBytes   *int64   `json:"total_vram_bytes"`
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
	vram := current.TotalVRAMBytes
	if req.TotalVRAMBytes != nil {
		vram = *req.TotalVRAMBytes
	}

	if err := s.ConfigStore.SaveProvider(name, enabled, req.APIKey, baseURL, budget, reserve, vram); err != nil {
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

	if err := s.ConfigStore.SaveModel(name, req.ModelKey, req.ModelName, req.InputCostPer1M, req.OutputCostPer1M, req.Capabilities, req.Quality, req.VRAMEstimateBytes, req.SupportsTools, req.PreferLoaded); err != nil {
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

	brainPath := s.ConfigStore.GetConfig("jodo.brain_path")
	if brainPath == "" {
		brainPath = "/opt/jodo/brain"
	}

	c.JSON(http.StatusOK, gin.H{
		"host":       host,
		"user":       user,
		"has_key":    hasKey,
		"brain_path": brainPath,
		"jodo_mode":  s.JodoMode,
	})
}

// GET /api/settings/subagent
func (s *Server) handleSettingsSubagentGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"max_concurrent": s.ConfigStore.GetConfigInt("jodo.max_subagents", 3),
		"max_timeout":    s.ConfigStore.GetConfigInt("jodo.subagent_timeout", 300),
	})
}

// PUT /api/settings/subagent
func (s *Server) handleSettingsSubagentPut(c *gin.Context) {
	var req struct {
		MaxConcurrent *int `json:"max_concurrent"`
		MaxTimeout    *int `json:"max_timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.MaxConcurrent != nil {
		v := *req.MaxConcurrent
		if v < 1 {
			v = 1
		}
		if v > 10 {
			v = 10
		}
		s.ConfigStore.SetConfig("jodo.max_subagents", fmt.Sprintf("%d", v))
	}
	if req.MaxTimeout != nil {
		v := *req.MaxTimeout
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		s.ConfigStore.SetConfig("jodo.subagent_timeout", fmt.Sprintf("%d", v))
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "note": "Changes apply on next seed restart/rebirth"})
}

// GET /api/settings/vram — returns VRAM tracker status
func (s *Server) handleSettingsVRAMStatus(c *gin.Context) {
	if s.LLM == nil || s.LLM.VRAM == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":          false,
			"total_vram_bytes": 0,
		})
		return
	}
	status := s.LLM.VRAM.GetStatus()
	status["enabled"] = true
	c.JSON(http.StatusOK, status)
}

// PUT /api/settings/providers/:name/models/:key
func (s *Server) handleSettingsModelUpdate(c *gin.Context) {
	name := c.Param("name")
	key := c.Param("key")

	var req struct {
		ModelName         *string  `json:"model_name"`
		InputCostPer1M    *float64 `json:"input_cost_per_1m"`
		OutputCostPer1M   *float64 `json:"output_cost_per_1m"`
		Capabilities      []string `json:"capabilities"`
		Quality           *int     `json:"quality"`
		Enabled           *bool    `json:"enabled"`
		VRAMEstimateBytes *int64   `json:"vram_estimate_bytes"`
		SupportsTools     *bool    `json:"supports_tools"`
		PreferLoaded      *bool    `json:"prefer_loaded"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// If just toggling enabled status
	if req.Enabled != nil {
		if err := s.ConfigStore.SetModelEnabled(name, key, *req.Enabled); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// If updating other fields, we need to load current values and merge
	if req.ModelName != nil || req.InputCostPer1M != nil || req.OutputCostPer1M != nil || req.Capabilities != nil || req.Quality != nil || req.VRAMEstimateBytes != nil || req.SupportsTools != nil || req.PreferLoaded != nil {
		current, err := s.ConfigStore.GetModel(name, key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		mn := current.ModelName
		if req.ModelName != nil {
			mn = *req.ModelName
		}
		ic := current.InputCostPer1M
		if req.InputCostPer1M != nil {
			ic = *req.InputCostPer1M
		}
		oc := current.OutputCostPer1M
		if req.OutputCostPer1M != nil {
			oc = *req.OutputCostPer1M
		}
		caps := current.Capabilities
		if req.Capabilities != nil {
			caps = req.Capabilities
		}
		q := current.Quality
		if req.Quality != nil {
			q = *req.Quality
		}
		vramEst := current.VRAMEstimateBytes
		if req.VRAMEstimateBytes != nil {
			vramEst = *req.VRAMEstimateBytes
		}
		supTools := current.SupportsTools
		if req.SupportsTools != nil {
			supTools = req.SupportsTools
		}
		prefLoaded := current.PreferLoaded
		if req.PreferLoaded != nil {
			prefLoaded = *req.PreferLoaded
		}
		if err := s.ConfigStore.SaveModel(name, key, mn, ic, oc, caps, q, vramEst, supTools, prefLoaded); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	s.reloadProxy()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// reloadProxy reloads the LLM proxy with updated config from DB.
func (s *Server) reloadProxy() {
	if s.LLM == nil || s.Config == nil {
		return
	}

	cfg, err := s.ConfigStore.LoadFullConfig(s.Config.Database)
	if err != nil {
		return
	}

	s.LLM.Reconfigure(cfg.Providers, cfg.Routing)
	s.Config.Providers = cfg.Providers
	s.Config.Routing = cfg.Routing
}
