package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
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

// GET /api/settings/providers/:name/discover
func (s *Server) handleSettingsProviderDiscover(c *gin.Context) {
	name := c.Param("name")

	switch name {
	case "ollama":
		s.discoverOllamaModels(c)
	case "claude":
		apiKey, err := s.ConfigStore.GetProviderAPIKey("claude")
		if err != nil || apiKey == "" {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "No API key configured for Claude"})
			return
		}
		discoverClaudeModels(c, apiKey)
	case "openai":
		apiKey, err := s.ConfigStore.GetProviderAPIKey("openai")
		if err != nil || apiKey == "" {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "No API key configured for OpenAI"})
			return
		}
		discoverOpenAIModels(c, apiKey)
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

	s.discoverOllamaModelsWithURL(c, baseURL)
}

func (s *Server) discoverOllamaModelsWithURL(c *gin.Context, baseURL string) {
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
				Family            string   `json:"family"`
				Families          []string `json:"families"`
				ParameterSize     string   `json:"parameter_size"`
				QuantizationLevel string   `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "failed to parse Ollama response"})
		return
	}

	type discoveredModel struct {
		Name            string      `json:"name"`
		Family          string      `json:"family"`
		ParameterSize   string      `json:"parameter_size"`
		Quantization    string      `json:"quantization"`
		SizeBytes       int64       `json:"size_bytes"`
		VRAMEstimate    int64       `json:"vram_estimate"`
		SupportsTools   *bool       `json:"supports_tools"`
		HasThinking     bool        `json:"has_thinking"`
		Recommended     *knownModel `json:"recommended,omitempty"`
	}

	// Fetch /api/show for each model concurrently (semaphore of 4)
	type showResult struct {
		index         int
		capabilities  []string
		vramEstimate  int64
	}

	showResults := make([]showResult, len(ollamaResp.Models))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i, m := range ollamaResp.Models {
		wg.Add(1)
		go func(idx int, modelName string, diskSize int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sr := showResult{index: idx}
			// VRAM estimate: disk size × 1.15 (KV cache overhead)
			sr.vramEstimate = int64(float64(diskSize) * 1.15)

			showBody := fmt.Sprintf(`{"name":%q}`, modelName)
			showResp, err := client.Post(baseURL+"/api/show", "application/json", strings.NewReader(showBody))
			if err != nil {
				showResults[idx] = sr
				return
			}
			defer showResp.Body.Close()

			if showResp.StatusCode == http.StatusOK {
				var showData struct {
					Capabilities []string `json:"capabilities"`
				}
				if err := json.NewDecoder(showResp.Body).Decode(&showData); err == nil {
					sr.capabilities = showData.Capabilities
				}
			}
			showResults[idx] = sr
		}(i, m.Name, m.Size)
	}
	wg.Wait()

	models := make([]discoveredModel, 0, len(ollamaResp.Models))
	for i, m := range ollamaResp.Models {
		dm := discoveredModel{
			Name:          m.Name,
			Family:        m.Details.Family,
			ParameterSize: m.Details.ParameterSize,
			Quantization:  m.Details.QuantizationLevel,
			SizeBytes:     m.Size,
			VRAMEstimate:  showResults[i].vramEstimate,
		}

		// Process capabilities from /api/show
		apiCaps := showResults[i].capabilities
		if len(apiCaps) > 0 {
			hasTools := containsStr(apiCaps, "tools")
			dm.SupportsTools = &hasTools
			dm.HasThinking = containsStr(apiCaps, "thinking")
		}

		// Match against known Ollama model families for recommended defaults
		if rec := matchOllamaModel(m.Name, m.Details.Family); rec != nil {
			// Override tool support from API if available
			if dm.SupportsTools != nil {
				if *dm.SupportsTools {
					if !containsStr(rec.Capabilities, "tools") {
						rec.Capabilities = append(rec.Capabilities, "tools")
					}
				} else {
					// Remove "tools" from capabilities if API says no tools
					filtered := make([]string, 0, len(rec.Capabilities))
					for _, c := range rec.Capabilities {
						if c != "tools" {
							filtered = append(filtered, c)
						}
					}
					rec.Capabilities = filtered
				}
			}
			dm.Recommended = rec
		}
		models = append(models, dm)
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
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

// --- Model Discovery ---

type knownModel struct {
	ModelKey        string   `json:"model_key"`
	ModelName       string   `json:"model_name"`
	InputCostPer1M  float64  `json:"input_cost_per_1m"`
	OutputCostPer1M float64  `json:"output_cost_per_1m"`
	Capabilities    []string `json:"capabilities"`
	Quality         int      `json:"quality"`
	Description     string   `json:"description"`
	Recommended     bool     `json:"recommended"`
	Tier            string   `json:"tier"` // "flagship", "mid", "budget", "embed", "reasoning"
}

// --- Pricing cache (fetched from litellm community pricing DB) ---

const pricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
const pricingTTL = 24 * time.Hour

type pricingEntry struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	Mode               string  `json:"mode"`
	SupportsFunctions  bool    `json:"supports_function_calling"`
	SupportsVision     bool    `json:"supports_vision"`
	SupportsReasoning  bool    `json:"supports_reasoning"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
}

