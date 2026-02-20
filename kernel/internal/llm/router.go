package llm

import (
	"fmt"

	"jodo-kernel/internal/config"
)

// RouteResult holds the chosen provider and model for a request.
type RouteResult struct {
	ProviderName string
	Provider     Provider
	Model        string             // actual API model name
	ModelKey     string             // config map key (for cost lookup)
	ModelConfig  config.ModelConfig
}

// Router selects the best affordable provider for a given intent.
type Router struct {
	providers map[string]Provider
	configs   map[string]config.ProviderConfig
	routing   config.RoutingConfig
	budget    *BudgetTracker
	busy      *BusyTracker
	vram      *VRAMTracker
}

func NewRouter(providers map[string]Provider, configs map[string]config.ProviderConfig, routing config.RoutingConfig, budget *BudgetTracker, busy *BusyTracker, vram *VRAMTracker) *Router {
	return &Router{
		providers: providers,
		configs:   configs,
		routing:   routing,
		budget:    budget,
		busy:      busy,
		vram:      vram,
	}
}

// Route picks the best provider for the given intent, budget, and tool requirements.
// Supports both "model@provider" references and legacy "provider" references.
//
// When a VRAMTracker is active, Route does two passes:
//  1. Try only Ollama models marked "prefer_loaded" that are already in VRAM
//  2. Normal pass — try all preferences in order (including cold Ollama models and cloud)
func (r *Router) Route(intent string, maxTokens int, needsTools bool) (*RouteResult, error) {
	preferences, ok := r.routing.IntentPreferences[intent]
	if !ok {
		preferences = make([]string, 0, len(r.providers))
		for name := range r.providers {
			preferences = append(preferences, name)
		}
	}

	// Pass 1: prefer already-loaded Ollama models marked prefer_loaded (avoids loading latency)
	if r.vram != nil {
		if result := r.tryRoute(preferences, intent, maxTokens, needsTools, true); result != nil {
			return result, nil
		}
	}

	// Pass 2: normal routing — all candidates
	if result := r.tryRoute(preferences, intent, maxTokens, needsTools, false); result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("no affordable provider available for intent %q", intent)
}

// tryRoute iterates preferences and returns the first viable route.
// If onlyLoaded is true, only considers Ollama models marked PreferLoaded that are in VRAM.
func (r *Router) tryRoute(preferences []string, intent string, maxTokens int, needsTools bool, onlyLoaded bool) *RouteResult {
	for _, ref := range preferences {
		modelKey, provName, isModelRef := config.ParseModelRef(ref)

		provider, ok := r.providers[provName]
		if !ok {
			continue
		}

		// In loaded-only pass, skip non-Ollama providers
		if onlyLoaded && provName != "ollama" {
			continue
		}

		// Skip providers that don't support tools when tools are needed
		if needsTools && !provider.SupportsTools() {
			continue
		}

		provCfg, ok := r.configs[provName]
		if !ok {
			continue
		}

		var mk string
		var mc config.ModelConfig
		var found bool

		if isModelRef {
			// Direct model@provider reference — look up specific model
			mc, found = provCfg.Models[modelKey]
			if !found {
				continue
			}
			mk = modelKey
			// Verify capabilities
			if !hasCapability(mc.Capabilities, intent) {
				continue
			}
			if needsTools && !hasCapability(mc.Capabilities, "tools") {
				continue
			}
		} else {
			// Legacy provider-only reference — pick best model
			mk, mc, found = r.bestModelForIntent(provCfg, intent, needsTools)
			if !found {
				continue
			}
		}

		// Check busy status (skip if model is overloaded)
		if r.busy != nil && r.busy.IsBusy(provName, mk) {
			continue
		}

		// VRAM check for Ollama
		if r.vram != nil && provName == "ollama" {
			if onlyLoaded {
				// Pass 1: only use models marked prefer_loaded that are in VRAM
				if !mc.PreferLoaded {
					continue
				}
				if !r.vram.IsLoaded(mc.ModelName(mk)) {
					continue
				}
			} else {
				// Pass 2: check if model would fit
				if !r.vram.CanFit(mc.ModelName(mk), mc.VRAMEstimateBytes) {
					continue
				}
			}
		}

		estimated := EstimateCost(mc, maxTokens)
		canAfford, _, err := r.budget.CanAfford(provName, estimated, intent)
		if err != nil || !canAfford {
			continue
		}

		return &RouteResult{
			ProviderName: provName,
			Provider:     provider,
			Model:        mc.ModelName(mk),
			ModelKey:     mk,
			ModelConfig:  mc,
		}
	}

	return nil
}

