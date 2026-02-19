package process

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"jodo-kernel/internal/config"
)

// Manager handles Jodo's process lifecycle on VPS 2 via SSH.
type Manager struct {
	cfg              config.JodoConfig
	kernelURL        string // externally-reachable kernel URL for seed.py
	mu               sync.RWMutex
	status           JodoStatus
	gracePeriodUntil time.Time // health checks don't escalate before this time
}

// JodoStatus represents Jodo's current state.
type JodoStatus struct {
	Status          string    `json:"status"` // running, starting, unhealthy, dead, rebirthing
	PID             int       `json:"pid"`
	Galla           int       `json:"galla"`
	Phase           string    `json:"phase"` // booting, thinking, sleeping
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
// On first success with unknown PID, discovers it via SSH.
func (m *Manager) SetHealthResult(ok bool) {
	m.mu.Lock()
	needsPID := ok && m.status.PID == 0
	if ok {
		m.status.Status = "running"
		if m.status.UptimeStart.IsZero() {
			m.status.UptimeStart = time.Now()
		}
	}
	m.status.LastHealthCheck = time.Now()
	m.status.HealthCheckOK = ok
	m.mu.Unlock()

	// Discover PID outside the lock (SSH is slow)
	if needsPID {
		if pid, err := m.GetPID(); err == nil && pid > 0 {
			m.mu.Lock()
			m.status.PID = pid
			m.mu.Unlock()
			log.Printf("[process] discovered Jodo PID: %d", pid)
		}
	}
}

// SetHeartbeat updates galla and phase from seed.py's heartbeat POST.
func (m *Manager) SetHeartbeat(galla int, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Galla = galla
	m.status.Phase = phase
}

// IncrementRestarts increments today's restart counter.
func (m *Manager) IncrementRestarts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.RestartsToday++
}

// SetGracePeriod sets a window during which health check failures won't escalate.
func (m *Manager) SetGracePeriod(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gracePeriodUntil = time.Now().Add(d)
}

// InGracePeriod returns true if we're within a post-restart grace period.
func (m *Manager) InGracePeriod() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Now().Before(m.gracePeriodUntil)
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

// StartJodo launches seed.py on VPS 2.
// seed.py is Jodo's consciousness — it always runs. Jodo manages his own apps.
func (m *Manager) StartJodo() error {
	m.SetStatus("starting")
	log.Printf("[process] starting seed.py on %s", m.cfg.Host)

	cmd := fmt.Sprintf(
		`cd %s && nohup python3 %s/seed.py > /var/log/jodo.log 2>&1 & echo $!`,
		m.cfg.BrainPath,
		m.cfg.BrainPath,
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

	m.SetGracePeriod(30 * time.Second)
	log.Printf("[process] seed.py started with PID %d", pid)
	return nil
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

	// Merge prompt files into seed template
	promptsDir := filepath.Join(filepath.Dir(seedPath), "prompts")
	if entries, err := os.ReadDir(promptsDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".prompt") {
				name := strings.TrimSuffix(entry.Name(), ".prompt")
				marker := "__PROMPT_" + strings.ToUpper(name) + "__"
				if content, err := os.ReadFile(filepath.Join(promptsDir, entry.Name())); err == nil {
					seedData = bytes.Replace(seedData, []byte(marker), content, 1)
				}
			}
		}
		log.Printf("[process] merged prompt files from %s", promptsDir)
	}

	// Replace config markers with actual values
	seedData = bytes.ReplaceAll(seedData, []byte("__KERNEL_URL__"), []byte(m.kernelURL))
	seedData = bytes.ReplaceAll(seedData, []byte("__BRAIN_PATH__"), []byte(m.cfg.BrainPath))
	seedData = bytes.ReplaceAll(seedData, []byte("__SEED_PORT__"), []byte(strconv.Itoa(m.cfg.Port)))
	seedData = bytes.ReplaceAll(seedData, []byte("__APP_PORT__"), []byte(strconv.Itoa(m.cfg.AppPort)))
	log.Printf("[process] replaced config markers (kernel=%s, brain=%s, seed_port=%d, app_port=%d)",
		m.kernelURL, m.cfg.BrainPath, m.cfg.Port, m.cfg.AppPort)

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

	// Run seed (use absolute path so pkill -f can match reliably)
	cmd := fmt.Sprintf(
		`cd %s && nohup python3 %s/seed.py > /var/log/jodo.log 2>&1 & echo $!`,
		m.cfg.BrainPath,
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

	m.SetGracePeriod(30 * time.Second)
	log.Printf("[process] seed started with PID %d", pid)
	return nil
}

// StopSeed kills only seed.py, leaving Jodo's apps (main.py etc.) running.
func (m *Manager) StopSeed() error {
	log.Printf("[process] stopping seed.py only")

	// Try killing by stored PID first (most reliable)
	m.mu.RLock()
	pid := m.status.PID
	m.mu.RUnlock()

	if pid > 0 {
		killCmd := fmt.Sprintf(`kill %d 2>/dev/null; true`, pid)
		m.RunSSH(killCmd)
	}

	// Also pkill by pattern — matches "python3 /opt/jodo/brain/seed.py" and "python3 seed.py"
	cmd := `pkill -f "python3.*seed\.py" 2>/dev/null; true`
	m.RunSSH(cmd)

	m.mu.Lock()
	m.status.PID = 0
	m.mu.Unlock()
	return nil
}

// StopAll kills ALL python processes in the brain dir (seed.py + any apps).
func (m *Manager) StopAll() error {
	log.Printf("[process] stopping all Jodo processes")

	// Kill by stored PID
	m.mu.RLock()
	pid := m.status.PID
	m.mu.RUnlock()
	if pid > 0 {
		killCmd := fmt.Sprintf(`kill %d 2>/dev/null; true`, pid)
		m.RunSSH(killCmd)
	}

	// Kill any python process referencing the brain path (absolute path launches)
	cmd := fmt.Sprintf(`pkill -f "python.*%s" 2>/dev/null; true`, m.cfg.BrainPath)
	m.RunSSH(cmd)

	// Also catch any relative-path processes (legacy or Jodo-launched)
	cmd2 := `pkill -f "python3.*seed\.py" 2>/dev/null; pkill -f "python3.*main\.py" 2>/dev/null; true`
	m.RunSSH(cmd2)

	m.mu.Lock()
	m.status.Status = "dead"
	m.status.PID = 0
	m.mu.Unlock()
	return nil
}

// RestartJodo restarts seed.py without killing Jodo's apps.
// seed.py will detect its apps and resume the galla loop.
func (m *Manager) RestartJodo() error {
	if err := m.StopSeed(); err != nil {
		log.Printf("[process] error stopping seed: %v", err)
	}
	time.Sleep(1 * time.Second)
	m.IncrementRestarts()
	return m.StartJodo()
}

// NuclearRestart kills everything and starts fresh.
func (m *Manager) NuclearRestart() error {
	if err := m.StopAll(); err != nil {
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

// WriteInbox posts a message to seed.py's inbox endpoint.
// This is how the kernel communicates with Jodo between gallas.
func (m *Manager) WriteInbox(message string) error {
	url := fmt.Sprintf("http://%s:%d/inbox", m.cfg.Host, m.cfg.Port)
	body := fmt.Sprintf(`{"message":%q,"source":"kernel"}`, message)

	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("write inbox: %w", err)
	}
	resp.Body.Close()

	log.Printf("[inbox] wrote to Jodo: %s", message)
	return nil
}
