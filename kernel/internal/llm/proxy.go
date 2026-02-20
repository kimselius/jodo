package llm

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
)

// Proxy is the main LLM gateway. Jodo calls this instead of calling providers directly.
// It routes requests, translates formats, enforces budgets, and tracks chain costs.
type Proxy struct {
	mu      sync.RWMutex
	Router  *Router
	Budget  *BudgetTracker
	Busy    *BusyTracker
	VRAM    *VRAMTracker
	Chains  *ChainTracker
	Audit   *audit.Logger
	DB      *sql.DB
	client  *http.Client
	configs map[string]config.ProviderConfig
}

// proxySubsystems holds the wired-up subsystems built from config.
type proxySubsystems struct {
	router *Router
	budget *BudgetTracker
	busy   *BusyTracker
	vram   *VRAMTracker
}

// buildSubsystems creates providers, trackers, and router from config.
// Single source of truth for wiring — used by NewProxy and Reconfigure.
func buildSubsystems(db *sql.DB, providerConfigs map[string]config.ProviderConfig, routing config.RoutingConfig) proxySubsystems {
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

	providerTypes := make(map[string]string)
	for name := range providerConfigs {
		providerTypes[name] = name
	}
	busy := NewBusyTracker(providerTypes)

	var vram *VRAMTracker
	if ollamaCfg, ok := providerConfigs["ollama"]; ok && ollamaCfg.TotalVRAMBytes > 0 {
		vram = NewVRAMTracker(ollamaCfg.BaseURL, ollamaCfg.TotalVRAMBytes)
		log.Printf("[llm] VRAM tracker enabled: %s total", formatBytes(ollamaCfg.TotalVRAMBytes))
	}

	router := NewRouter(providers, providerConfigs, routing, budget, busy, vram)

	return proxySubsystems{router: router, budget: budget, busy: busy, vram: vram}
}

