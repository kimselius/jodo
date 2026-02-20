package llm

import (
	"fmt"
	"log"
	"strings"

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
func (r *Router) Route(intent string, needsTools bool) (*RouteResult, error) {
	preferences, ok := r.routing.IntentPreferences[intent]
	if !ok {
		preferences = make([]string, 0, len(r.providers))
		for name := range r.providers {
			preferences = append(preferences, name)
		}
	}

	log.Printf("[router] routing intent=%q tools=%v preferences=[%s]", intent, needsTools, strings.Join(preferences, ", "))

	// Pass 1: prefer already-loaded Ollama models (avoids loading latency)
	if r.vram != nil {
		if result := r.tryRoute(preferences, intent, needsTools, true); result != nil {
			log.Printf("[router] pass1 selected: %s/%s (already loaded in VRAM)", result.ProviderName, result.Model)
			return result, nil
		}
		log.Printf("[router] pass1: no loaded model found, falling through to pass2")
	}

	// Pass 2: normal routing — all candidates in preference order
	if result := r.tryRoute(preferences, intent, needsTools, false); result != nil {
		log.Printf("[router] pass2 selected: %s/%s", result.ProviderName, result.Model)
		return result, nil
	}

	log.Printf("[router] no viable model found for intent=%q", intent)
	return nil, fmt.Errorf("no affordable provider available for intent %q", intent)
}

// tryRoute iterates preferences and returns the first viable route.
// If onlyLoaded is true, only considers Ollama models marked PreferLoaded that are in VRAM.
func (r *Router) tryRoute(preferences []string, intent string, needsTools bool, onlyLoaded bool) *RouteResult {
	pass := "pass2"
	if onlyLoaded {
		pass = "pass1"
	}

	for i, ref := range preferences {
		modelKey, provName, isModelRef := config.ParseModelRef(ref)

		provider, ok := r.providers[provName]
		if !ok {
			log.Printf("[router] %s [%d] %s: skip — provider %q not registered", pass, i, ref, provName)
			continue
		}

		if onlyLoaded && provName != "ollama" {
			continue // silently skip non-ollama in pass1
		}

		provCfg, ok := r.configs[provName]
		if !ok {
			log.Printf("[router] %s [%d] %s: skip — no config for provider %q", pass, i, ref, provName)
			continue
		}

		var mk string
		var mc config.ModelConfig
		var found bool

		if isModelRef {
			mc, found = provCfg.Models[modelKey]
			if !found {
				log.Printf("[router] %s [%d] %s: skip — model key %q not found in provider %q (available: %s)",
					pass, i, ref, modelKey, provName, modelKeys(provCfg.Models))
				continue
			}
			mk = modelKey
			if !hasCapability(mc.Capabilities, intent) {
				log.Printf("[router] %s [%d] %s: skip — missing capability %q (has: %v)", pass, i, ref, intent, mc.Capabilities)
				continue
			}
			if needsTools && !hasCapability(mc.Capabilities, "tools") {
				log.Printf("[router] %s [%d] %s: skip — needs tools but model lacks tools capability", pass, i, ref)
				continue
			}
		} else {
			mk, mc, found = r.bestModelForIntent(provCfg, intent, needsTools)
			if !found {
				log.Printf("[router] %s [%d] %s: skip — no model with capability %q in provider", pass, i, ref, intent)
				continue
			}
		}

		if r.busy != nil && r.busy.IsBusy(provName, mk) {
			log.Printf("[router] %s [%d] %s: skip — model is busy", pass, i, ref)
			continue
		}

		if r.vram != nil && provName == "ollama" {
			modelName := mc.ModelName(mk)
			if onlyLoaded {
				if !mc.PreferLoaded {
					log.Printf("[router] %s [%d] %s: skip — prefer_loaded=false", pass, i, ref)
					continue
				}
				if !r.vram.IsLoaded(modelName) {
					log.Printf("[router] %s [%d] %s: skip — not loaded in VRAM (model_name=%q)", pass, i, ref, modelName)
					continue
				}
			} else if !r.vram.CanFit(modelName, mc.VRAMEstimateBytes) {
				log.Printf("[router] %s [%d] %s: skip — won't fit in VRAM (need=%s, model_name=%q)",
					pass, i, ref, formatBytes(mc.VRAMEstimateBytes), modelName)
				continue
			}
		}

		if !r.budget.HasBudget(provName, intent) {
			log.Printf("[router] %s [%d] %s: skip — budget exhausted for %s/%s", pass, i, ref, provName, intent)
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

// modelKeys returns a comma-separated list of model keys for logging.
func modelKeys(models map[string]config.ModelConfig) string {
	keys := make([]string, 0, len(models))
	for k := range models {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
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

		if !r.budget.HasBudget(provName, "embed") {
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
			return &RouteResult{
				ProviderName: provName,
				Provider:     provider,
				Model:        mc.ModelName(modelKey),
				ModelKey:     modelKey,
				ModelConfig:  mc,
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
