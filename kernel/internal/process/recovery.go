package process

import (
	"log"

	"jodo-kernel/internal/git"
	"jodo-kernel/internal/growth"
)

// Recovery handles the escalation logic when Jodo becomes unhealthy.
type Recovery struct {
	manager   *Manager
	gitMgr    *git.Manager
	growth    *growth.Logger
	seedPath  string
	maxFails  int // from config.KernelConfig.MaxRestartAttempts
}

func NewRecovery(manager *Manager, gitMgr *git.Manager, growth *growth.Logger, seedPath string, maxFails int) *Recovery {
	return &Recovery{
		manager:  manager,
		gitMgr:   gitMgr,
		growth:   growth,
		seedPath: seedPath,
		maxFails: maxFails,
	}
}

// HandleFailure is called by the health checker on each consecutive failure.
// Escalation:
//
//	fail 1-2: log warning, wait
//	fail 3:   restart Jodo process
//	fail 6:   rollback to last stable tag
//	fail 9:   nuclear rebirth (wipe brain, redeploy seed)
func (r *Recovery) HandleFailure(failCount int) {
	switch {
	case failCount < 3:
		log.Printf("[recovery] warning: Jodo unhealthy (fail #%d), waiting...", failCount)
		r.manager.SetStatus("unhealthy")

	case failCount == 3:
		log.Printf("[recovery] restarting Jodo (fail #%d)", failCount)
		r.manager.SetStatus("unhealthy")
		if err := r.manager.RestartJodo(); err != nil {
			log.Printf("[recovery] restart failed: %v", err)
		}
		r.growth.Log("restart", "Health check failed 3 times, restarting", "", nil)

	case failCount == 6:
		log.Printf("[recovery] rolling back Jodo (fail #%d)", failCount)
		r.manager.SetStatus("unhealthy")

		tag, err := r.gitMgr.LastStableTag()
		if err != nil || tag == "" {
			log.Printf("[recovery] no stable tag found, skipping rollback")
			return
		}

		if _, err := r.gitMgr.Rollback(tag); err != nil {
			log.Printf("[recovery] rollback failed: %v", err)
			return
		}

		if err := r.manager.RestartJodo(); err != nil {
			log.Printf("[recovery] restart after rollback failed: %v", err)
		}
		r.growth.Log("rollback", "Rolled back to "+tag, tag, nil)

	case failCount == 9:
		log.Printf("[recovery] NUCLEAR REBIRTH (fail #%d)", failCount)
		r.manager.SetStatus("rebirthing")

		if err := r.manager.StopAll(); err != nil {
			log.Printf("[recovery] stop failed: %v", err)
		}

		if err := r.gitMgr.WipeBrain(); err != nil {
			log.Printf("[recovery] wipe failed: %v", err)
		}

		if err := r.manager.StartSeed(r.seedPath); err != nil {
			log.Printf("[recovery] rebirth failed: %v", err)
			r.manager.SetStatus("dead")
		}
		r.growth.Log("rebirth", "Nuclear rebirth after 9 consecutive health check failures", "", nil)
	}
}
