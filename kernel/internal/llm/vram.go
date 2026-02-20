package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VRAMTracker polls Ollama's /api/ps to track loaded models and VRAM usage.
// Used by the Router to skip models that won't fit in GPU memory.
type VRAMTracker struct {
	mu        sync.RWMutex
	totalVRAM int64          // configured capacity (bytes)
	loaded    []LoadedModel  // from /api/ps polling
	usedVRAM  int64          // sum of loaded[].SizeVRAM
	inflight  map[string]int // model name → concurrent request count
	baseURL   string
	cancel    context.CancelFunc
}

// LoadedModel represents a model currently loaded in Ollama's GPU memory.
type LoadedModel struct {
	Name     string `json:"name"`
	SizeVRAM int64  `json:"size_vram"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewVRAMTracker creates a tracker that polls Ollama's /api/ps every 5 seconds.
// If totalVRAM is 0, VRAM checking is disabled (CanFit always returns true).
func NewVRAMTracker(baseURL string, totalVRAM int64) *VRAMTracker {
	ctx, cancel := context.WithCancel(context.Background())
	vt := &VRAMTracker{
		totalVRAM: totalVRAM,
		inflight:  make(map[string]int),
		baseURL:   baseURL,
		cancel:    cancel,
	}
	go vt.pollLoop(ctx)
	return vt
}

// CanFit returns true if the model can be loaded into VRAM.
// A model fits if: it's already loaded, OR free VRAM >= estimate, OR VRAM tracking is disabled.
func (vt *VRAMTracker) CanFit(modelName string, vramEstimate int64) bool {
	if vt.totalVRAM == 0 {
		return true // VRAM tracking disabled
	}
	if vramEstimate == 0 {
		log.Printf("[vram] CanFit(%q): no estimate, allowing", modelName)
		return true // no estimate available, allow it
	}

	vt.mu.RLock()
	defer vt.mu.RUnlock()

	// Already loaded — always fits
	for _, m := range vt.loaded {
		if m.Name == modelName {
			log.Printf("[vram] CanFit(%q): already loaded (%s), fits", modelName, formatBytes(m.SizeVRAM))
			return true
		}
	}

	// Check if there's enough free VRAM
	free := vt.totalVRAM - vt.usedVRAM
	fits := free >= vramEstimate
	if !fits {
		log.Printf("[vram] CanFit(%q): need %s but only %s free (%s used / %s total) — does not fit",
			modelName, formatBytes(vramEstimate), formatBytes(free), formatBytes(vt.usedVRAM), formatBytes(vt.totalVRAM))
	} else {
		log.Printf("[vram] CanFit(%q): need %s, %s free — fits", modelName, formatBytes(vramEstimate), formatBytes(free))
	}
	return fits
}

// IsLoaded returns true if the model is currently loaded in VRAM.
// Also checks ExpiresAt — if the model's keep-alive has expired, it may be
// about to be evicted so we don't count it as reliably loaded.
func (vt *VRAMTracker) IsLoaded(modelName string) bool {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	for _, m := range vt.loaded {
		if m.Name == modelName {
			if !m.ExpiresAt.IsZero() && time.Now().After(m.ExpiresAt) {
				log.Printf("[vram] model %q in loaded list but expired at %s — treating as not loaded",
					modelName, m.ExpiresAt.Format(time.RFC3339))
				return false
			}
			return true
		}
	}
	return false
}

// Acquire reserves a concurrency slot for a model. Max 1 per model.
func (vt *VRAMTracker) Acquire(modelName string) bool {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	if vt.inflight[modelName] >= 1 {
		return false
	}
	vt.inflight[modelName]++
	return true
}

// Release frees a concurrency slot for a model.
func (vt *VRAMTracker) Release(modelName string) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	if vt.inflight[modelName] > 0 {
		vt.inflight[modelName]--
	}
}

// IsBusy returns true if the model has a request in flight.
func (vt *VRAMTracker) IsBusy(modelName string) bool {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.inflight[modelName] >= 1
}

// GetStatus returns current VRAM state for debugging/UI.
func (vt *VRAMTracker) GetStatus() map[string]interface{} {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	models := make([]map[string]interface{}, len(vt.loaded))
	for i, m := range vt.loaded {
		models[i] = map[string]interface{}{
			"name":       m.Name,
			"size_vram":  m.SizeVRAM,
			"expires_at": m.ExpiresAt,
		}
	}

	return map[string]interface{}{
		"total_vram_bytes": vt.totalVRAM,
		"used_vram_bytes":  vt.usedVRAM,
		"free_vram_bytes":  vt.totalVRAM - vt.usedVRAM,
		"loaded_models":    models,
		"inflight":         vt.inflight,
	}
}

// Stop cancels the background polling goroutine.
func (vt *VRAMTracker) Stop() {
	vt.cancel()
}

func (vt *VRAMTracker) pollLoop(ctx context.Context) {
	// Initial poll
	vt.poll()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			vt.poll()
		}
	}
}

func (vt *VRAMTracker) poll() {
	url := vt.baseURL + "/api/ps"
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		// Ollama might be down — keep last known state
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var result struct {
		Models []struct {
			Name      string `json:"name"`
			SizeVRAM  int64  `json:"size_vram"`
			ExpiresAt string `json:"expires_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[vram] failed to decode /api/ps: %v", err)
		return
	}

	loaded := make([]LoadedModel, 0, len(result.Models))
	var usedVRAM int64
	for _, m := range result.Models {
		expires, _ := time.Parse(time.RFC3339Nano, m.ExpiresAt)
		loaded = append(loaded, LoadedModel{
			Name:      m.Name,
			SizeVRAM:  m.SizeVRAM,
			ExpiresAt: expires,
		})
		usedVRAM += m.SizeVRAM
	}

	vt.mu.Lock()
	vt.loaded = loaded
	vt.usedVRAM = usedVRAM
	vt.mu.Unlock()

	if len(loaded) > 0 {
		names := make([]string, len(loaded))
		for i, m := range loaded {
			names[i] = fmt.Sprintf("%s(%s, expires=%s)", m.Name, formatBytes(m.SizeVRAM), m.ExpiresAt.Format(time.RFC3339))
		}
		log.Printf("[vram] polled: %d models loaded [%s], %s / %s used",
			len(loaded), strings.Join(names, ", "), formatBytes(usedVRAM), formatBytes(vt.totalVRAM))
	}
}

func formatBytes(b int64) string {
	const gb = 1024 * 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("%.0f MB", float64(b)/float64(mb))
}
