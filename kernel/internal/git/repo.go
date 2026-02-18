package git

import (
	"fmt"
	"strings"
	"time"

	"jodo-kernel/internal/config"
)

// Manager handles git operations on Jodo's brain/ repo via SSH.
type Manager struct {
	cfg       config.JodoConfig
	sshRunner func(cmd string) (string, error)
}

func NewManager(cfg config.JodoConfig, sshRunner func(cmd string) (string, error)) *Manager {
	return &Manager{cfg: cfg, sshRunner: sshRunner}
}

// Init ensures the git repo is initialized on VPS 2.
func (m *Manager) Init() error {
	cmd := fmt.Sprintf(
		`cd %s && if [ ! -d .git ]; then git init && git config user.name "Jodo" && git config user.email "jodo@localhost"; fi`,
		m.cfg.BrainPath,
	)
	_, err := m.sshRunner(cmd)
	return err
}

// Commit stages all changes and commits with the given message.
func (m *Manager) Commit(message string) (string, error) {
	cmd := fmt.Sprintf(
		`cd %s && git add -A && git diff --cached --quiet && echo "NOTHING_TO_COMMIT" || git commit -m %q`,
		m.cfg.BrainPath, message,
	)
	output, err := m.sshRunner(cmd)
	if err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	if strings.Contains(output, "NOTHING_TO_COMMIT") {
		return "", nil
	}

	// Get the commit hash
	hash, err := m.sshRunner(fmt.Sprintf("cd %s && git rev-parse --short HEAD", m.cfg.BrainPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(hash), nil
}

// Tag creates a git tag.
func (m *Manager) Tag(tag string) error {
	cmd := fmt.Sprintf("cd %s && git tag %s", m.cfg.BrainPath, tag)
	_, err := m.sshRunner(cmd)
	return err
}

// CurrentHash returns the current HEAD short hash.
func (m *Manager) CurrentHash() (string, error) {
	output, err := m.sshRunner(fmt.Sprintf("cd %s && git rev-parse --short HEAD 2>/dev/null || echo ''", m.cfg.BrainPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// CurrentTag returns the tag pointing at HEAD, if any.
func (m *Manager) CurrentTag() (string, error) {
	output, err := m.sshRunner(fmt.Sprintf("cd %s && git describe --tags --exact-match HEAD 2>/dev/null || echo ''", m.cfg.BrainPath))
	if err != nil {
		return "", nil // not an error if no tag
	}
	return strings.TrimSpace(output), nil
}

// Commit represents a git log entry.
type CommitEntry struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Tag       string `json:"tag,omitempty"`
}

// Log returns recent git commits.
func (m *Manager) Log(limit int) ([]CommitEntry, error) {
	if limit == 0 {
		limit = 20
	}
	cmd := fmt.Sprintf(
		`cd %s && git log --format="%%h|||%%s|||%%aI" -n %d 2>/dev/null || echo ''`,
		m.cfg.BrainPath, limit,
	)
	output, err := m.sshRunner(cmd)
	if err != nil {
		return nil, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return []CommitEntry{}, nil
	}

	// Get all tags
	tagsOutput, _ := m.sshRunner(fmt.Sprintf("cd %s && git tag -l --format='%%(objectname:short) %%(*objectname:short) %%(refname:short)' 2>/dev/null || echo ''", m.cfg.BrainPath))
	tagMap := parseTagMap(tagsOutput)

	var entries []CommitEntry
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "|||", 3)
		if len(parts) != 3 {
			continue
		}
		entry := CommitEntry{
			Hash:      parts[0],
			Message:   parts[1],
			Timestamp: parts[2],
		}
		if tag, ok := tagMap[entry.Hash]; ok {
			entry.Tag = tag
		}
		entries = append(entries, entry)
	}

	if entries == nil {
		entries = []CommitEntry{}
	}
	return entries, nil
}

func parseTagMap(output string) map[string]string {
	tags := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := fields[0]
			tag := fields[len(fields)-1]
			tags[hash] = tag
		}
	}
	return tags
}

// LastStableTag returns the most recent stable-* tag.
func (m *Manager) LastStableTag() (string, error) {
	output, err := m.sshRunner(fmt.Sprintf(
		`cd %s && git tag -l 'stable-*' --sort=-version:refname | head -1`,
		m.cfg.BrainPath,
	))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// HasBrainMain checks if brain/main.py exists on VPS 2.
func (m *Manager) HasBrainMain() (bool, error) {
	output, err := m.sshRunner(fmt.Sprintf("test -f %s/main.py && echo 'yes' || echo 'no'", m.cfg.BrainPath))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "yes", nil
}

// StableTagCount returns the number of existing stable tags for generating the next one.
func (m *Manager) StableTagCount() (int, error) {
	output, err := m.sshRunner(fmt.Sprintf(
		`cd %s && git tag -l 'stable-*' | wc -l`,
		m.cfg.BrainPath,
	))
	if err != nil {
		return 0, err
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &count)
	return count, nil
}

// HealthySince returns how many seconds ago the brain was last modified.
// Used to determine if Jodo has been stable enough to tag.
func (m *Manager) LastModifiedAgo() (time.Duration, error) {
	output, err := m.sshRunner(fmt.Sprintf(
		`cd %s && git log -1 --format=%%aI 2>/dev/null || echo ''`,
		m.cfg.BrainPath,
	))
	if err != nil {
		return 0, err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, nil
	}
	t, err := time.Parse(time.RFC3339, output)
	if err != nil {
		return 0, err
	}
	return time.Since(t), nil
}
