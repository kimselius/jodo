package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleBudget(c *gin.Context) {
	budgetStatus, err := s.LLM.Budget.GetAllBudgetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	spentToday, _ := s.LLM.Budget.TotalSpentToday()

	// Calculate next budget reset (first of next month)
	now := time.Now().UTC()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	c.JSON(http.StatusOK, gin.H{
		"providers":        budgetStatus,
		"total_spent_today": spentToday,
		"budget_resets":    nextMonth.Format(time.RFC3339),
	})
}
