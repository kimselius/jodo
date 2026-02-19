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
		"providers":         budgetStatus,
		"total_spent_today": spentToday,
		"budget_resets":     nextMonth.Format(time.RFC3339),
	})
}

// GET /api/budget/breakdown — per-model spending breakdown
func (s *Server) handleBudgetBreakdown(c *gin.Context) {
	firstOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)

	rows, err := s.DB.Query(
		`SELECT provider, model, intent, COUNT(*) as calls,
		        COALESCE(SUM(tokens_in), 0) as total_tokens_in,
		        COALESCE(SUM(tokens_out), 0) as total_tokens_out,
		        COALESCE(SUM(cost), 0) as total_cost
		 FROM budget_usage
		 WHERE created_at >= $1
		 GROUP BY provider, model, intent
		 ORDER BY total_cost DESC`,
		firstOfMonth,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type modelBreakdown struct {
		Provider  string  `json:"provider"`
		Model     string  `json:"model"`
		Intent    string  `json:"intent"`
		Calls     int     `json:"calls"`
		TokensIn  int     `json:"tokens_in"`
		TokensOut int     `json:"tokens_out"`
		Cost      float64 `json:"cost"`
	}

	var breakdown []modelBreakdown
	for rows.Next() {
		var mb modelBreakdown
		if err := rows.Scan(&mb.Provider, &mb.Model, &mb.Intent, &mb.Calls, &mb.TokensIn, &mb.TokensOut, &mb.Cost); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		breakdown = append(breakdown, mb)
	}

	c.JSON(http.StatusOK, gin.H{"breakdown": breakdown})
}
