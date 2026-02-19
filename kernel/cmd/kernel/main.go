package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/api"
	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
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
	chatHub := api.NewChatHub()
	wsHub := api.NewWSHub()

	// Wire growth logger to broadcast events via WebSocket
	growthLogger.OnEvent = func(event, note, gitHash string) {
		wsHub.Broadcast("growth", map[string]string{
			"event":    event,
			"note":     note,
			"git_hash": gitHash,
		})
	}

	server := &api.Server{
		Config:      cfg,
		Genesis:     genesis,
		GenesisPath: genesisPath,
		LLM:         proxy,
		Memory:      memStore,
		Searcher:    memSearcher,
		Process:     procManager,
		Git:         gitManager,
		Growth:      growthLogger,
		Audit:       proxy.Audit,
		DB:          database,
		ChatHub:     chatHub,
		WS:          wsHub,
	}

	router := server.SetupRouter()

	// Serve Vue SPA (built frontend)
	webDir := envOr("WEB_DIR", "/app/web")
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		router.Static("/assets", webDir+"/assets")
		router.StaticFile("/favicon.svg", webDir+"/favicon.svg")
		router.StaticFile("/favicon.ico", webDir+"/favicon.ico")

		// SPA fallback: serve index.html for all non-API routes
		indexPath := webDir + "/index.html"
		router.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			c.File(indexPath)
		})
		log.Printf("[boot] serving SPA from %s", webDir)
	} else {
		log.Printf("[boot] no web directory at %s — SPA disabled", webDir)
	}

	// 5. Boot Jodo on VPS 2
	go bootJodo(cfg, procManager, gitManager, growthLogger)

	// 6. Start health checker with recovery
	healthChecker := process.NewHealthChecker(cfg.Jodo, cfg.Kernel, procManager, database)
	recovery := process.NewRecovery(procManager, gitManager, growthLogger, seedPath(), cfg.Kernel.MaxRestartAttempts)
	healthChecker.SetEscalationHandler(recovery.HandleFailure)
	healthChecker.Start()

	// 7. Start periodic maintenance
	go maintenanceLoop(cfg, database, gitManager, procManager, growthLogger)

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

	// Detect brain state: is this a normal restart or a rebirth?
	hasGit := gitMgr.GitExists()
	hasMainPy := gitMgr.MainPyExists()
	hasGalla := gitMgr.GallaFileExists()
	hasPreviousLife := hasGit || hasMainPy

	if hasPreviousLife && !hasGalla {
		// .git or main.py exists but no .galla file — inconsistent state.
		// seed.py writes .galla every galla, so its absence means something
		// went wrong. Wipe brain and let Jodo start fresh.
		log.Printf("[boot] REBIRTH: previous life detected (git=%v, main.py=%v) but no .galla — wiping brain", hasGit, hasMainPy)

		proc.StopAll()

		// Backup before wipe (skip if > 250MB)
		if backupPath, err := gitMgr.BackupBrain(250); err != nil {
			log.Printf("[boot] backup skipped: %v", err)
		} else {
			log.Printf("[boot] brain backed up to %s", backupPath)
		}

		if err := gitMgr.WipeBrain(); err != nil {
			log.Printf("[boot] wipe failed: %v", err)
		}

		growthLog.Log("rebirth", "Boot rebirth: previous life without .galla — backed up and wiped brain", "", nil)
	} else {
		// Normal boot — stop old seed.py but leave Jodo's apps alive
		proc.StopSeed()
	}

	// Initialize git repo on VPS 2
	if err := gitMgr.Init(); err != nil {
		log.Printf("[boot] git init warning: %v", err)
	}

	// Deploy and start fresh seed.py — it IS Jodo's consciousness.
	// seed.py detects .galla file and resumes the galla loop.
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

func maintenanceLoop(cfg *config.Config, database *sql.DB, gitMgr *git.Manager, proc *process.Manager, growthLog *growth.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	appClient := &http.Client{Timeout: 5 * time.Second}
	var lastAppNudge time.Time

	for range ticker.C {
		// Prune old health checks
		db.PruneOldHealthChecks(database)

		status := proc.GetStatus()

		// Check if Jodo's app (port 9000) is reachable
		if status.Status == "running" && status.HealthCheckOK {
			appURL := fmt.Sprintf("http://%s:%d/health", cfg.Jodo.Host, cfg.Jodo.AppPort)
			resp, err := appClient.Get(appURL)
			appOK := err == nil && resp != nil && resp.StatusCode == 200
			if resp != nil {
				resp.Body.Close()
			}

			if !appOK && time.Since(lastAppNudge) > 15*time.Minute {
				msg := fmt.Sprintf("[KERNEL] Your app on port %d is not responding to health checks. Make sure main.py is running with GET /health on port %d.",
					cfg.Jodo.AppPort, cfg.Jodo.AppPort)
				if err := proc.WriteInbox(msg); err != nil {
					log.Printf("[maintenance] inbox write failed: %v", err)
				}
				lastAppNudge = time.Now()
				growthLog.Log("app_down", fmt.Sprintf("App on port %d unreachable, nudged Jodo", cfg.Jodo.AppPort), "", nil)
			}
		}

		// Auto-tag stable versions: if Jodo has been healthy 5+ minutes since last code change
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
