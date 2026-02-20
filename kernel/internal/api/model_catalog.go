package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// knownModel represents a model with enriched metadata (pricing, tier, capabilities).
// Used by model discovery for both cloud providers and Ollama.
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

	// 4. Add "code" and "plan" capabilities for non-embedding models
	if !isEmbed {
		if km.Quality >= 70 && !containsStr(km.Capabilities, "code") {
			km.Capabilities = append(km.Capabilities, "code")
		}
		if km.Quality >= 60 && !containsStr(km.Capabilities, "plan") {
			km.Capabilities = append(km.Capabilities, "plan")
		}
	}

	// 5. Fill in defaults if pricing DB didn't have this model
	if len(km.Capabilities) == 0 {
		if isEmbed {
			km.Capabilities = []string{"embed"}
		} else {
			km.Capabilities = []string{"chat"}
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

// buildOllamaDefaults builds recommended defaults for a discovered Ollama model
// using the Ollama API response instead of a hardcoded model catalog.
//
// Rules:
//   - Embedding models (name contains "embed", or known embed-only names) → ["embed"]
//   - All other models → ["chat", "code"]
//   - Ollama reports "thinking" capability → add "plan" (thinking = structured reasoning)
//   - Quality derived from parameter size
//   - NOTE: Ollama "tools" capability is tracked separately via supports_tools, NOT in capabilities.
//     Capabilities only contain kernel intents: code, plan, chat, embed.
func buildOllamaDefaults(name, family, paramSize string, apiCaps []string) *knownModel {
	nameLower := strings.ToLower(name)

	km := &knownModel{
		ModelKey:  name,
		ModelName: name,
	}

	// Embedding detection: name contains "embed" or known embedding-only model names
	if isEmbeddingModel(nameLower) {
		km.Capabilities = []string{"embed"}
		km.Quality = qualityFromParamSize(paramSize, 65)
		km.Description = "Embedding model"
		return km
	}

	// All non-embedding models get chat + code
	km.Capabilities = []string{"chat", "code"}

	// Add plan capability from Ollama "thinking" capability
	if containsStr(apiCaps, "thinking") {
		km.Capabilities = append(km.Capabilities, "plan")
	}

	km.Quality = qualityFromParamSize(paramSize, 65)

	// Description from family or base model name
	if family != "" {
		km.Description = family + " model"
	} else {
		base := name
		if idx := strings.Index(name, ":"); idx > 0 {
			base = name[:idx]
		}
		km.Description = base
	}

	return km
}

// isEmbeddingModel detects embedding-only models from their name.
func isEmbeddingModel(nameLower string) bool {
	if strings.Contains(nameLower, "embed") {
		return true
	}
	// Known embedding models without "embed" in the name
	embedPrefixes := []string{"all-minilm", "bge-", "bge:"}
	for _, p := range embedPrefixes {
		if strings.HasPrefix(nameLower, p) {
			return true
		}
	}
	return false
}

// qualityFromParamSize derives a quality score from the model's parameter size string.
// paramSize is typically "7B", "13B", "70B", "1.5B", etc.
func qualityFromParamSize(paramSize string, defaultQuality int) int {
	if paramSize == "" {
		return defaultQuality
	}

	s := strings.ToLower(strings.TrimSpace(paramSize))
	s = strings.TrimSuffix(s, "b")

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultQuality
	}

	switch {
	case val >= 70:
		return 88
	case val >= 30:
		return 80
	case val >= 13:
		return 72
	case val >= 7:
		return 65
	case val >= 3:
		return 55
	default:
		return 40
	}
}