// NewProxy creates a fully wired LLM proxy from config.
func NewProxy(db *sql.DB, providerConfigs map[string]config.ProviderConfig, routing config.RoutingConfig) *Proxy {
	s := buildSubsystems(db, providerConfigs, routing)
	return &Proxy{
		Router:  s.router,
		Budget:  s.budget,
		Busy:    s.busy,
		VRAM:    s.vram,
		Chains:  NewChainTracker(),
		DB:      db,
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

	// Snapshot router under read lock — released before HTTP call
	p.mu.RLock()
	router := p.Router
	budget := p.Budget
	vram := p.VRAM
	busy := p.Busy
	p.mu.RUnlock()

	// Route to best affordable provider
	route, err := router.Route(req.Intent)
	if err != nil {
		return nil, err
	}

	log.Printf("[llm] routing %q intent to %s/%s", req.Intent, route.ProviderName, route.Model)

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
	if vram != nil && route.ProviderName == "ollama" {
		if !vram.Acquire(route.Model) {
			return nil, fmt.Errorf("model %s/%s is busy", route.ProviderName, route.Model)
		}
		defer vram.Release(route.Model)
	} else if busy != nil {
		if !busy.Acquire(route.ProviderName, route.ModelKey) {
			return nil, fmt.Errorf("model %s/%s is busy", route.ProviderName, route.ModelKey)
		}
		defer busy.Release(route.ProviderName, route.ModelKey)
	}

	// Build provider-specific HTTP request
	provReq, err := route.Provider.BuildRequest(req, route.Model)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// Execute HTTP call with retry
	respBody, statusCode, err := p.executeWithRetry(ctx, provReq, route.ProviderName)
	if err != nil {
		return nil, err
	}

	// Parse provider-specific response into Jodo Format
	provResp, err := route.Provider.ParseResponse(statusCode, respBody)
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
		p.logCallToDB(req, route, start, nil, err)
		return nil, err
	}

	// Calculate actual cost
	cost := CalculateCost(route.ModelConfig, provResp.TokensIn, provResp.TokensOut)

	// Log usage
	if logErr := budget.LogUsage(route.ProviderName, route.Model, req.Intent, provResp.TokensIn, provResp.TokensOut, cost); logErr != nil {
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

	// Log successful call to llm_calls table
	p.logCallToDB(req, route, start, provResp, nil)

	// Get budget remaining
	budgetRemaining, _ := budget.GetAllBudgetStatus()

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

// executeWithRetry performs the HTTP call with exponential backoff for transient errors (429, 529, etc.).
func (p *Proxy) executeWithRetry(ctx context.Context, provReq *ProviderHTTPRequest, providerName string) ([]byte, int, error) {
	backoff := time.Second
	const maxRetries = 3

	for attempt := 0; ; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", provReq.URL, bytes.NewReader(provReq.Body))
		if err != nil {
			return nil, 0, fmt.Errorf("create http request: %w", err)
		}
		for k, v := range provReq.Headers {
			httpReq.Header.Set(k, v)
		}

		resp, err := p.client.Do(httpReq)
		if err != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				log.Printf("[llm] request to %s failed: %v, retrying in %v (%d/%d)", providerName, err, backoff, attempt+1, maxRetries)
				select {
				case <-time.After(backoff):
					backoff *= 2
					continue
				case <-ctx.Done():
					return nil, 0, ctx.Err()
				}
			}
			return nil, 0, fmt.Errorf("http request to %s: %w", providerName, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if !isRetryableStatus(resp.StatusCode) || attempt >= maxRetries {
			return body, resp.StatusCode, nil
		}

		log.Printf("[llm] %s returned %d, retrying in %v (%d/%d)", providerName, resp.StatusCode, backoff, attempt+1, maxRetries)
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
}

// logCallToDB records a Think call (success or error) to the llm_calls table.
func (p *Proxy) logCallToDB(req *JodoRequest, route *RouteResult, start time.Time, resp *ProviderHTTPResponse, callErr error) {
	if p.DB == nil {
		return
	}

	reqMsgsJSON, _ := json.Marshal(req.Messages)
	reqToolsJSON, _ := json.Marshal(req.Tools)
	chainID := sql.NullString{String: req.ChainID, Valid: req.ChainID != ""}
	durationMs := time.Since(start).Milliseconds()

	if callErr != nil {
		p.DB.Exec(
			`INSERT INTO llm_calls (intent, provider, model, duration_ms, chain_id, request_system, request_messages, request_tools, error)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			req.Intent, route.ProviderName, route.Model,
			durationMs, chainID,
			req.System, reqMsgsJSON, reqToolsJSON, callErr.Error(),
		)
		return
	}

	respToolCallsJSON, _ := json.Marshal(resp.ToolCalls)
	if _, dbErr := p.DB.Exec(
		`INSERT INTO llm_calls (intent, provider, model, tokens_in, tokens_out, cost, duration_ms, chain_id, request_system, request_messages, request_tools, response_content, response_tool_calls, response_done)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		req.Intent, route.ProviderName, route.Model,
		resp.TokensIn, resp.TokensOut, CalculateCost(route.ModelConfig, resp.TokensIn, resp.TokensOut),
		durationMs, chainID,
		req.System, reqMsgsJSON, reqToolsJSON,
		resp.Content, respToolCallsJSON, resp.Done,
	); dbErr != nil {
		log.Printf("[llm] failed to log call to DB: %v", dbErr)
	}
}

// Reconfigure swaps the provider and routing config without restarting the kernel.
// This is called when settings are changed via the UI.
func (p *Proxy) Reconfigure(providerConfigs map[string]config.ProviderConfig, routing config.RoutingConfig) {
	s := buildSubsystems(p.DB, providerConfigs, routing)

	p.mu.Lock()
	oldVRAM := p.VRAM
	p.Router = s.router
	p.Budget = s.budget
	p.Busy = s.busy
	p.VRAM = s.vram
	p.configs = providerConfigs
	p.mu.Unlock()

	if oldVRAM != nil {
		oldVRAM.Stop()
	}

	log.Printf("[llm] reconfigured with %d providers", len(s.router.providers))
}

// Embed generates an embedding vector for the given text.
func (p *Proxy) Embed(ctx context.Context, text string) (*EmbedResponse, error) {
	p.mu.RLock()
	router := p.Router
	budget := p.Budget
	p.mu.RUnlock()

	route, err := router.RouteEmbed()
	if err != nil {
		return nil, err
	}

	embedding, tokensIn, err := route.Provider.Embed(ctx, route.Model, text)
	if err != nil {
		return nil, err
	}

	cost := float64(tokensIn) * route.ModelConfig.InputCostPer1MTokens / 1_000_000

	if logErr := budget.LogUsage(route.ProviderName, route.Model, "embed", tokensIn, 0, cost); logErr != nil {
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

// isRetryableStatus returns true for HTTP status codes that indicate transient errors
// worth retrying: rate limits, server overload, bad gateway.
func isRetryableStatus(code int) bool {
	switch code {
	case 429, // rate limited
		502, // bad gateway
		503, // service unavailable
		529: // overloaded (Anthropic-specific)
		return true
	}
	return false
}
