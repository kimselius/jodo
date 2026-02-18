package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"jodo-kernel/internal/config"
)

// HealthChecker periodically pings Jodo's /health endpoint.
type HealthChecker struct {
	cfg        config.JodoConfig
	kernelCfg  config.KernelConfig
	manager    *Manager
	db         *sql.DB
	client     *http.Client
	failCount  atomic.Int32
	onEscalate func(failCount int) // callback for recovery escalation
	cancel     context.CancelFunc
}

func NewHealthChecker(cfg config.JodoConfig, kernelCfg config.KernelConfig, manager *Manager, db *sql.DB) *HealthChecker {
	return &HealthChecker{
		cfg:       cfg,
		kernelCfg: kernelCfg,
		manager:   manager,
		db:        db,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SetEscalationHandler sets the callback for health check failures.
func (h *HealthChecker) SetEscalationHandler(fn func(failCount int)) {
	h.onEscalate = fn
}

// Start begins the health check loop. Call Stop() to terminate.
func (h *HealthChecker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	interval := time.Duration(h.kernelCfg.HealthCheckInterval) * time.Second
	if interval == 0 {
		interval = 10 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.check()
			}
		}
	}()

	log.Printf("[health] started check loop (every %s)", interval)
}

// Stop terminates the health check loop.
func (h *HealthChecker) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
}

// FailCount returns the current consecutive failure count.
func (h *HealthChecker) FailCount() int {
	return int(h.failCount.Load())
}

func (h *HealthChecker) check() {
	url := fmt.Sprintf("http://%s:%d%s", h.cfg.Host, h.cfg.Port, h.cfg.HealthEndpoint)

	start := time.Now()
	resp, err := h.client.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		h.handleFail("timeout", int(elapsed.Milliseconds()), err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		h.handleFail("fail", int(elapsed.Milliseconds()), fmt.Sprintf("status %d: %s", resp.StatusCode, string(body)))
		return
	}

	// Verify the response body
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Status != "ok" {
		h.handleFail("fail", int(elapsed.Milliseconds()), "invalid health response")
		return
	}

	// Success
	h.handleSuccess(int(elapsed.Milliseconds()))
}

func (h *HealthChecker) handleSuccess(responseTimeMs int) {
	prevFails := h.failCount.Swap(0)
	h.manager.SetHealthResult(true)

	// Only log recovery and periodic summaries, not every success
	if prevFails > 0 {
		log.Printf("[health] Jodo recovered after %d failures (%dms)", prevFails, responseTimeMs)
		h.logToDB("ok", responseTimeMs, fmt.Sprintf(`{"recovered_after": %d}`, prevFails))
	}
}

func (h *HealthChecker) handleFail(status string, responseTimeMs int, detail string) {
	count := int(h.failCount.Add(1))
	h.manager.SetHealthResult(false)

	log.Printf("[health] fail #%d: %s (%dms)", count, detail, responseTimeMs)
	h.logToDB(status, responseTimeMs, fmt.Sprintf(`{"detail": %q}`, detail))

	if h.onEscalate != nil {
		h.onEscalate(count)
	}
}

func (h *HealthChecker) logToDB(status string, responseTimeMs int, details string) {
	_, err := h.db.Exec(
		`INSERT INTO health_checks (status, response_time_ms, details) VALUES ($1, $2, $3::jsonb)`,
		status, responseTimeMs, details,
	)
	if err != nil {
		log.Printf("[health] db log error: %v", err)
	}
}
