package llm

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"jodo-kernel/internal/config"
)

// BudgetTracker manages per-provider spending limits.
type BudgetTracker struct {
	db        *sql.DB
	providers map[string]config.ProviderConfig
}

func NewBudgetTracker(db *sql.DB, providers map[string]config.ProviderConfig) *BudgetTracker {
	return &BudgetTracker{db: db, providers: providers}
}

// SpentThisMonth returns total spent for a provider in the current billing period.
func (b *BudgetTracker) SpentThisMonth(provider string) (float64, error) {
	firstOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)

	var spent sql.NullFloat64
	err := b.db.QueryRow(
		`SELECT COALESCE(SUM(cost), 0) FROM budget_usage WHERE provider = $1 AND created_at >= $2`,
		provider, firstOfMonth,
	).Scan(&spent)
	if err != nil {
		return 0, fmt.Errorf("query budget: %w", err)
	}
	return spent.Float64, nil
}

// HasBudget returns true if the provider still has budget remaining this month.
// For "repair" intent, the emergency reserve is also available.
func (b *BudgetTracker) HasBudget(provider string, intent string) bool {
	cfg, ok := b.providers[provider]
	if !ok {
		return false
	}

	// No budget configured = unlimited (e.g. Ollama)
	if cfg.MonthlyBudget == 0 && cfg.EmergencyReserve == 0 {
		return true
	}

	spent, err := b.SpentThisMonth(provider)
	if err != nil {
		return false
	}

	remaining := cfg.MonthlyBudget - spent
	if intent == "repair" {
		return remaining > 0
	}
	return remaining-cfg.EmergencyReserve > 0
}

// LogUsage records a completed API call's cost.
func (b *BudgetTracker) LogUsage(provider, model, intent string, tokensIn, tokensOut int, cost float64) error {
	_, err := b.db.Exec(
		`INSERT INTO budget_usage (provider, model, intent, tokens_in, tokens_out, cost) VALUES ($1, $2, $3, $4, $5, $6)`,
		provider, model, intent, tokensIn, tokensOut, cost,
	)
	return err
}

// CalculateCost computes the cost for a given token count and model config.
func CalculateCost(modelCfg config.ModelConfig, tokensIn, tokensOut int) float64 {
	inCost := float64(tokensIn) * modelCfg.InputCostPer1MTokens / 1_000_000
	outCost := float64(tokensOut) * modelCfg.OutputCostPer1MTokens / 1_000_000
	return inCost + outCost
}

// GetAllBudgetStatus returns budget status for all providers.
func (b *BudgetTracker) GetAllBudgetStatus() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for name, cfg := range b.providers {
		if cfg.MonthlyBudget == 0 && cfg.EmergencyReserve == 0 {
			result[name] = map[string]interface{}{
				"monthly_budget": 0,
				"remaining":      "unlimited",
			}
			continue
		}

		spent, err := b.SpentThisMonth(name)
		if err != nil {
			return nil, err
		}

		remaining := cfg.MonthlyBudget - spent
		available := remaining - cfg.EmergencyReserve
		if available < 0 {
			available = 0
		}

		result[name] = BudgetStatus{
			MonthlyBudget:      cfg.MonthlyBudget,
			SpentThisMonth:     spent,
			Remaining:          remaining,
			EmergencyReserve:   cfg.EmergencyReserve,
			AvailableForNormal: available,
		}
	}

	return result, nil
}

// TotalSpentToday returns total cost across all providers today.
func (b *BudgetTracker) TotalSpentToday() (float64, error) {
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)

	var spent sql.NullFloat64
	err := b.db.QueryRow(
		`SELECT COALESCE(SUM(cost), 0) FROM budget_usage WHERE created_at >= $1`,
		startOfDay,
	).Scan(&spent)
	if err != nil {
		return 0, err
	}
	return spent.Float64, nil
}

// ---------- Chain Cost Tracker ----------

// ChainTracker tracks cumulative cost across multi-turn tool loops.
type ChainTracker struct {
	mu     sync.Mutex
	chains map[string]*chainEntry
}

type chainEntry struct {
	cost    float64
	lastUse time.Time
}

func NewChainTracker() *ChainTracker {
	ct := &ChainTracker{chains: make(map[string]*chainEntry)}
	go ct.cleanup()
	return ct
}

// AddCost adds cost to a chain and returns the new cumulative total.
func (ct *ChainTracker) AddCost(chainID string, cost float64) float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	entry, ok := ct.chains[chainID]
	if !ok {
		entry = &chainEntry{}
		ct.chains[chainID] = entry
	}
	entry.cost += cost
	entry.lastUse = time.Now()
	return entry.cost
}

// GetCost returns the cumulative cost for a chain.
func (ct *ChainTracker) GetCost(chainID string) float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if entry, ok := ct.chains[chainID]; ok {
		return entry.cost
	}
	return 0
}

// cleanup periodically removes expired chains (older than 1 hour).
func (ct *ChainTracker) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		ct.mu.Lock()
		for id, entry := range ct.chains {
			if time.Since(entry.lastUse) > 1*time.Hour {
				delete(ct.chains, id)
			}
		}
		ct.mu.Unlock()
	}
}
