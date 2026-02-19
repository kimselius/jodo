package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/audit"
	"jodo-kernel/internal/config"
	"jodo-kernel/internal/git"
	"jodo-kernel/internal/growth"
	"jodo-kernel/internal/llm"
	"jodo-kernel/internal/memory"
	"jodo-kernel/internal/process"
)

// Server holds all dependencies for the API handlers.
type Server struct {
	Config   *config.Config
	Genesis  *config.Genesis
	LLM      *llm.Proxy
	Memory   *memory.Store
	Searcher *memory.Searcher
	Process  *process.Manager
	Git      *git.Manager
	Growth   *growth.Logger
	Audit    *audit.Logger
	DB       *sql.DB
	ChatHub  *ChatHub
}

// SetupRouter creates and configures the Gin router with all API routes.
func (s *Server) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API routes
	api := r.Group("/api")
	{
		api.POST("/think", s.handleThink)
		api.POST("/memory/store", s.handleMemoryStore)
		api.POST("/memory/search", s.handleMemorySearch)
		api.GET("/budget", s.handleBudget)
		api.GET("/status", s.handleStatus)
		api.GET("/genesis", s.handleGenesis)
		api.POST("/commit", s.handleCommit)
		api.POST("/restart", s.handleRestart)
		api.POST("/rollback", s.handleRollback)
		api.GET("/history", s.handleHistory)
		api.POST("/log", s.handleLog)
		api.POST("/chat", s.handleChatPost)
		api.GET("/chat", s.handleChatGet)
		api.GET("/chat/stream", s.handleChatStream)
		api.POST("/chat/ack", s.handleChatAck)
	}

	// Dashboard is mounted externally in main.go

	return r
}
