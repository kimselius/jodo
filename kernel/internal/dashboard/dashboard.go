package dashboard

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"jodo-kernel/internal/git"
	"jodo-kernel/internal/growth"
	"jodo-kernel/internal/llm"
	"jodo-kernel/internal/process"
)

// Handler serves the status dashboard.
type Handler struct {
	Process     *process.Manager
	LLM         *llm.Proxy
	Git         *git.Manager
	Growth      *growth.Logger
	KernelStart time.Time
}

func (h *Handler) Render(c *gin.Context) {
	jodoStatus := h.Process.GetStatus()

	budgetStatus, _ := h.LLM.Budget.GetAllBudgetStatus()
	spentToday, _ := h.LLM.Budget.TotalSpentToday()

	commits, _ := h.Git.Log(10)
	gitHash, _ := h.Git.CurrentHash()
	gitTag, _ := h.Git.CurrentTag()

	milestones, _ := h.Growth.Recent(10)

	html := h.buildHTML(jodoStatus, budgetStatus, spentToday, commits, gitHash, gitTag, milestones)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (h *Handler) buildHTML(
	jodoStatus process.JodoStatus,
	budgetStatus map[string]interface{},
	spentToday float64,
	commits []git.CommitEntry,
	gitHash, gitTag string,
	milestones []growth.Milestone,
) string {
	statusColor := map[string]string{
		"running":    "#22c55e",
		"starting":   "#eab308",
		"unhealthy":  "#f97316",
		"dead":       "#ef4444",
		"rebirthing": "#a855f7",
	}
	color := statusColor[jodoStatus.Status]
	if color == "" {
		color = "#6b7280"
	}

	var budgetHTML strings.Builder
	for name, status := range budgetStatus {
		switch v := status.(type) {
		case llm.BudgetStatus:
			pct := 0.0
			if v.MonthlyBudget > 0 {
				pct = v.SpentThisMonth / v.MonthlyBudget * 100
			}
			budgetHTML.WriteString(fmt.Sprintf(`
				<div class="budget-item">
					<div class="budget-header"><strong>%s</strong> $%.2f / $%.2f</div>
					<div class="bar"><div class="bar-fill" style="width: %.1f%%; background: %s;"></div></div>
					<div class="budget-detail">Emergency reserve: $%.2f | Available: $%.2f</div>
				</div>`, name, v.SpentThisMonth, v.MonthlyBudget, pct, barColor(pct), v.EmergencyReserve, v.AvailableForNormal))
		default:
			budgetHTML.WriteString(fmt.Sprintf(`
				<div class="budget-item">
					<div class="budget-header"><strong>%s</strong> — free / unlimited</div>
					<div class="bar"><div class="bar-fill" style="width: 0%%; background: #22c55e;"></div></div>
				</div>`, name))
		}
	}

	var commitsHTML strings.Builder
	for _, c := range commits {
		tag := ""
		if c.Tag != "" {
			tag = fmt.Sprintf(` <span class="tag">%s</span>`, c.Tag)
		}
		commitsHTML.WriteString(fmt.Sprintf(
			`<tr><td class="hash">%s</td><td>%s%s</td><td>%s</td></tr>`,
			c.Hash, c.Message, tag, c.Timestamp,
		))
	}

	var milestonesHTML strings.Builder
	for _, m := range milestones {
		milestonesHTML.WriteString(fmt.Sprintf(
			`<tr><td class="event">%s</td><td>%s</td><td>%s</td></tr>`,
			m.Event, m.Note, m.CreatedAt.Format("2006-01-02 15:04"),
		))
	}

	uptime := int(time.Since(h.KernelStart).Seconds())
	jodoUptime := h.Process.UptimeSeconds()

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="10">
<title>Jodo Kernel Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #0f172a; color: #e2e8f0; padding: 24px; }
  .container { max-width: 960px; margin: 0 auto; }
  h1 { font-size: 24px; margin-bottom: 24px; color: #f8fafc; }
  h1 span { color: #64748b; font-weight: 400; font-size: 14px; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 24px; }
  .card { background: #1e293b; border-radius: 8px; padding: 20px; }
  .card h2 { font-size: 14px; text-transform: uppercase; color: #64748b; margin-bottom: 12px; letter-spacing: 0.05em; }
  .status-badge { display: inline-block; padding: 4px 12px; border-radius: 12px; font-size: 14px; font-weight: 600; color: white; }
  .stat { margin: 8px 0; font-size: 14px; }
  .stat strong { color: #94a3b8; }
  .bar { background: #334155; border-radius: 4px; height: 8px; margin: 4px 0; }
  .bar-fill { height: 100%%; border-radius: 4px; transition: width 0.3s; }
  .budget-item { margin-bottom: 12px; }
  .budget-header { font-size: 14px; }
  .budget-detail { font-size: 12px; color: #64748b; }
  table { width: 100%%; border-collapse: collapse; font-size: 13px; }
  th { text-align: left; color: #64748b; padding: 6px 8px; border-bottom: 1px solid #334155; font-size: 12px; text-transform: uppercase; }
  td { padding: 6px 8px; border-bottom: 1px solid #1e293b; }
  .hash { font-family: monospace; color: #38bdf8; }
  .tag { background: #22c55e; color: #0f172a; padding: 1px 6px; border-radius: 4px; font-size: 11px; font-weight: 600; }
  .event { font-family: monospace; color: #a78bfa; }
  .controls { display: flex; gap: 8px; margin-top: 12px; }
  .btn { padding: 8px 16px; border: none; border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 600; }
  .btn-restart { background: #eab308; color: #0f172a; }
  .btn-rollback { background: #f97316; color: white; }
  .btn-kill { background: #ef4444; color: white; }
  .btn-rebirth { background: #a855f7; color: white; }
  .full-width { grid-column: 1 / -1; }
</style>
</head>
<body>
<div class="container">
  <h1>Jodo Kernel <span>v1.0.0 | uptime %ds</span></h1>

  <div class="grid">
    <div class="card">
      <h2>Jodo Status</h2>
      <div><span class="status-badge" style="background: %s;">%s</span></div>
      <div class="stat"><strong>PID:</strong> %d</div>
      <div class="stat"><strong>Uptime:</strong> %ds</div>
      <div class="stat"><strong>Last health check:</strong> %s</div>
      <div class="stat"><strong>Restarts today:</strong> %d</div>
      <div class="stat"><strong>Git:</strong> %s %s</div>
      <div class="controls">
        <form method="POST" action="/api/restart"><button class="btn btn-restart" type="submit">Restart</button></form>
        <form method="POST" action="/api/rollback"><input type="hidden" name="target" value=""><button class="btn btn-rollback" type="submit" disabled>Roll Back</button></form>
      </div>
    </div>

    <div class="card">
      <h2>Budget</h2>
      %s
      <div class="stat" style="margin-top: 12px;"><strong>Spent today:</strong> $%.4f</div>
    </div>

    <div class="card full-width">
      <h2>Git History</h2>
      <table>
        <tr><th>Hash</th><th>Message</th><th>Time</th></tr>
        %s
      </table>
    </div>

    <div class="card full-width">
      <h2>Growth Milestones</h2>
      <table>
        <tr><th>Event</th><th>Note</th><th>Time</th></tr>
        %s
      </table>
    </div>
  </div>
</div>
</body>
</html>`,
		uptime,
		color, jodoStatus.Status,
		jodoStatus.PID,
		jodoUptime,
		jodoStatus.LastHealthCheck.Format("15:04:05"),
		jodoStatus.RestartsToday,
		gitHash, gitTag,
		budgetHTML.String(),
		spentToday,
		commitsHTML.String(),
		milestonesHTML.String(),
	)
}

func barColor(pct float64) string {
	switch {
	case pct > 90:
		return "#ef4444"
	case pct > 70:
		return "#f97316"
	case pct > 50:
		return "#eab308"
	default:
		return "#22c55e"
	}
}
