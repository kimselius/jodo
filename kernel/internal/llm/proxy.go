package llm

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
)

// Proxy is the main LLM gateway. Jodo calls this instead of calling providers directly.
// It routes requests, translates formats, enforces budgets, and tracks chain costs.
type Proxy struct {
	Router  *Router
	Budget  *BudgetTracker
	Busy    *BusyTracker
	VRAM    *VRAMTracker
	Chains  *ChainTracker
	Audit   *audit.Logger
	client  *http.Client
	configs map[string]config.ProviderConfig
}

// NewProxy creates a fully wired LLM proxy from config.
func NewProxy(db *sql.DB, providerConfigs map[string]config.ProviderConfig, routing config.RoutingConfig) *Proxy {
	providers := make(map[string]Provider)

	for name, cfg := range providerConfigs {
		switch name {
		case "claude":
			if cfg.APIKey != "" {
				providers[name] = NewClaudeProvider(cfg)
			}
		case "openai":
			if cfg.APIKey != "" {
				providers[name] = NewOpenAIProvider(cfg)
			}
		case "ollama":
			providers[name] = NewOllamaProvider(cfg)
		default:
			log.Printf("[llm] unknown provider %q, skipping", name)
		}
	}

	budget := NewBudgetTracker(db, providerConfigs)

	// Build provider type map for busy tracking
	providerTypes := make(map[string]string)
	for name := range providerConfigs {
		providerTypes[name] = name // provider name is the type (ollama, claude, openai)
	}
	busy := NewBusyTracker(providerTypes)

	// Create VRAM tracker for Ollama if VRAM capacity is configured
	var vram *VRAMTracker
	if ollamaCfg, ok := providerConfigs["ollama"]; ok && ollamaCfg.TotalVRAMBytes > 0 {
		vram = NewVRAMTracker(ollamaCfg.BaseURL, ollamaCfg.TotalVRAMBytes)
		log.Printf("[llm] VRAM tracker enabled: %s total", formatBytes(ollamaCfg.TotalVRAMBytes))
	}

	router := NewRouter(providers, providerConfigs, routing, budget, busy, vram)

	return &Proxy{
		Router:  router,
		Budget:  budget,
		Busy:    busy,
		VRAM:    vram,
		Chains:  NewChainTracker(),
		client:  &http.Client{Timeout: 120 * time.Second},
		configs: providerConfigs,
	}
}

