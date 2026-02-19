package main

import (
	"context"
	"database/sql"
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
	"jodo-kernel/internal/crypto"
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

	// 1. Bootstrap: load DB config from env (chicken-and-egg)
	dbCfg := config.LoadDatabaseConfig()
	if dbCfg.Password == "" {
		log.Fatal("[boot] JODO_DB_PASSWORD not set. Run './jodo.sh setup' first.")
	}

	// 2. Connect to Postgres
	database, err := db.Connect(dbCfg)
	if err != nil {
		log.Fatalf("[boot] database: %v", err)
	}
	defer database.Close()
	log.Println("[boot] database connected")

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("[boot] migrations: %v", err)
	}
	log.Println("[boot] migrations complete")

	// 3. Initialize encryptor
	encryptor, err := crypto.NewFromEnv()
	if err != nil {
		log.Fatalf("[boot] encryption: %v", err)
	}
	log.Println("[boot] encryptor ready")

	// 4. Create config store
	configStore := config.NewDBStore(database, encryptor)

	// 5. Docker mode: auto-import mounted SSH key if present
	if envOr("JODO_MODE", "vps") == "docker" {
		autoImportDockerSSHKey(configStore)
	}

	// 6. Check setup status
	setupComplete := configStore.IsSetupComplete()

	if !setupComplete {
		// Try auto-migration from YAML
		setupComplete = autoMigrateYAML(configStore, database)
	}

	// 7. Create server — either in setup mode or fully operational
	if setupComplete {
		log.Println("[boot] setup complete — starting in operational mode")
		startOperational(database, dbCfg, configStore, encryptor)
	} else {
		log.Println("[boot] setup not complete — starting in setup mode")
		startSetupMode(database, dbCfg, configStore, encryptor)
	}
}

// startOperational boots the kernel with all subsystems (normal mode).
func startOperational(database *sql.DB, dbCfg config.DatabaseConfig, configStore *config.DBStore, encryptor *crypto.Encryptor) {
	cfg, err := configStore.LoadFullConfig(dbCfg)
	if err != nil {
		log.Fatalf("[boot] load config from DB: %v", err)
	}
	log.Printf("[boot] config loaded from DB (port %d, %d providers)", cfg.Kernel.Port, len(cfg.Providers))

	genesis, err := configStore.LoadGenesis()
	if err != nil {
		log.Fatalf("[boot] load genesis from DB: %v", err)
	}
	log.Printf("[boot] genesis loaded: %s v%d", genesis.Identity.Name, genesis.Identity.Version)

	// Initialize subsystems
	proxy := llm.NewProxy(database, cfg.Providers, cfg.Routing)
	log.Println("[boot] LLM proxy ready")

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
		log.Printf("[boot] WARNING: kernel.external_url not set — using %s", kernelURL)
	}

	// Write SSH key to temp file for process manager
	sshKeyPath, err := writeSSHKeyToTemp(configStore)
	if err != nil {
		log.Printf("[boot] WARNING: SSH key not available: %v (Jodo boot will fail)", err)
	}
	cfg.Jodo.SSHKeyPath = sshKeyPath

	procManager := process.NewManager(cfg.Jodo, kernelURL)
	gitManager := git.NewManager(cfg.Jodo, procManager.RunSSH)
	growthLogger := growth.NewLogger(database)

	// Set up API server
	chatHub := api.NewChatHub()
	wsHub := api.NewWSHub()

	growthLogger.OnEvent = func(event, note, gitHash string) {
		wsHub.Broadcast("growth", map[string]string{
			"event":    event,
			"note":     note,
			"git_hash": gitHash,
		})
	}

	server := &api.Server{
		Config:        cfg,
		Genesis:       genesis,
		LLM:           proxy,
		Memory:        memStore,
		Searcher:      memSearcher,
		Process:       procManager,
		Git:           gitManager,
		Growth:        growthLogger,
		Audit:         proxy.Audit,
		DB:            database,
		ChatHub:       chatHub,
		WS:            wsHub,
		ConfigStore:   configStore,
		Encryptor:     encryptor,
		SetupComplete: true,
		JodoMode:      envOr("JODO_MODE", "vps"),
	}

	router := server.SetupRouter()

	// Serve Vue SPA
	webDir := envOr("WEB_DIR", "/app/web")
	serveSPA(router, webDir)

	// Boot Jodo + health checker
	healthChecker := process.NewHealthChecker(cfg.Jodo, cfg.Kernel, procManager, database)
	recovery := process.NewRecovery(procManager, gitManager, growthLogger, seedPath(), cfg.Kernel.MaxRestartAttempts)
	healthChecker.SetEscalationHandler(recovery.HandleFailure)

	go func() {
		bootJodo(cfg, procManager, gitManager, growthLogger)
		log.Println("[boot] waiting 30s for seed.py to initialize...")
		time.Sleep(30 * time.Second)
		healthChecker.Start()
	}()

	go maintenanceLoop(cfg, database, gitManager, procManager, growthLogger)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Kernel.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("[boot] API server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[boot] server: %v", err)
		}
	}()

	growthLogger.Log("boot", "Kernel started", "", nil)
	log.Println("[boot] Jodo Kernel is alive.")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[shutdown] signal received, shutting down...")
	healthChecker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("[shutdown] Jodo Kernel stopped.")
}

