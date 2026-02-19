package llm

import "sync"

// BusyTracker tracks concurrent requests per model to prevent overloading
// local models (e.g., Ollama on a single GPU).
type BusyTracker struct {
	mu       sync.Mutex
	inflight map[string]int // "provider:model_key" → count
	limits   map[string]int // "provider:model_key" → max concurrent
}

// NewBusyTracker creates a new tracker. Provider type determines default limits:
// "ollama" gets limit 1 (single GPU), cloud providers get 0 (unlimited).
func NewBusyTracker(providerTypes map[string]string) *BusyTracker {
	limits := make(map[string]int)
	for prov, ptype := range providerTypes {
		if ptype == "ollama" {
			// Ollama can only run one model inference at a time per model
			limits[prov] = 1
		}
		// Cloud providers (claude, openai) have no meaningful concurrency limit
	}
	return &BusyTracker{
		inflight: make(map[string]int),
		limits:   limits,
	}
}

func key(provider, modelKey string) string {
	return provider + ":" + modelKey
}

// Acquire tries to reserve a slot for a request. Returns true if the model is available.
func (bt *BusyTracker) Acquire(provider, modelKey string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	k := key(provider, modelKey)
	limit, hasLimit := bt.limits[provider]
	if !hasLimit || limit == 0 {
		// No limit — always available
		bt.inflight[k]++
		return true
	}

	if bt.inflight[k] >= limit {
		return false
	}
	bt.inflight[k]++
	return true
}

// Release frees a slot after a request completes.
func (bt *BusyTracker) Release(provider, modelKey string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	k := key(provider, modelKey)
	if bt.inflight[k] > 0 {
		bt.inflight[k]--
	}
}

// IsBusy returns true if a model has reached its concurrency limit.
func (bt *BusyTracker) IsBusy(provider, modelKey string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	k := key(provider, modelKey)
	limit, hasLimit := bt.limits[provider]
	if !hasLimit || limit == 0 {
		return false
	}
	return bt.inflight[k] >= limit
}