var pricingDB = &modelPricingDB{}

type modelPricingDB struct {
	mu        sync.RWMutex
	entries   map[string]pricingEntry
	fetchedAt time.Time
}

// lookup tries exact match, then provider-prefixed matches.
func (db *modelPricingDB) lookup(modelID string) (pricingEntry, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.entries == nil {
		return pricingEntry{}, false
	}
	// Exact match
	if e, ok := db.entries[modelID]; ok {
		return e, true
	}
	// Try common prefixes (litellm sometimes uses provider/model format)
	for _, prefix := range []string{"anthropic/", "openai/", "azure/"} {
		if e, ok := db.entries[prefix+modelID]; ok {
			return e, true
		}
	}
	return pricingEntry{}, false
}

// refresh fetches pricing data if cache is stale. Non-blocking if fresh.
func (db *modelPricingDB) refresh() {
	db.mu.RLock()
	fresh := db.entries != nil && time.Since(db.fetchedAt) < pricingTTL
	db.mu.RUnlock()
	if fresh {
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(pricingURL)
	if err != nil {
		log.Printf("[settings] failed to fetch pricing DB: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("[settings] pricing DB returned %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		log.Printf("[settings] failed to parse pricing DB: %v", err)
		return
	}

	entries := make(map[string]pricingEntry, len(raw))
	for k, v := range raw {
		if k == "sample_spec" {
			continue
		}
		var e pricingEntry
		json.Unmarshal(v, &e) // ignore individual parse errors
		entries[k] = e
	}

	db.mu.Lock()
	db.entries = entries
	db.fetchedAt = time.Now()
	db.mu.Unlock()
	log.Printf("[settings] pricing DB refreshed: %d models", len(entries))
}

// --- Claude discovery ---

func discoverClaudeModels(c *gin.Context, apiKey string) {
	pricingDB.refresh()

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.anthropic.com/v1/models?limit=100", nil)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": fmt.Sprintf("Cannot reach Anthropic API: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": fmt.Sprintf("Anthropic API returned %d: %s", resp.StatusCode, string(body))})
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "Failed to parse Anthropic response"})
		return
	}

	models := make([]knownModel, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		if m.Type != "" && m.Type != "model" {
			continue
		}
		km := enrichModel(m.ID, m.DisplayName, "claude")
		models = append(models, km)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Quality > models[j].Quality
	})
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// --- OpenAI discovery ---

