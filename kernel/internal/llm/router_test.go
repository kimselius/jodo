package llm

import (
	"context"
	"testing"

	"jodo-kernel/internal/config"
)

// stubProvider implements Provider for testing.
type stubProvider struct {
	name    string
	canEmbed bool
}

func (s *stubProvider) Name() string       { return s.name }
func (s *stubProvider) SupportsEmbed() bool { return s.canEmbed }
func (s *stubProvider) BuildRequest(_ *JodoRequest, _ string) (*ProviderHTTPRequest, error) {
	return nil, nil
}
func (s *stubProvider) ParseResponse(_ int, _ []byte) (*ProviderHTTPResponse, error) {
	return nil, nil
}
func (s *stubProvider) Embed(_ context.Context, _ string, _ string) ([]float32, int, error) {
	return nil, 0, nil
}

// stubBudget always returns true for HasBudget (unlimited budget).
type stubBudget struct{}

func newUnlimitedBudget() *BudgetTracker {
	return &BudgetTracker{
		db:        nil,
		providers: map[string]config.ProviderConfig{
			"cloud":  {MonthlyBudget: 0, EmergencyReserve: 0},
			"ollama": {MonthlyBudget: 0, EmergencyReserve: 0},
		},
	}
}

func TestRouteByIntent(t *testing.T) {
	providers := map[string]Provider{
		"cloud": &stubProvider{name: "cloud"},
	}
	configs := map[string]config.ProviderConfig{
		"cloud": {
			Models: map[string]config.ModelConfig{
				"gpt-4": {
					Capabilities: []string{"chat", "code", "tools"},
					Quality:      90,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"chat": {"cloud"},
			"code": {"cloud"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"cloud": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.Route("chat")
	if err != nil {
		t.Fatalf("Route(chat) failed: %v", err)
	}
	if result.ProviderName != "cloud" {
		t.Errorf("expected provider cloud, got %s", result.ProviderName)
	}
	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.Model)
	}
}

func TestRouteNoViableProvider(t *testing.T) {
	providers := map[string]Provider{
		"cloud": &stubProvider{name: "cloud"},
	}
	configs := map[string]config.ProviderConfig{
		"cloud": {
			Models: map[string]config.ModelConfig{
				"gpt-4": {
					Capabilities: []string{"chat"},
					Quality:      90,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"embed": {"cloud"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"cloud": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	_, err := router.Route("embed")
	if err == nil {
		t.Fatal("expected error for no viable provider, got nil")
	}
}

func TestRouteBestModelByQuality(t *testing.T) {
	providers := map[string]Provider{
		"cloud": &stubProvider{name: "cloud"},
	}
	configs := map[string]config.ProviderConfig{
		"cloud": {
			Models: map[string]config.ModelConfig{
				"low-q": {
					Capabilities: []string{"chat", "tools"},
					Quality:      50,
				},
				"high-q": {
					Capabilities: []string{"chat", "tools"},
					Quality:      95,
				},
				"mid-q": {
					Capabilities: []string{"chat", "tools"},
					Quality:      75,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"chat": {"cloud"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"cloud": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.Route("chat")
	if err != nil {
		t.Fatalf("Route(chat) failed: %v", err)
	}
	if result.ModelKey != "high-q" {
		t.Errorf("expected highest quality model high-q, got %s", result.ModelKey)
	}
}

func TestRouteModelAtProvider(t *testing.T) {
	providers := map[string]Provider{
		"cloud": &stubProvider{name: "cloud"},
	}
	configs := map[string]config.ProviderConfig{
		"cloud": {
			Models: map[string]config.ModelConfig{
				"gpt-4": {
					Capabilities: []string{"chat", "tools"},
					Quality:      90,
				},
				"gpt-3": {
					Capabilities: []string{"chat"},
					Quality:      60,
				},
			},
		},
	}
	// Use explicit model@provider reference
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"chat": {"gpt-3@cloud"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"cloud": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.Route("chat")
	if err != nil {
		t.Fatalf("Route(chat) failed: %v", err)
	}
	// Should use explicitly requested gpt-3, not the higher quality gpt-4
	if result.ModelKey != "gpt-3" {
		t.Errorf("expected gpt-3 from explicit ref, got %s", result.ModelKey)
	}
}

func TestRouteEmbedPicksEmbedCapableProvider(t *testing.T) {
	providers := map[string]Provider{
		"cloud":  &stubProvider{name: "cloud", canEmbed: true},
		"noEmbed": &stubProvider{name: "noEmbed", canEmbed: false},
	}
	configs := map[string]config.ProviderConfig{
		"cloud": {
			Models: map[string]config.ModelConfig{
				"text-embed": {
					Capabilities: []string{"embed"},
					Quality:      60,
				},
			},
		},
		"noEmbed": {
			Models: map[string]config.ModelConfig{
				"llm": {
					Capabilities: []string{"chat"},
					Quality:      90,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"embed": {"noEmbed", "cloud"},
		},
	}

	budget := newUnlimitedBudget()
	budget.providers["noEmbed"] = config.ProviderConfig{}
	busy := NewBusyTracker(map[string]string{"cloud": "openai", "noEmbed": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.RouteEmbed()
	if err != nil {
		t.Fatalf("RouteEmbed failed: %v", err)
	}
	if result.ProviderName != "cloud" {
		t.Errorf("expected cloud (supports embed), got %s", result.ProviderName)
	}
}

func TestRouteRespectsPreferenceOrder(t *testing.T) {
	// Two Ollama models for "code" intent: model-a is #1, model-b is #2.
	// Without VRAM constraints, model-a MUST be selected.
	providers := map[string]Provider{
		"ollama": &stubProvider{name: "ollama"},
	}
	configs := map[string]config.ProviderConfig{
		"ollama": {
			Models: map[string]config.ModelConfig{
				"model-a": {
					Capabilities: []string{"code", "plan", "chat"},
					Quality:      80,
				},
				"model-b": {
					Capabilities: []string{"code", "plan", "chat"},
					Quality:      90, // higher quality but lower preference
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"code": {"model-a@ollama", "model-b@ollama"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"ollama": "ollama"})
	// No VRAM tracker — both models are viable
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.Route("code")
	if err != nil {
		t.Fatalf("Route(code) failed: %v", err)
	}
	if result.ModelKey != "model-a" {
		t.Errorf("expected model-a (preference #1), got %s", result.ModelKey)
	}
}

func TestRoutePreferenceOrderWithVRAM(t *testing.T) {
	// Two Ollama models: model-a is #1, model-b is #2.
	// Neither is loaded in VRAM. model-a should still be selected in pass2.
	providers := map[string]Provider{
		"ollama": &stubProvider{name: "ollama"},
	}
	configs := map[string]config.ProviderConfig{
		"ollama": {
			TotalVRAMBytes: 48 * 1024 * 1024 * 1024, // 48 GB
			Models: map[string]config.ModelConfig{
				"model-a": {
					Capabilities:  []string{"code", "plan", "chat"},
					Quality:       80,
					VRAMEstimateBytes: 12 * 1024 * 1024 * 1024, // 12 GB
					PreferLoaded:  true,
				},
				"model-b": {
					Capabilities:  []string{"code", "plan", "chat"},
					Quality:       90,
					VRAMEstimateBytes: 8 * 1024 * 1024 * 1024, // 8 GB
					PreferLoaded:  true,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"code": {"model-a@ollama", "model-b@ollama"},
		},
	}

	budget := newUnlimitedBudget()
	busy := NewBusyTracker(map[string]string{"ollama": "ollama"})
	// VRAM tracker with nothing loaded — use a stub that reports empty
	vram := &VRAMTracker{totalVRAM: 48 * 1024 * 1024 * 1024}
	router := NewRouter(providers, configs, routing, budget, busy, vram)

	result, err := router.Route("code")
	if err != nil {
		t.Fatalf("Route(code) failed: %v", err)
	}
	// Pass 1 should find nothing (nothing loaded).
	// Pass 2 should pick model-a (preference #1, fits in VRAM).
	if result.ModelKey != "model-a" {
		t.Errorf("expected model-a (preference #1), got %s — preference order not respected", result.ModelKey)
	}
}

func TestRouteFallsThrough(t *testing.T) {
	// First provider has no model for intent, should fall through to second
	providers := map[string]Provider{
		"a": &stubProvider{name: "a"},
		"b": &stubProvider{name: "b"},
	}
	configs := map[string]config.ProviderConfig{
		"a": {
			Models: map[string]config.ModelConfig{
				"embed-only": {
					Capabilities: []string{"embed"},
					Quality:      60,
				},
			},
		},
		"b": {
			Models: map[string]config.ModelConfig{
				"chat-model": {
					Capabilities: []string{"chat"},
					Quality:      80,
				},
			},
		},
	}
	routing := config.RoutingConfig{
		IntentPreferences: map[string][]string{
			"chat": {"a", "b"},
		},
	}

	budget := newUnlimitedBudget()
	budget.providers["a"] = config.ProviderConfig{}
	budget.providers["b"] = config.ProviderConfig{}
	busy := NewBusyTracker(map[string]string{"a": "ollama", "b": "openai"})
	router := NewRouter(providers, configs, routing, budget, busy, nil)

	result, err := router.Route("chat")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if result.ProviderName != "b" {
		t.Errorf("expected fallthrough to b, got %s", result.ProviderName)
	}
}
