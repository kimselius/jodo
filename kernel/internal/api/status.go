package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// KernelStartTime is set at boot.
var KernelStartTime time.Time

func (s *Server) handleStatus(c *gin.Context) {
	jodoStatus := s.Process.GetStatus()

	// Get git info
	gitHash, _ := s.Git.CurrentHash()
	gitTag, _ := s.Git.CurrentTag()

	// Memory count
	memCount, _ := s.Memory.Count()

	c.JSON(http.StatusOK, gin.H{
		"kernel": gin.H{
			"status":         "running",
			"uptime_seconds": int(time.Since(KernelStartTime).Seconds()),
			"version":        "1.0.0",
		},
		"jodo": gin.H{
			"status":           jodoStatus.Status,
			"pid":              jodoStatus.PID,
			"galla":            jodoStatus.Galla,
			"phase":            jodoStatus.Phase,
			"uptime_seconds":   s.Process.UptimeSeconds(),
			"last_health_check": jodoStatus.LastHealthCheck.Format(time.RFC3339),
			"health_check_ok":  jodoStatus.HealthCheckOK,
			"restarts_today":   jodoStatus.RestartsToday,
			"current_git_tag":  gitTag,
			"current_git_hash": gitHash,
		},
		"database": gin.H{
			"status":          "connected",
			"memories_stored": memCount,
		},
	})
}