func discoverOpenAIModels(c *gin.Context, apiKey string) {
	pricingDB.refresh()

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": fmt.Sprintf("Cannot reach OpenAI API: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": fmt.Sprintf("OpenAI API returned %d: %s", resp.StatusCode, string(body))})
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "Failed to parse OpenAI response"})
		return
	}

	// Filter to relevant model families
	relevantPrefixes := []string{
		"gpt-4", "gpt-3.5", "gpt-5",
		"o1", "o3", "o4",
		"text-embedding", "chatgpt-4o",
	}

	models := make([]knownModel, 0)
	for _, m := range apiResp.Data {
		relevant := false
		for _, prefix := range relevantPrefixes {
			if strings.HasPrefix(m.ID, prefix) {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		// Skip non-text variants
		if strings.Contains(m.ID, "-audio") || strings.Contains(m.ID, "-realtime") ||
			strings.Contains(m.ID, "-search") || strings.Contains(m.ID, "-instruct") ||
			strings.Contains(m.ID, "-transcribe") || strings.Contains(m.ID, "-tts") {
			continue
		}
		km := enrichModel(m.ID, "", "openai")
		models = append(models, km)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Quality > models[j].Quality
	})
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// --- Shared enrichment ---

// enrichModel enriches a model ID with pricing from litellm + tier/quality from family inference.
func enrichModel(id, displayName, provider string) knownModel {
	km := knownModel{
		ModelKey:  id,
		ModelName: id,
	}
	if displayName != "" {
		km.Description = displayName
	}

	// 1. Try to get pricing + capabilities from litellm pricing DB
	if pe, ok := pricingDB.lookup(id); ok {
		km.InputCostPer1M = pe.InputCostPerToken * 1_000_000
		km.OutputCostPer1M = pe.OutputCostPerToken * 1_000_000

		// Derive capabilities from litellm fields
		caps := []string{}
		if pe.Mode == "chat" {
			caps = append(caps, "chat")
		}
		if pe.Mode == "embedding" {
			caps = append(caps, "embed")
		}
		if pe.SupportsFunctions {
			caps = append(caps, "tools")
		}
		if pe.SupportsReasoning {
			caps = append(caps, "reasoning")
		}
		if len(caps) > 0 {
			km.Capabilities = caps
		}
	}

	// 2. Infer tier, quality, and recommended from model family (stable across versions)
	inferTierAndQuality(&km, id, provider)

	// 3. Detect embedding models from the model ID itself
	idLower := strings.ToLower(id)
	isEmbed := km.Tier == "embed" || containsStr(km.Capabilities, "embed") ||
		strings.Contains(idLower, "embed")

	// 4. Add "code" capability for non-embedding models with decent quality
	if !isEmbed && !containsStr(km.Capabilities, "code") {
		if km.Quality >= 70 {
			km.Capabilities = append(km.Capabilities, "code")
		}
	}

	// 5. Fill in defaults if pricing DB didn't have this model
	if len(km.Capabilities) == 0 {
		if isEmbed {
			km.Capabilities = []string{"embed"}
		} else {
			km.Capabilities = []string{"chat", "tools"}
		}
	}
	if km.Tier == "" {
		km.Tier = "mid"
	}
	if km.Quality == 0 {
		km.Quality = 70
	}
	if km.Description == "" {
		km.Description = strings.Title(provider) + " model"
	}

	return km
}

// inferTierAndQuality assigns tier, quality, recommended, and optionally description
// based on the model family prefix. This is the only part that uses pattern matching;
// pricing comes from litellm.
func inferTierAndQuality(km *knownModel, id, provider string) {
	type tierInfo struct {
		tier        string
		quality     int
		recommended bool
		desc        string
	}

	// Ordered from most specific to least specific
	var patterns []struct {
		prefix string
		info   tierInfo
	}

	if provider == "claude" {
		patterns = []struct {
			prefix string
			info   tierInfo
		}{
			{"claude-opus-4", tierInfo{"flagship", 100, false, "Most capable Claude model"}},
			{"claude-sonnet-4", tierInfo{"mid", 90, true, "Best balance of speed, cost, and intelligence"}},
			{"claude-haiku-4", tierInfo{"budget", 75, true, "Fast and affordable"}},
			{"claude-3-5-sonnet", tierInfo{"mid", 85, false, "Claude 3.5 Sonnet"}},
			{"claude-3-5-haiku", tierInfo{"budget", 70, true, "Fast and affordable"}},
			{"claude-3-opus", tierInfo{"flagship", 95, false, "Claude 3 Opus"}},
			{"claude-3-sonnet", tierInfo{"mid", 80, false, "Claude 3 Sonnet"}},
			{"claude-3-haiku", tierInfo{"budget", 65, false, "Claude 3 Haiku"}},
		}
	} else {
		patterns = []struct {
			prefix string
			info   tierInfo
		}{
			// GPT-5 family
			{"gpt-5.1", tierInfo{"flagship", 98, false, "GPT-5.1"}},
			{"gpt-5", tierInfo{"flagship", 96, false, "GPT-5"}},
			// Reasoning
			{"o4-mini", tierInfo{"reasoning", 92, true, "Fast reasoning model"}},
			{"o3-pro", tierInfo{"flagship", 100, false, "Most capable reasoning model"}},
			{"o3-mini", tierInfo{"reasoning", 88, false, "Efficient reasoning model"}},
			{"o3", tierInfo{"flagship", 98, false, "Advanced reasoning model"}},
			{"o1-pro", tierInfo{"flagship", 96, false, "Deep reasoning model"}},
			{"o1-mini", tierInfo{"reasoning", 82, false, "Small reasoning model"}},
			{"o1", tierInfo{"reasoning", 90, false, "Reasoning model"}},
			// GPT-4.1 family
			{"gpt-4.1-nano", tierInfo{"budget", 55, false, "Ultra-cheap for simple tasks"}},
			{"gpt-4.1-mini", tierInfo{"budget", 75, false, "Affordable GPT-4.1"}},
			{"gpt-4.1", tierInfo{"mid", 88, true, "Strong coding model"}},
			// GPT-4o family
			{"gpt-4o-mini", tierInfo{"budget", 70, true, "Fast and cheap"}},
			{"gpt-4o", tierInfo{"mid", 85, true, "GPT-4o"}},
			// GPT-3.5
			{"gpt-3.5-turbo", tierInfo{"budget", 50, false, "Legacy GPT-3.5"}},
			// Embeddings
			{"text-embedding-3-large", tierInfo{"embed", 80, false, "Higher quality embeddings"}},
			{"text-embedding-3-small", tierInfo{"embed", 60, true, "Efficient embedding model"}},
			{"text-embedding-ada", tierInfo{"embed", 40, false, "Legacy embedding model"}},
		}
	}

	for _, p := range patterns {
		if strings.HasPrefix(id, p.prefix) {
			km.Tier = p.info.tier
			km.Quality = p.info.quality
			km.Recommended = p.info.recommended
			if km.Description == "" || km.Description == id {
				km.Description = p.info.desc
			}
			return
		}
	}
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// matchOllamaModel returns recommended defaults for a discovered Ollama model
// based on its name and family.
func matchOllamaModel(name, family string) *knownModel {
	nameLower := strings.ToLower(name)

	// 1. Check name for embedding/embed — these are always embedding-only models
	if strings.Contains(nameLower, "embed") {
		return &knownModel{
			ModelKey: name, ModelName: name,
			Capabilities: []string{"embed"},
			Quality: 65, Description: "Embedding model",
		}
	}

	// 2. Check for reasoning models (deepseek-r1, qwq, etc.)
	if strings.Contains(nameLower, "-r1") || strings.HasPrefix(nameLower, "qwq") ||
		strings.Contains(nameLower, "reasoning") {
		return &knownModel{
			ModelKey: name, ModelName: name,
			Capabilities: []string{"chat", "code", "reasoning"},
			Quality: 80, Description: "Reasoning model",
		}
	}

	// 3. Check for code-specialized models
	if strings.Contains(nameLower, "coder") || strings.Contains(nameLower, "codellama") ||
		strings.HasPrefix(nameLower, "starcoder") || strings.HasPrefix(nameLower, "codestral") {
		return &knownModel{
			ModelKey: name, ModelName: name,
			Capabilities: []string{"code", "chat"},
			Quality: 70, Description: "Code-specialized model",
		}
	}

	// 4. Known model families with sensible defaults (ordered: more specific first)
	type ollamaPattern struct {
		prefix string
		model  knownModel
	}
	patterns := []ollamaPattern{
		// Specific families first
		{"deepseek-v3", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 85, Description: "DeepSeek V3 model"}},
		{"deepseek-coder", knownModel{Capabilities: []string{"code", "chat"}, Quality: 70, Description: "DeepSeek Coder model"}},
		{"deepseek", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 75, Description: "DeepSeek model"}},
		{"llama4", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 80, Description: "Meta Llama 4 model"}},
		{"llama3.3", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 78, Description: "Meta Llama 3.3 model"}},
		{"llama3.2", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "Meta Llama 3.2 model"}},
		{"llama3.1", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 70, Description: "Meta Llama 3.1 model"}},
		{"llama", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 70, Description: "Meta Llama model"}},
		{"qwen3", knownModel{Capabilities: []string{"chat", "code", "tools", "reasoning"}, Quality: 78, Description: "Alibaba Qwen3 model"}},
		{"qwen2.5-coder", knownModel{Capabilities: []string{"code", "chat", "tools"}, Quality: 75, Description: "Qwen 2.5 Coder model"}},
		{"qwen2.5", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "Alibaba Qwen 2.5 model"}},
		{"qwen2", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 70, Description: "Alibaba Qwen2 model"}},
		{"gemma3", knownModel{Capabilities: []string{"chat", "code"}, Quality: 72, Description: "Google Gemma 3 model"}},
		{"gemma2", knownModel{Capabilities: []string{"chat", "code"}, Quality: 70, Description: "Google Gemma 2 model"}},
		{"gemma", knownModel{Capabilities: []string{"chat", "code"}, Quality: 65, Description: "Google Gemma model"}},
		{"mistral-small", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "Mistral Small model"}},
		{"mistral-large", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 82, Description: "Mistral Large model"}},
		{"mistral-nemo", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 70, Description: "Mistral Nemo model"}},
		{"mistral", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 70, Description: "Mistral AI model"}},
		{"mixtral", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "Mixtral MoE model"}},
		{"codestral", knownModel{Capabilities: []string{"code", "chat", "tools"}, Quality: 78, Description: "Mistral Codestral model"}},
		{"phi4", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "Microsoft Phi-4 model"}},
		{"phi3.5", knownModel{Capabilities: []string{"chat", "code"}, Quality: 65, Description: "Microsoft Phi-3.5 model"}},
		{"phi3", knownModel{Capabilities: []string{"chat", "code"}, Quality: 62, Description: "Microsoft Phi-3 model"}},
		{"phi", knownModel{Capabilities: []string{"chat", "code"}, Quality: 60, Description: "Microsoft Phi model"}},
		{"command-r", knownModel{Capabilities: []string{"chat", "tools"}, Quality: 72, Description: "Cohere Command-R model"}},
		{"aya", knownModel{Capabilities: []string{"chat"}, Quality: 60, Description: "Cohere Aya multilingual model"}},
		{"glm4", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "GLM-4 model"}},
		{"glm-4", knownModel{Capabilities: []string{"chat", "code", "tools"}, Quality: 72, Description: "GLM-4 model"}},
		{"yi", knownModel{Capabilities: []string{"chat", "code"}, Quality: 65, Description: "01.AI Yi model"}},
		{"internlm", knownModel{Capabilities: []string{"chat", "code"}, Quality: 65, Description: "InternLM model"}},
		{"nomic-embed", knownModel{Capabilities: []string{"embed"}, Quality: 60, Description: "Nomic embedding model"}},
		{"mxbai-embed", knownModel{Capabilities: []string{"embed"}, Quality: 70, Description: "Mixedbread embedding model"}},
		{"all-minilm", knownModel{Capabilities: []string{"embed"}, Quality: 50, Description: "All-MiniLM embedding model"}},
		{"snowflake-arctic-embed", knownModel{Capabilities: []string{"embed"}, Quality: 68, Description: "Snowflake Arctic embedding model"}},
		{"bge-", knownModel{Capabilities: []string{"embed"}, Quality: 65, Description: "BGE embedding model"}},
	}

	// Try name prefix match (ordered, so more specific patterns match first)
	for _, p := range patterns {
		if strings.HasPrefix(nameLower, p.prefix) {
			result := p.model
			result.ModelKey = name
			result.ModelName = name
			return &result
		}
	}

	// 5. Try family match as fallback
	if family != "" {
		familyLower := strings.ToLower(family)
		for _, p := range patterns {
			if familyLower == p.prefix || strings.HasPrefix(familyLower, p.prefix) {
				result := p.model
				result.ModelKey = name
				result.ModelName = name
				return &result
			}
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
