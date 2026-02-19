package git

import (
	"fmt"
	"strings"
)

// RollbackResponse is the API response for a rollback.
type RollbackResponse struct {
	Status string `json:"status"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// Rollback resets Jodo's brain to a previous commit or tag.
func (m *Manager) Rollback(target string) (*RollbackResponse, error) {
	// Validate target to prevent shell injection
	if !validGitRef.MatchString(target) {
		return nil, fmt.Errorf("invalid rollback target: %q", target)
	}

	// Check .git exists
	if !m.GitExists() {
		return nil, fmt.Errorf("rollback impossible: .git directory missing")
	}

	currentHash, err := m.CurrentHash()
	if err != nil {
		return nil, fmt.Errorf("get current hash: %w", err)
	}

	cmd := fmt.Sprintf("cd %s && git checkout %s -- . && git clean -fd && git reset HEAD", m.cfg.BrainPath, target)
	_, err = m.sshRunner(cmd)
	if err != nil {
		return nil, fmt.Errorf("rollback to %s: %w", target, err)
	}

	return &RollbackResponse{
		Status: "rolling_back",
		From:   currentHash,
		To:     target,
	}, nil
}

// WipeBrain removes everything in brain/ for a nuclear rebirth.
// Handles missing .git gracefully — just deletes all files.
func (m *Manager) WipeBrain() error {
	if m.GitExists() {
		// Preserve .git, delete everything else
		cmd := fmt.Sprintf(
			"cd %s && find . -not -name '.git' -not -path './.git/*' -not -name '.' -delete 2>/dev/null; true",
			m.cfg.BrainPath,
		)
		if _, err := m.sshRunner(cmd); err != nil {
			return fmt.Errorf("wipe brain: %w", err)
		}
		m.Commit("nuclear rebirth — wiped brain")
	} else {
		// No .git — just delete everything and recreate the directory
		cmd := fmt.Sprintf("rm -rf %s/* %s/.* 2>/dev/null; mkdir -p %s; true",
			m.cfg.BrainPath, m.cfg.BrainPath, m.cfg.BrainPath)
		if _, err := m.sshRunner(cmd); err != nil {
			return fmt.Errorf("wipe brain (no git): %w", err)
		}
	}
	return nil
}

// ListTags returns all tags.
func (m *Manager) ListTags() ([]string, error) {
	output, err := m.sshRunner(fmt.Sprintf("cd %s && git tag -l --sort=-version:refname 2>/dev/null || echo ''", m.cfg.BrainPath))
	if err != nil {
		return nil, err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}
