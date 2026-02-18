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
func (m *Manager) WipeBrain() error {
	cmd := fmt.Sprintf(
		"cd %s && find . -not -name '.git' -not -path './.git/*' -not -name '.' -delete 2>/dev/null; true",
		m.cfg.BrainPath,
	)
	_, err := m.sshRunner(cmd)
	if err != nil {
		return fmt.Errorf("wipe brain: %w", err)
	}

	// Commit the wipe
	m.Commit("nuclear rebirth — wiped brain")
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
