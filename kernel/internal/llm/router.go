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
}

func NewRouter(providers map[string]Provider, configs map[string]config.ProviderConfig, routing config.RoutingConfig, budget *BudgetTracker) *Router {
	return &Router{
		providers: providers,
		configs:   configs,
		routing:   routing,
		budget:    budget,
	}
}

// Route picks the best provider for the given intent, budget, and tool requirements.
func (r *Router) Route(intent string, maxTokens int, needsTools bool) (*RouteResult, error) {
	preferences, ok := r.routing.IntentPreferences[intent]
	if !ok {
		preferences = make([]string, 0, len(r.providers))
		for name := range r.providers {
			preferences = append(preferences, name)
		}
	}

	for _, provName := range preferences {
		provider, ok := r.providers[provName]
		if !ok {
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

		modelKey, modelCfg, found := r.bestModelForIntent(provCfg, intent, needsTools)
		if !found {
			continue
		}

		estimated := EstimateCost(modelCfg, maxTokens)
		canAfford, _, err := r.budget.CanAfford(provName, estimated, intent)
		if err != nil || !canAfford {
			continue
		}

		return &RouteResult{
			ProviderName: provName,
			Provider:     provider,
			Model:        modelCfg.ModelName(modelKey),
			ModelKey:     modelKey,
			ModelConfig:  modelCfg,
		}, nil
	}

	return nil, fmt.Errorf("no affordable provider available for intent %q", intent)
}

// RouteEmbed picks a provider that supports embeddings.
func (r *Router) RouteEmbed() (*RouteResult, error) {
	preferences, ok := r.routing.IntentPreferences["embed"]
	if !ok {
		preferences = []string{"ollama", "openai"}
	}

	for _, provName := range preferences {
		provider, ok := r.providers[provName]
		if !ok || !provider.SupportsEmbed() {
			continue
		}

		provCfg := r.configs[provName]

		for modelKey, modelCfg := range provCfg.Models {
			if !hasCapability(modelCfg.Capabilities, "embed") {
				continue
			}
			canAfford, _, _ := r.budget.CanAfford(provName, EstimateCost(modelCfg, 100), "embed")
			if canAfford {
				return &RouteResult{
					ProviderName: provName,
					Provider:     provider,
					Model:        modelCfg.ModelName(modelKey),
					ModelKey:     modelKey,
					ModelConfig:  modelCfg,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no provider available for embeddings")
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
