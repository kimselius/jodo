package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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

// GET /api/settings/providers/:name/discover
func (s *Server) handleSettingsProviderDiscover(c *gin.Context) {
	name := c.Param("name")

	switch name {
	case "ollama":
		s.discoverOllamaModels(c)
	case "claude":
		c.JSON(http.StatusOK, gin.H{"models": knownClaudeModels()})
	case "openai":
		c.JSON(http.StatusOK, gin.H{"models": knownOpenAIModels()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown provider"})
	}
}

func (s *Server) discoverOllamaModels(c *gin.Context) {
	// Get base URL from DB
	providers, err := s.ConfigStore.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	baseURL := "http://host.docker.internal:11434"
	for _, p := range providers {
		if p.Name == "ollama" && p.BaseURL != "" {
			baseURL = p.BaseURL
			break
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": fmt.Sprintf("Cannot reach Ollama at %s: %v", baseURL, err)})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var ollamaResp struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Size    int64  `json:"size"`
			Details struct {
				Family          string   `json:"family"`
				Families        []string `json:"families"`
				ParameterSize   string   `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "failed to parse Ollama response"})
		return
	}

	type discoveredModel struct {
		Name            string   `json:"name"`
		Family          string   `json:"family"`
		ParameterSize   string   `json:"parameter_size"`
		Quantization    string   `json:"quantization"`
		SizeBytes       int64    `json:"size_bytes"`
		Recommended     *knownModel `json:"recommended,omitempty"`
	}

	models := make([]discoveredModel, 0, len(ollamaResp.Models))
	for _, m := range ollamaResp.Models {
		dm := discoveredModel{
			Name:          m.Name,
			Family:        m.Details.Family,
			ParameterSize: m.Details.ParameterSize,
			Quantization:  m.Details.QuantizationLevel,
			SizeBytes:     m.Size,
		}
		// Match against known Ollama model families for recommended defaults
		if rec := matchOllamaModel(m.Name, m.Details.Family); rec != nil {
			dm.Recommended = rec
		}
		models = append(models, dm)
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// PUT /api/settings/providers/:name/models/:key
func (s *Server) handleSettingsModelUpdate(c *gin.Context) {
	name := c.Param("name")
	key := c.Param("key")

	var req struct {
		ModelName      *string  `json:"model_name"`
		InputCostPer1M *float64 `json:"input_cost_per_1m"`
		OutputCostPer1M *float64 `json:"output_cost_per_1m"`
		Capabilities   []string `json:"capabilities"`
		Quality        *int     `json:"quality"`
		Enabled        *bool    `json:"enabled"`
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
	if req.ModelName != nil || req.InputCostPer1M != nil || req.OutputCostPer1M != nil || req.Capabilities != nil || req.Quality != nil {
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
		if err := s.ConfigStore.SaveModel(name, key, mn, ic, oc, caps, q); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	s.reloadProxy()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Known Model Catalogs ---

type knownModel struct {
	ModelKey       string   `json:"model_key"`
	ModelName      string   `json:"model_name"`
	InputCostPer1M float64  `json:"input_cost_per_1m"`
	OutputCostPer1M float64 `json:"output_cost_per_1m"`
	Capabilities   []string `json:"capabilities"`
	Quality        int      `json:"quality"`
	Description    string   `json:"description"`
	Recommended    bool     `json:"recommended"`
}

func knownClaudeModels() []knownModel {
	return []knownModel{
		{
			ModelKey: "claude-sonnet-4-20250514", ModelName: "claude-sonnet-4-20250514",
			InputCostPer1M: 3.0, OutputCostPer1M: 15.0,
			Capabilities: []string{"chat", "code", "repair", "tools", "quick"},
			Quality: 90, Description: "Best balance of speed, cost, and intelligence", Recommended: true,
		},
		{
			ModelKey: "claude-opus-4-20250514", ModelName: "claude-opus-4-20250514",
			InputCostPer1M: 15.0, OutputCostPer1M: 75.0,
			Capabilities: []string{"chat", "code", "repair", "tools"},
			Quality: 100, Description: "Most capable, highest cost",
		},
		{
			ModelKey: "claude-haiku-3-5", ModelName: "claude-3-5-haiku-20241022",
			InputCostPer1M: 0.80, OutputCostPer1M: 4.0,
			Capabilities: []string{"chat", "code", "tools", "quick"},
			Quality: 70, Description: "Fast and affordable for routine tasks", Recommended: true,
		},
	}
}

func knownOpenAIModels() []knownModel {
	return []knownModel{
		{
			ModelKey: "gpt-4o", ModelName: "gpt-4o",
			InputCostPer1M: 2.50, OutputCostPer1M: 10.0,
			Capabilities: []string{"chat", "code", "repair", "tools"},
			Quality: 85, Description: "Flagship GPT-4o model", Recommended: true,
		},
		{
			ModelKey: "gpt-4o-mini", ModelName: "gpt-4o-mini",
			InputCostPer1M: 0.15, OutputCostPer1M: 0.60,
			Capabilities: []string{"chat", "code", "tools", "quick"},
			Quality: 70, Description: "Fast and cheap for simple tasks", Recommended: true,
		},
		{
			ModelKey: "gpt-4.1", ModelName: "gpt-4.1",
			InputCostPer1M: 2.0, OutputCostPer1M: 8.0,
			Capabilities: []string{"chat", "code", "repair", "tools"},
			Quality: 88, Description: "Latest GPT-4.1 with strong coding",
		},
		{
			ModelKey: "gpt-4.1-mini", ModelName: "gpt-4.1-mini",
			InputCostPer1M: 0.40, OutputCostPer1M: 1.60,
			Capabilities: []string{"chat", "code", "tools", "quick"},
			Quality: 75, Description: "Affordable GPT-4.1 variant",
		},
		{
			ModelKey: "gpt-4.1-nano", ModelName: "gpt-4.1-nano",
			InputCostPer1M: 0.10, OutputCostPer1M: 0.40,
			Capabilities: []string{"chat", "quick"},
			Quality: 55, Description: "Ultra-cheap for simple tasks",
		},
		{
			ModelKey: "text-embedding-3-small", ModelName: "text-embedding-3-small",
			InputCostPer1M: 0.02, OutputCostPer1M: 0,
			Capabilities: []string{"embed"},
			Quality: 60, Description: "Efficient embedding model",
		},
		{
			ModelKey: "text-embedding-3-large", ModelName: "text-embedding-3-large",
			InputCostPer1M: 0.13, OutputCostPer1M: 0,
			Capabilities: []string{"embed"},
			Quality: 80, Description: "Higher quality embeddings",
		},
	}
}

// matchOllamaModel returns recommended defaults for a discovered Ollama model
// based on its name and family.
func matchOllamaModel(name, family string) *knownModel {
	// Known model families with sensible defaults
	ollamaDefaults := map[string]knownModel{
		"llama": {
			Capabilities: []string{"chat", "code", "tools"},
			Quality: 70, Description: "Meta Llama model",
		},
		"qwen2": {
			Capabilities: []string{"chat", "code", "tools"},
			Quality: 70, Description: "Alibaba Qwen2 model",
		},
		"qwen3": {
			Capabilities: []string{"chat", "code", "tools"},
			Quality: 75, Description: "Alibaba Qwen3 model",
		},
		"gemma": {
			Capabilities: []string{"chat", "code"},
			Quality: 65, Description: "Google Gemma model",
		},
		"gemma2": {
			Capabilities: []string{"chat", "code"},
			Quality: 70, Description: "Google Gemma 2 model",
		},
		"mistral": {
			Capabilities: []string{"chat", "code", "tools"},
			Quality: 70, Description: "Mistral AI model",
		},
		"codellama": {
			Capabilities: []string{"code", "chat"},
			Quality: 65, Description: "Meta Code Llama model",
		},
		"deepseek-coder": {
			Capabilities: []string{"code", "chat"},
			Quality: 70, Description: "DeepSeek Coder model",
		},
		"phi": {
			Capabilities: []string{"chat", "code"},
			Quality: 60, Description: "Microsoft Phi model",
		},
		"nomic-embed-text": {
			Capabilities: []string{"embed"},
			Quality: 60, Description: "Nomic embedding model",
		},
		"mxbai-embed-large": {
			Capabilities: []string{"embed"},
			Quality: 70, Description: "Mixedbread embedding model",
		},
		"all-minilm": {
			Capabilities: []string{"embed"},
			Quality: 50, Description: "All-MiniLM embedding model",
		},
		"glm4": {
			Capabilities: []string{"chat", "code", "tools"},
			Quality: 70, Description: "GLM-4 model",
		},
	}

	// Try family match first
	if family != "" {
		if def, ok := ollamaDefaults[family]; ok {
			result := def
			result.ModelKey = name
			result.ModelName = name
			return &result
		}
	}

	// Try name prefix match
	for prefix, def := range ollamaDefaults {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			result := def
			result.ModelKey = name
			result.ModelName = name
			return &result
		}
	}

	return nil
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