// Think handles a chat/tool completion request with budget-aware routing.
// The proxy:
//  1. Checks chain budget if chain_id is set
//  2. Routes to the best affordable provider
//  3. Transforms the request to provider format (via BuildRequest)
//  4. Executes the HTTP call
//  5. Transforms the response back to Jodo Format (via ParseResponse)
//  6. Calculates cost, logs usage, tracks chain cost
func (p *Proxy) Think(ctx context.Context, req *JodoRequest) (*JodoResponse, error) {
	// Defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 1000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.Intent == "" {
		req.Intent = "chat"
	}

	// Chain budget check
	if req.ChainID != "" && req.MaxCost > 0 {
		currentCost := p.Chains.GetCost(req.ChainID)
		if currentCost >= req.MaxCost {
			return &JodoResponse{
				Content:        fmt.Sprintf("This thought chain has cost $%.4f, exceeding max_cost of $%.2f.", currentCost, req.MaxCost),
				Done:           true,
				TotalChainCost: currentCost,
			}, nil
		}
	}

	// Route to best affordable provider
	needsTools := len(req.Tools) > 0
	route, err := p.Router.Route(req.Intent, req.MaxTokens, needsTools)
	if err != nil {
		return nil, err
	}

	log.Printf("[llm] routing %q intent to %s/%s (tools=%v)", req.Intent, route.ProviderName, route.Model, needsTools)

	// Audit: log the incoming request
	start := time.Now()
	if p.Audit != nil {
		p.Audit.Log(audit.Entry{
			Event:  "think_request",
			Intent: req.Intent,
			Data: audit.ThinkRequest{
				Intent:     req.Intent,
				System:     req.System,
				Messages:   req.Messages,
				Tools:      req.Tools,
				ToolChoice: req.ToolChoice,
				MaxTokens:  req.MaxTokens,
				ChainID:    req.ChainID,
				MaxCost:    req.MaxCost,
			},
		})
	}

	// Acquire concurrency slot (prevents overloading local models)
	if p.VRAM != nil && route.ProviderName == "ollama" {
		// Use VRAM tracker for Ollama (per-model concurrency)
		if !p.VRAM.Acquire(route.Model) {
			return nil, fmt.Errorf("model %s/%s is busy", route.ProviderName, route.Model)
		}
		defer p.VRAM.Release(route.Model)
	} else if p.Busy != nil {
		if !p.Busy.Acquire(route.ProviderName, route.ModelKey) {
			return nil, fmt.Errorf("model %s/%s is busy", route.ProviderName, route.ModelKey)
		}
		defer p.Busy.Release(route.ProviderName, route.ModelKey)
	}

	// Build provider-specific HTTP request
	provReq, err := route.Provider.BuildRequest(req, route.Model)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// Execute HTTP call
	httpReq, err := http.NewRequestWithContext(ctx, "POST", provReq.URL, bytes.NewReader(provReq.Body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	for k, v := range provReq.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request to %s: %w", route.ProviderName, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// Parse provider-specific response into Jodo Format
	provResp, err := route.Provider.ParseResponse(resp.StatusCode, respBody)
	if err != nil {
		if p.Audit != nil {
			p.Audit.Log(audit.Entry{
				Event:    "think_error",
				Intent:   req.Intent,
				Provider: route.ProviderName,
				Model:    route.Model,
				Duration: time.Since(start).String(),
				Error:    err.Error(),
			})
		}
		return nil, err
	}

	// Calculate actual cost
	cost := CalculateCost(route.ModelConfig, provResp.TokensIn, provResp.TokensOut)

	// Log usage
	if logErr := p.Budget.LogUsage(route.ProviderName, route.Model, req.Intent, provResp.TokensIn, provResp.TokensOut, cost); logErr != nil {
		log.Printf("[llm] failed to log usage: %v", logErr)
	}

	// Track chain cost
	var totalChainCost float64
	if req.ChainID != "" {
		totalChainCost = p.Chains.AddCost(req.ChainID, cost)
	} else {
		totalChainCost = cost
	}

	// Audit: log the response
	if p.Audit != nil {
		p.Audit.Log(audit.Entry{
			Event:     "think_response",
			Intent:    req.Intent,
			Provider:  route.ProviderName,
			Model:     route.Model,
			TokensIn:  provResp.TokensIn,
			TokensOut: provResp.TokensOut,
			Cost:      cost,
			Duration:  time.Since(start).String(),
			Data: audit.ThinkResponse{
				Content:   provResp.Content,
				ToolCalls: provResp.ToolCalls,
				Done:      provResp.Done,
			},
		})
	}

	// Get budget remaining
	budgetRemaining, _ := p.Budget.GetAllBudgetStatus()

	return &JodoResponse{
		Content:         provResp.Content,
		ToolCalls:       provResp.ToolCalls,
		Done:            provResp.Done,
		ModelUsed:       route.Model,
		Provider:        route.ProviderName,
		TokensIn:        provResp.TokensIn,
		TokensOut:       provResp.TokensOut,
		Cost:            cost,
		TotalChainCost:  totalChainCost,
		BudgetRemaining: budgetRemaining,
	}, nil
}

// Reconfigure swaps the provider and routing config without restarting the kernel.
// This is called when settings are changed via the UI.
func (p *Proxy) Reconfigure(providerConfigs map[string]config.ProviderConfig, routing config.RoutingConfig) {
	// Stop old VRAM tracker if it exists
	if p.VRAM != nil {
		p.VRAM.Stop()
	}

	providers := make(map[string]Provider)
	for name, cfg := range providerConfigs {
		switch name {
		case "claude":
			if cfg.APIKey != "" {
				providers[name] = NewClaudeProvider(cfg)
			}
		case "openai":
			if cfg.APIKey != "" {
				providers[name] = NewOpenAIProvider(cfg)
			}
		case "ollama":
			providers[name] = NewOllamaProvider(cfg)
		}
	}

	budget := NewBudgetTracker(p.Budget.db, providerConfigs)

	providerTypes := make(map[string]string)
	for name := range providerConfigs {
		providerTypes[name] = name
	}
	busy := NewBusyTracker(providerTypes)

	// Create new VRAM tracker if Ollama has VRAM configured
	var vram *VRAMTracker
	if ollamaCfg, ok := providerConfigs["ollama"]; ok && ollamaCfg.TotalVRAMBytes > 0 {
		vram = NewVRAMTracker(ollamaCfg.BaseURL, ollamaCfg.TotalVRAMBytes)
		log.Printf("[llm] VRAM tracker enabled: %s total", formatBytes(ollamaCfg.TotalVRAMBytes))
	}

	router := NewRouter(providers, providerConfigs, routing, budget, busy, vram)

	p.Router = router
	p.Budget = budget
	p.Busy = busy
	p.VRAM = vram
	p.configs = providerConfigs

	log.Printf("[llm] reconfigured with %d providers", len(providers))
}

// Embed generates an embedding vector for the given text.
func (p *Proxy) Embed(ctx context.Context, text string) (*EmbedResponse, error) {
	route, err := p.Router.RouteEmbed()
	if err != nil {
		return nil, err
	}

	embedding, tokensIn, err := route.Provider.Embed(ctx, route.Model, text)
	if err != nil {
		return nil, err
	}

	cost := float64(tokensIn) * route.ModelConfig.InputCostPer1MTokens / 1_000_000

	if logErr := p.Budget.LogUsage(route.ProviderName, route.Model, "embed", tokensIn, 0, cost); logErr != nil {
		log.Printf("[llm] failed to log embed usage: %v", logErr)
	}

	return &EmbedResponse{
		Embedding: embedding,
		Model:     route.Model,
		Provider:  route.ProviderName,
		TokensIn:  tokensIn,
		Cost:      cost,
	}, nil
}
