package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

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
		Name          string      `json:"name"`
		Family        string      `json:"family"`
		ParameterSize string      `json:"parameter_size"`
		Quantization  string      `json:"quantization"`
		SizeBytes     int64       `json:"size_bytes"`
		VRAMEstimate  int64       `json:"vram_estimate"`
		SupportsTools *bool       `json:"supports_tools"`
		HasThinking   bool        `json:"has_thinking"`
		Recommended   *knownModel `json:"recommended,omitempty"`
	}

	// Fetch /api/show for each model concurrently (semaphore of 4)
	type showResult struct {
		index        int
		capabilities []string
		vramEstimate int64
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
			// VRAM estimate: disk size x 1.15 (KV cache overhead)
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
			// Add "plan" capability for non-embedding models (any model that can code/chat can plan)
			if !containsStr(rec.Capabilities, "plan") && !containsStr(rec.Capabilities, "embed") && rec.Quality >= 60 {
				rec.Capabilities = append(rec.Capabilities, "plan")
			}
			dm.Recommended = rec
		}
		models = append(models, dm)
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
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