// RouteEmbed picks a provider that supports embeddings.
// Like Route(), prefers already-loaded Ollama models to avoid swaps.
func (r *Router) RouteEmbed() (*RouteResult, error) {
	preferences, ok := r.routing.IntentPreferences["embed"]
	if !ok {
		preferences = []string{"ollama", "openai"}
	}

	// Pass 1: prefer already-loaded Ollama embed models
	if r.vram != nil {
		if result := r.tryRouteEmbed(preferences, true); result != nil {
			return result, nil
		}
	}

	// Pass 2: normal routing
	if result := r.tryRouteEmbed(preferences, false); result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("no provider available for embeddings")
}

func (r *Router) tryRouteEmbed(preferences []string, onlyLoaded bool) *RouteResult {
	for _, ref := range preferences {
		modelKey, provName, isModelRef := config.ParseModelRef(ref)

		provider, ok := r.providers[provName]
		if !ok || !provider.SupportsEmbed() {
			continue
		}

		if onlyLoaded && provName != "ollama" {
			continue
		}

		provCfg := r.configs[provName]

		if isModelRef {
			mc, found := provCfg.Models[modelKey]
			if !found || !hasCapability(mc.Capabilities, "embed") {
				continue
			}
			if r.vram != nil && provName == "ollama" {
				if onlyLoaded {
					if !mc.PreferLoaded || !r.vram.IsLoaded(mc.ModelName(modelKey)) {
						continue
					}
				} else if !r.vram.CanFit(mc.ModelName(modelKey), mc.VRAMEstimateBytes) {
					continue
				}
			}
			canAfford, _, _ := r.budget.CanAfford(provName, EstimateCost(mc, 100), "embed")
			if canAfford {
				return &RouteResult{
					ProviderName: provName,
					Provider:     provider,
					Model:        mc.ModelName(modelKey),
					ModelKey:     modelKey,
					ModelConfig:  mc,
				}
			}
		} else {
			for mk, mc := range provCfg.Models {
				if !hasCapability(mc.Capabilities, "embed") {
					continue
				}
				if r.vram != nil && provName == "ollama" {
					if onlyLoaded {
						if !mc.PreferLoaded || !r.vram.IsLoaded(mc.ModelName(mk)) {
							continue
						}
					} else if !r.vram.CanFit(mc.ModelName(mk), mc.VRAMEstimateBytes) {
						continue
					}
				}
				canAfford, _, _ := r.budget.CanAfford(provName, EstimateCost(mc, 100), "embed")
				if canAfford {
					return &RouteResult{
						ProviderName: provName,
						Provider:     provider,
						Model:        mc.ModelName(mk),
						ModelKey:     mk,
						ModelConfig:  mc,
					}
				}
			}
		}
	}

	return nil
}

// bestModelForIntent finds the highest-quality model that supports the given intent.
// If needsTools is true, the model must have the "tools" capability.
func (r *Router) bestModelForIntent(provCfg config.ProviderConfig, intent string, needsTools bool) (string, config.ModelConfig, bool) {
	var bestKey string
	var bestCfg config.ModelConfig
	bestQuality := -1

	for key, cfg := range provCfg.Models {
		if !hasCapability(cfg.Capabilities, intent) {
			continue
		}
		if needsTools && !hasCapability(cfg.Capabilities, "tools") {
			continue
		}
		if cfg.Quality > bestQuality {
			bestQuality = cfg.Quality
			bestKey = key
			bestCfg = cfg
		}
	}

	return bestKey, bestCfg, bestQuality >= 0
}

func hasCapability(caps []string, intent string) bool {
	for _, c := range caps {
		if c == intent {
			return true
		}
	}
	return false
}