// startSetupMode starts the kernel in setup mode (wizard UI, no Jodo boot).
func startSetupMode(database *sql.DB, dbCfg config.DatabaseConfig, configStore *config.DBStore, encryptor *crypto.Encryptor) {
	server := &api.Server{
		DB:            database,
		ConfigStore:   configStore,
		Encryptor:     encryptor,
		SetupComplete: false,
		JodoMode:      envOr("JODO_MODE", "vps"),
		// Subsystems are nil — operational routes will return 503
	}

	router := server.SetupRouter()

	// Serve Vue SPA
	webDir := envOr("WEB_DIR", "/app/web")
	serveSPA(router, webDir)

	port := 8080 // default port during setup
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr, Handler: router}

	// Provide callback for birth
	server.OnBirth = func() error {
		return birthJodo(server, database, dbCfg, configStore, encryptor)
	}

	go func() {
		log.Printf("[boot] Setup wizard listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[boot] server: %v", err)
		}
	}()

	log.Println("[boot] Waiting for setup to complete via UI...")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[shutdown] signal received, shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// birthJodo is called when the user clicks "Birth Jodo" in the setup wizard.
// It initializes all subsystems and boots Jodo.
func birthJodo(server *api.Server, database *sql.DB, dbCfg config.DatabaseConfig, configStore *config.DBStore, encryptor *crypto.Encryptor) error {
	log.Println("[birth] initializing subsystems...")

	cfg, err := configStore.LoadFullConfig(dbCfg)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	genesis, err := configStore.LoadGenesis()
	if err != nil {
		return fmt.Errorf("load genesis: %w", err)
	}

	proxy := llm.NewProxy(database, cfg.Providers, cfg.Routing)
	if cfg.Kernel.AuditLogPath != "" {
		if auditLogger, err := audit.NewLogger(cfg.Kernel.AuditLogPath); err == nil {
			proxy.Audit = auditLogger
		}
	}

	memStore := memory.NewStore(database, proxy)
	memSearcher := memory.NewSearcher(database, proxy)

	kernelURL := cfg.Kernel.ExternalURL
	if kernelURL == "" {
		kernelURL = fmt.Sprintf("http://localhost:%d", cfg.Kernel.Port)
	}

	sshKeyPath, err := writeSSHKeyToTemp(configStore)
	if err != nil {
		return fmt.Errorf("SSH key: %w", err)
	}
	cfg.Jodo.SSHKeyPath = sshKeyPath

	procManager := process.NewManager(cfg.Jodo, kernelURL)
	gitManager := git.NewManager(cfg.Jodo, procManager.RunSSH)
	growthLogger := growth.NewLogger(database)

	chatHub := api.NewChatHub()
	wsHub := api.NewWSHub()
	growthLogger.OnEvent = func(event, note, gitHash string) {
		wsHub.Broadcast("growth", map[string]string{
			"event": event, "note": note, "git_hash": gitHash,
		})
	}

	// Wire everything into the server
	server.Config = cfg
	server.Genesis = genesis
	server.LLM = proxy
	server.Memory = memStore
	server.Searcher = memSearcher
	server.Process = procManager
	server.Git = gitManager
	server.Growth = growthLogger
	server.Audit = proxy.Audit
	server.ChatHub = chatHub
	server.WS = wsHub
	server.SetupComplete = true

	// Boot Jodo
	go func() {
		bootJodo(cfg, procManager, gitManager, growthLogger)
		healthChecker := process.NewHealthChecker(cfg.Jodo, cfg.Kernel, procManager, database)
		recovery := process.NewRecovery(procManager, gitManager, growthLogger, seedPath(), cfg.Kernel.MaxRestartAttempts)
		healthChecker.SetEscalationHandler(recovery.HandleFailure)

		log.Println("[birth] waiting 30s for seed.py to initialize...")
		time.Sleep(30 * time.Second)
		healthChecker.Start()

		go maintenanceLoop(cfg, database, gitManager, procManager, growthLogger)
	}()

	growthLogger.Log("birth", "Jodo was born!", "", nil)
	log.Println("[birth] Jodo is being born!")
	return nil
}

