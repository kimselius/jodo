package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

// BackupBrain creates a tar.gz backup of the brain directory before a wipe.
// Returns the backup path, or an error. Skips backup if dir > maxMB.
func (m *Manager) BackupBrain(maxMB int) (string, error) {
	// Check directory size
	sizeOut, err := m.sshRunner(fmt.Sprintf("du -sm %s 2>/dev/null | cut -f1", m.cfg.BrainPath))
	if err != nil {
		return "", fmt.Errorf("check brain size: %w", err)
	}
	sizeMB, _ := strconv.Atoi(strings.TrimSpace(sizeOut))
	if sizeMB > maxMB {
		return "", fmt.Errorf("brain too large (%dMB > %dMB limit), skipping backup", sizeMB, maxMB)
	}

	// Create backups directory
	backupDir := "/opt/jodo/backups"
	m.sshRunner(fmt.Sprintf("mkdir -p %s", backupDir))

	// Create timestamped backup
	ts := time.Now().UTC().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s/brain-%s.tar.gz", backupDir, ts)
	cmd := fmt.Sprintf("tar czf %s -C %s . 2>/dev/null", backupPath, m.cfg.BrainPath)
	if _, err := m.sshRunner(cmd); err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}

	return backupPath, nil
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
