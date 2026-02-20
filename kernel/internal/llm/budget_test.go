package llm

import (
	"testing"

	"jodo-kernel/internal/config"
)

func TestCalculateCost(t *testing.T) {
	mc := config.ModelConfig{
		InputCostPer1MTokens:  3.0,  // $3 per 1M input tokens
		OutputCostPer1MTokens: 15.0, // $15 per 1M output tokens
	}

	cost := CalculateCost(mc, 1000, 500)

	// Expected: (1000 * 3.0 / 1M) + (500 * 15.0 / 1M) = 0.003 + 0.0075 = 0.0105
	expected := 0.0105
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Errorf("expected cost ~%.4f, got %.4f", expected, cost)
	}
}

func TestCalculateCostZeroTokens(t *testing.T) {
	mc := config.ModelConfig{
		InputCostPer1MTokens:  3.0,
		OutputCostPer1MTokens: 15.0,
	}

	cost := CalculateCost(mc, 0, 0)
	if cost != 0 {
		t.Errorf("expected 0 cost for 0 tokens, got %f", cost)
	}
}

func TestCalculateCostFreeModel(t *testing.T) {
	mc := config.ModelConfig{
		InputCostPer1MTokens:  0,
		OutputCostPer1MTokens: 0,
	}

	cost := CalculateCost(mc, 10000, 5000)
	if cost != 0 {
		t.Errorf("expected 0 cost for free model, got %f", cost)
	}
}

func TestChainTrackerAddAndGet(t *testing.T) {
	ct := &ChainTracker{chains: make(map[string]*chainEntry)}

	total := ct.AddCost("chain-1", 0.05)
	if total != 0.05 {
		t.Errorf("expected 0.05, got %f", total)
	}

	total = ct.AddCost("chain-1", 0.03)
	if total < 0.0799 || total > 0.0801 {
		t.Errorf("expected ~0.08, got %f", total)
	}

	got := ct.GetCost("chain-1")
	if got < 0.0799 || got > 0.0801 {
		t.Errorf("expected ~0.08, got %f", got)
	}
}

func TestChainTrackerIndependentChains(t *testing.T) {
	ct := &ChainTracker{chains: make(map[string]*chainEntry)}

	ct.AddCost("a", 1.0)
	ct.AddCost("b", 2.0)

	if ct.GetCost("a") != 1.0 {
		t.Errorf("chain a: expected 1.0, got %f", ct.GetCost("a"))
	}
	if ct.GetCost("b") != 2.0 {
		t.Errorf("chain b: expected 2.0, got %f", ct.GetCost("b"))
	}
}

func TestChainTrackerUnknownChain(t *testing.T) {
	ct := &ChainTracker{chains: make(map[string]*chainEntry)}

	cost := ct.GetCost("nonexistent")
	if cost != 0 {
		t.Errorf("expected 0 for unknown chain, got %f", cost)
	}
}