// autoImportDockerSSHKey imports the mounted SSH key pair into the DB for Docker mode.
// Keys are generated by jodo.sh setup and mounted at /app/ssh/.
func autoImportDockerSSHKey(store *config.DBStore) {
	keyPath := "/app/ssh/jodo_key"
	pubPath := "/app/ssh/jodo_key.pub"

	// Check if key already exists in DB
	existing, _ := store.GetSecret("ssh_private_key")
	if existing != "" {
		return
	}

	privKey, err := os.ReadFile(keyPath)
	if err != nil {
		log.Printf("[boot] no mounted SSH key at %s: %v", keyPath, err)
		return
	}
	pubKey, err := os.ReadFile(pubPath)
	if err != nil {
		log.Printf("[boot] no mounted SSH public key at %s: %v", pubPath, err)
		return
	}

	if err := store.SaveSecret("ssh_private_key", string(privKey)); err != nil {
		log.Printf("[boot] failed to import SSH private key: %v", err)
		return
	}
	if err := store.SetConfig("ssh_public_key", strings.TrimSpace(string(pubKey))); err != nil {
		log.Printf("[boot] failed to import SSH public key: %v", err)
		return
	}

	// Auto-configure Docker SSH connection defaults
	store.SetConfig("jodo.host", "jodo")
	store.SetConfig("jodo.ssh_user", "root")

	log.Println("[boot] Docker SSH key imported and connection configured")
}

// writeSSHKeyToTemp reads the SSH private key from the DB and writes it to a temp file.
func writeSSHKeyToTemp(store *config.DBStore) (string, error) {
	key, err := store.GetSecret("ssh_private_key")
	if err != nil {
		return "", fmt.Errorf("get ssh key from db: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("no SSH key configured")
	}

	tmpFile, err := os.CreateTemp("", "jodo-ssh-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := tmpFile.WriteString(key); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0600)
	return tmpFile.Name(), nil
}

// autoMigrateYAML tries to import existing YAML config into the database.
func autoMigrateYAML(store *config.DBStore, database *sql.DB) bool {
	configPath := envOr("KERNEL_CONFIG", "/app/configs/config.yaml")
	genesisPath := envOr("KERNEL_GENESIS", "/app/configs/genesis.yaml")

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("[boot] no valid YAML config at %s: %v", configPath, err)
		return false
	}

	genesis, err := config.LoadGenesis(genesisPath)
	if err != nil {
		log.Printf("[boot] no valid genesis at %s: %v", genesisPath, err)
		return false
	}

	// Only auto-migrate if YAML has meaningful provider config
	if len(cfg.Providers) == 0 {
		log.Println("[boot] YAML config has no providers — skipping auto-migration")
		return false
	}

	log.Println("[boot] auto-migrating YAML config to database...")
	if err := store.ImportConfig(cfg); err != nil {
		log.Printf("[boot] auto-migration failed: %v", err)
		return false
	}
	if err := store.ImportGenesis(genesis); err != nil {
		log.Printf("[boot] genesis migration failed: %v", err)
		return false
	}
	if err := store.SetConfig("setup_complete", "true"); err != nil {
		log.Printf("[boot] failed to mark setup complete: %v", err)
		return false
	}

	log.Println("[boot] YAML config migrated to database successfully")
	return true
}

func bootJodo(cfg *config.Config, proc *process.Manager, gitMgr *git.Manager, growthLog *growth.Logger) {
	log.Println("[boot] connecting to VPS 2...")

	hasGit := gitMgr.GitExists()
	hasMainPy := gitMgr.MainPyExists()
	hasGalla := gitMgr.GallaFileExists()
	hasPreviousLife := hasGit || hasMainPy

	if hasPreviousLife && !hasGalla {
		log.Printf("[boot] REBIRTH: previous life detected (git=%v, main.py=%v) but no .galla — wiping brain", hasGit, hasMainPy)
		proc.StopAll()

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
		proc.StopSeed()
	}

	if err := gitMgr.Init(); err != nil {
		log.Printf("[boot] git init warning: %v", err)
	}

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
		db.PruneOldHealthChecks(database)

		status := proc.GetStatus()

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

func serveSPA(router *gin.Engine, webDir string) {
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		router.Static("/assets", webDir+"/assets")
		router.StaticFile("/favicon.svg", webDir+"/favicon.svg")
		router.StaticFile("/favicon.ico", webDir+"/favicon.ico")

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
}
