package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
	"jodo-kernel/internal/crypto"
	"jodo-kernel/internal/git"
	"jodo-kernel/internal/growth"
	"jodo-kernel/internal/llm"
	"jodo-kernel/internal/memory"
	"jodo-kernel/internal/process"
)

// Server holds all dependencies for the API handlers.
type Server struct {
	Config    *config.Config
	Genesis   *config.Genesis
	LLM       *llm.Proxy
	Memory    *memory.Store
	Searcher  *memory.Searcher
	Process   *process.Manager
	Git       *git.Manager
	Growth    *growth.Logger
	Audit     *audit.Logger
	DB        *sql.DB
	ChatHub   *ChatHub
	WS        *WSHub

	// Config-in-DB
	ConfigStore   *config.DBStore
	Encryptor     *crypto.Encryptor
	SetupComplete bool
	JodoMode      string       // "vps" or "docker"
	OnBirth       func() error // called by setup wizard to birth Jodo
}

// requireSetupComplete returns 503 if setup hasn't been completed yet.
func (s *Server) requireSetupComplete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.SetupComplete {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "setup_not_complete",
				"message": "Complete the setup wizard first",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetupRouter creates and configures the Gin router with all API routes.
func (s *Server) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")

	// Setup endpoints — always accessible
	setup := api.Group("/setup")
	{
		setup.GET("/status", s.handleSetupStatus)
		setup.POST("/ssh/generate", s.handleSetupSSHGenerate)
		setup.POST("/ssh/verify", s.handleSetupSSHVerify)
		setup.POST("/providers", s.handleSetupProviders)
		setup.POST("/genesis", s.handleSetupGenesis)
		setup.POST("/test-provider", s.handleSetupTestProvider)
		setup.POST("/birth", s.handleSetupBirth)
		setup.POST("/config", s.handleSetupConfig)
		setup.POST("/provision", s.handleSetupProvision)
		setup.POST("/discover", s.handleSetupDiscover)
		setup.POST("/routing", s.handleSetupRouting)
	}

	// Settings endpoints — require setup complete
	settings := api.Group("/settings", s.requireSetupComplete())
	{
		settings.GET("/providers", s.handleSettingsProvidersGet)
		settings.PUT("/providers/:name", s.handleSettingsProviderPut)
		settings.POST("/providers/:name/models", s.handleSettingsModelAdd)
		settings.PUT("/providers/:name/models/:key", s.handleSettingsModelUpdate)
		settings.DELETE("/providers/:name/models/:key", s.handleSettingsModelDelete)
		settings.GET("/providers/:name/discover", s.handleSettingsProviderDiscover)
		settings.GET("/genesis", s.handleSettingsGenesisGet)
		settings.PUT("/genesis", s.handleSettingsGenesisPut)
		settings.GET("/routing", s.handleSettingsRoutingGet)
		settings.PUT("/routing", s.handleSettingsRoutingPut)
		settings.GET("/kernel", s.handleSettingsKernelGet)
		settings.PUT("/kernel", s.handleSettingsKernelPut)
		settings.GET("/ssh", s.handleSettingsSSHGet)
		settings.POST("/ssh/generate", s.handleSetupSSHGenerate)
		settings.POST("/ssh/verify", s.handleSetupSSHVerify)
		settings.GET("/subagent", s.handleSettingsSubagentGet)
		settings.PUT("/subagent", s.handleSettingsSubagentPut)
		settings.GET("/vram", s.handleSettingsVRAMStatus)
	}

	// Operational endpoints — require setup complete
	ops := api.Group("", s.requireSetupComplete())
	{
		ops.POST("/think", s.handleThink)
		ops.POST("/memory/store", s.handleMemoryStore)
		ops.POST("/memory/search", s.handleMemorySearch)
		ops.GET("/budget", s.handleBudget)
		ops.GET("/budget/breakdown", s.handleBudgetBreakdown)
		ops.GET("/status", s.handleStatus)
		ops.GET("/genesis", s.handleGenesis)
		ops.POST("/commit", s.handleCommit)
		ops.POST("/restart", s.handleRestart)
		ops.POST("/rollback", s.handleRollback)
		ops.GET("/history", s.handleHistory)
		ops.POST("/log", s.handleLog)
		ops.POST("/chat", s.handleChatPost)
		ops.GET("/chat", s.handleChatGet)
		ops.GET("/chat/stream", s.handleChatStream)
		ops.POST("/chat/ack", s.handleChatAck)
		ops.GET("/genesis/identity", s.handleGenesisIdentityGet)
		ops.PUT("/genesis/identity", s.handleGenesisIdentityPut)
		ops.GET("/memories", s.handleMemoriesList)
		ops.GET("/growth", s.handleGrowthLog)
		ops.POST("/galla", s.handleGallaPost)
		ops.GET("/galla", s.handleGallaGet)
		ops.POST("/heartbeat", s.handleHeartbeat)
		ops.GET("/ws", s.handleWS)

		// Library
		ops.GET("/library", s.handleLibraryList)
		ops.POST("/library", s.handleLibraryCreate)
		ops.GET("/library/:id", s.handleLibraryGet)
		ops.PUT("/library/:id", s.handleLibraryUpdate)
		ops.PATCH("/library/:id", s.handleLibraryPatch)
		ops.DELETE("/library/:id", s.handleLibraryDelete)
		ops.POST("/library/:id/comments", s.handleLibraryCommentPost)

		// Inbox
		ops.GET("/inbox", s.handleInboxList)
		ops.POST("/inbox", s.handleInboxPost)

		// LLM Calls
		ops.GET("/llm-calls", s.handleLLMCallsList)
		ops.GET("/llm-calls/:id", s.handleLLMCallDetail)

		// Memory search (GET-based for UI)
		ops.GET("/memories/search", s.handleMemoriesSearchGet)
	}

	// Reverse proxy to Jodo's app — registered on the base router (not /api)
	// so it's caught before the SPA fallback. Requires setup complete.
	r.Any("/jodo/*path", s.requireSetupComplete(), s.jodoProxy())

	return r
}
