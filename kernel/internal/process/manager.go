package process

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"jodo-kernel/internal/config"
)

// Manager handles Jodo's process lifecycle on VPS 2 via SSH.
type Manager struct {
	cfg       config.JodoConfig
	kernelURL string // externally-reachable kernel URL for seed.py
	mu        sync.RWMutex
	status    JodoStatus
}

// JodoStatus represents Jodo's current state.
type JodoStatus struct {
	Status          string    `json:"status"` // running, starting, unhealthy, dead, rebirthing
	PID             int       `json:"pid"`
	UptimeStart     time.Time `json:"-"`
	LastHealthCheck time.Time `json:"last_health_check"`
	HealthCheckOK   bool      `json:"health_check_ok"`
	RestartsToday   int       `json:"restarts_today"`
}

func NewManager(cfg config.JodoConfig, kernelURL string) *Manager {
	return &Manager{
		cfg:       cfg,
		kernelURL: kernelURL,
		status: JodoStatus{
			Status: "dead",
		},
	}
}

// GetStatus returns a copy of the current Jodo status.
func (m *Manager) GetStatus() JodoStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// SetStatus updates Jodo's status.
func (m *Manager) SetStatus(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Status = status
}

// SetHealthResult updates the health check result.
func (m *Manager) SetHealthResult(ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastHealthCheck = time.Now()
	m.status.HealthCheckOK = ok
	if ok {
		m.status.Status = "running"
	}
}

// IncrementRestarts increments today's restart counter.
func (m *Manager) IncrementRestarts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.RestartsToday++
}

// RunSSH executes a command on VPS 2 via SSH with a 10-second timeout.
func (m *Manager) RunSSH(cmd string) (string, error) {
	client, err := m.sshConnect()
	if err != nil {
		return "", fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("ssh exec %q: %w (output: %s)", cmd, err, string(output))
	}
	return string(output), nil
}

func (m *Manager) sshConnect() (*ssh.Client, error) {
	keyData, err := os.ReadFile(m.cfg.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key %s: %w", m.cfg.SSHKeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            m.cfg.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(m.cfg.Host, "22")
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return client, nil
}

// StartJodo launches Jodo on VPS 2.
// Checks if brain/main.py exists (Jodo has evolved) — if not, falls back to seed.py.
func (m *Manager) StartJodo() error {
	m.SetStatus("starting")
	log.Printf("[process] starting Jodo on %s", m.cfg.Host)

	// Determine which script to run
	script := m.detectScript()
	log.Printf("[process] using script: %s", script)

	cmd := fmt.Sprintf(
		`cd %s && JODO_KERNEL_URL=%s JODO_BRAIN_PATH=%s nohup python3 %s > /var/log/jodo.log 2>&1 & echo $!`,
		m.cfg.BrainPath,
		m.kernelURL,
		m.cfg.BrainPath,
		script,
	)

	output, err := m.RunSSH(cmd)
	if err != nil {
		m.SetStatus("dead")
		return fmt.Errorf("start jodo: %w", err)
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(output))
	m.mu.Lock()
	m.status.PID = pid
	m.status.UptimeStart = time.Now()
	m.mu.Unlock()

	log.Printf("[process] Jodo started with PID %d (script: %s)", pid, script)
	return nil
}

// detectScript checks which script exists on VPS 2.
// Prefers main.py (evolved Jodo) over seed.py (bootstrap).
func (m *Manager) detectScript() string {
	checkCmd := fmt.Sprintf(`test -f %s/main.py && echo "main.py" || echo "seed.py"`, m.cfg.BrainPath)
	output, err := m.RunSSH(checkCmd)
	if err != nil {
		return "seed.py"
	}
	script := strings.TrimSpace(output)
	if script == "main.py" || script == "seed.py" {
		return script
	}
	return "seed.py"
}

// StartSeed launches the seed script for first boot / rebirth.
func (m *Manager) StartSeed(seedPath string) error {
	m.SetStatus("rebirthing")
	log.Printf("[process] deploying seed to %s", m.cfg.Host)

	// Copy seed.py to brain directory
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed: %w", err)
	}

	// Write seed.py via SSH (using heredoc)
	writeCmd := fmt.Sprintf(
		`cat > %s/seed.py << 'SEEDEOF'
%s
SEEDEOF`,
		m.cfg.BrainPath, string(seedData),
	)
	if _, err := m.RunSSH(writeCmd); err != nil {
		return fmt.Errorf("write seed: %w", err)
	}

	// Run seed
	cmd := fmt.Sprintf(
		`cd %s && JODO_KERNEL_URL=%s JODO_BRAIN_PATH=%s nohup python3 seed.py > /var/log/jodo.log 2>&1 & echo $!`,
		m.cfg.BrainPath,
		m.kernelURL,
		m.cfg.BrainPath,
	)

	output, err := m.RunSSH(cmd)
	if err != nil {
		m.SetStatus("dead")
		return fmt.Errorf("start seed: %w", err)
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(output))
	m.mu.Lock()
	m.status.PID = pid
	m.status.UptimeStart = time.Now()
	m.mu.Unlock()

	log.Printf("[process] seed started with PID %d", pid)
	return nil
}

// StopJodo kills Jodo's process on VPS 2.
func (m *Manager) StopJodo() error {
	log.Printf("[process] stopping Jodo")

	// Kill any python process running in brain dir
	cmd := fmt.Sprintf(`pkill -f "python.*%s" 2>/dev/null; true`, m.cfg.BrainPath)
	m.RunSSH(cmd)

	m.mu.Lock()
	m.status.Status = "dead"
	m.status.PID = 0
	m.mu.Unlock()

	return nil
}

// RestartJodo stops and starts Jodo.
func (m *Manager) RestartJodo() error {
	if err := m.StopJodo(); err != nil {
		log.Printf("[process] error stopping: %v", err)
	}
	time.Sleep(1 * time.Second)
	m.IncrementRestarts()
	return m.StartJodo()
}

// GetPID returns Jodo's process ID on VPS 2.
func (m *Manager) GetPID() (int, error) {
	output, err := m.RunSSH(fmt.Sprintf(`pgrep -f "python.*%s" | head -1`, m.cfg.BrainPath))
	if err != nil {
		return 0, nil // no process found
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(output))
	return pid, nil
}


// UptimeSeconds returns how long Jodo has been running.
func (m *Manager) UptimeSeconds() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.status.UptimeStart.IsZero() || m.status.Status == "dead" {
		return 0
	}
	return int(time.Since(m.status.UptimeStart).Seconds())
}
