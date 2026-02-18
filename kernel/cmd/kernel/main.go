package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jodo-kernel/internal/api"
	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
	"jodo-kernel/internal/dashboard"
	"database/sql"
	"jodo-kernel/internal/db"
	"jodo-kernel/internal/git"
	"jodo-kernel/internal/growth"
	"jodo-kernel/internal/llm"
	"jodo-kernel/internal/memory"
	"jodo-kernel/internal/process"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	api.KernelStartTime = time.Now()

	log.Println("========================================")
	log.Println("  JODO KERNEL — booting...")
	log.Println("========================================")

	// 1. Load configuration
	configPath := envOr("KERNEL_CONFIG", "/app/configs/config.yaml")
	genesisPath := envOr("KERNEL_GENESIS", "/app/configs/genesis.yaml")

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[boot] config: %v", err)
	}
	log.Printf("[boot] config loaded (port %d, %d providers)", cfg.Kernel.Port, len(cfg.Providers))

	genesis, err := config.LoadGenesis(genesisPath)
	if err != nil {
		log.Fatalf("[boot] genesis: %v", err)
	}
	log.Printf("[boot] genesis loaded: %s v%d", genesis.Identity.Name, genesis.Identity.Version)

	// 2. Connect to Postgres
	database, err := db.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("[boot] database: %v", err)
	}
	defer database.Close()
	log.Println("[boot] database connected")

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("[boot] migrations: %v", err)
	}
	log.Println("[boot] migrations complete")

	// 3. Initialize subsystems
	proxy := llm.NewProxy(database, cfg.Providers, cfg.Routing)
	log.Println("[boot] LLM proxy ready")

	// Audit logger — records every prompt/response for safety review
	if cfg.Kernel.AuditLogPath != "" {
		auditLogger, err := audit.NewLogger(cfg.Kernel.AuditLogPath)
		if err != nil {
			log.Printf("[boot] WARNING: audit log failed: %v (continuing without audit)", err)
		} else {
			proxy.Audit = auditLogger
			defer auditLogger.Close()
			log.Printf("[boot] audit logging to %s", cfg.Kernel.AuditLogPath)
		}
	}

	memStore := memory.NewStore(database, proxy)
	memSearcher := memory.NewSearcher(database, proxy)

	kernelURL := cfg.Kernel.ExternalURL
	if kernelURL == "" {
		kernelURL = fmt.Sprintf("http://localhost:%d", cfg.Kernel.Port)
		log.Printf("[boot] WARNING: kernel.external_url not set — using %s (won't work across VPS)", kernelURL)
	}
	procManager := process.NewManager(cfg.Jodo, kernelURL)
	gitManager := git.NewManager(cfg.Jodo, procManager.RunSSH)
	growthLogger := growth.NewLogger(database)

	// 4. Set up API server
	server := &api.Server{
		Config:   cfg,
		Genesis:  genesis,
		LLM:      proxy,
		Memory:   memStore,
		Searcher: memSearcher,
		Process:  procManager,
		Git:      gitManager,
		Growth:   growthLogger,
		Audit:    proxy.Audit,
	}

	router := server.SetupRouter()

	// Wire up dashboard
	dash := &dashboard.Handler{
		Process:     procManager,
		LLM:         proxy,
		Git:         gitManager,
		Growth:      growthLogger,
		KernelStart: api.KernelStartTime,
	}
	// Override the dashboard route with the properly wired handler
	router.GET("/dashboard", dash.Render)

	// 5. Boot Jodo on VPS 2
	go bootJodo(cfg, procManager, gitManager, growthLogger)

	// 6. Start health checker with recovery
	healthChecker := process.NewHealthChecker(cfg.Jodo, cfg.Kernel, procManager, database)
	recovery := process.NewRecovery(procManager, gitManager, growthLogger, seedPath(), cfg.Kernel.MaxRestartAttempts)
	healthChecker.SetEscalationHandler(recovery.HandleFailure)
	healthChecker.Start()

	// 7. Start periodic maintenance
	go maintenanceLoop(database, gitManager, procManager, growthLogger)

	// 8. Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Kernel.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Printf("[boot] API server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[boot] server: %v", err)
		}
	}()

	growthLogger.Log("boot", "Kernel started", "", nil)
	log.Println("[boot] Jodo Kernel is alive.")

	// 9. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[shutdown] signal received, shutting down...")
	healthChecker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("[shutdown] Jodo Kernel stopped. Jodo continues to run independently.")
}

func bootJodo(cfg *config.Config, proc *process.Manager, gitMgr *git.Manager, growthLog *growth.Logger) {
	log.Println("[boot] connecting to VPS 2...")

	// Initialize git repo on VPS 2
	if err := gitMgr.Init(); err != nil {
		log.Printf("[boot] git init warning: %v", err)
	}

	// Always deploy and start seed.py — it IS Jodo's consciousness.
	// seed.py detects if main.py exists and manages everything else.
	log.Println("[boot] deploying seed.py...")
	if err := proc.StartSeed(seedPath()); err != nil {
		log.Printf("[boot] seed failed: %v", err)
		proc.SetStatus("dead")
		return
	}
	growthLog.Log("boot", "seed.py deployed and started", "", nil)
}

func seedPath() string {
	return envOr("SEED_PATH", "/app/seed/seed.py")
}

func maintenanceLoop(database *sql.DB, gitMgr *git.Manager, proc *process.Manager, growthLog *growth.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Prune old health checks
		db.PruneOldHealthChecks(database)

		// Auto-tag stable versions: if Jodo has been healthy 5+ minutes since last code change
		status := proc.GetStatus()
		if status.Status == "running" && status.HealthCheckOK {
			ago, err := gitMgr.LastModifiedAgo()
			if err == nil && ago > 5*time.Minute {
				currentTag, _ := gitMgr.CurrentTag()
				if currentTag == "" || !isStableTag(currentTag) {
					count, _ := gitMgr.StableTagCount()
					newTag := fmt.Sprintf("stable-v%d", count+1)
					if err := gitMgr.Tag(newTag); err == nil {
						log.Printf("[maintenance] tagged %s as stable", newTag)
						growthLog.Log("stable_tag", "Auto-tagged "+newTag, "", nil)
					}
				}
			}
		}
	}
}

func isStableTag(tag string) bool {
	return len(tag) > 7 && tag[:7] == "stable-"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
