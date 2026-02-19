package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"jodo-kernel/internal/config"
	"jodo-kernel/internal/crypto"
)

// GET /api/setup/status
func (s *Server) handleSetupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"setup_complete": s.SetupComplete,
	})
}

// POST /api/setup/ssh/generate
func (s *Server) handleSetupSSHGenerate(c *gin.Context) {
	keyPair, err := crypto.GenerateSSHKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate key: %v", err)})
		return
	}

	// Store private key encrypted in DB
	if err := s.ConfigStore.SaveSecret("ssh_private_key", keyPair.PrivateKeyPEM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("store key: %v", err)})
		return
	}

	log.Println("[setup] SSH keypair generated and stored")
	c.JSON(http.StatusOK, gin.H{
		"public_key": keyPair.PublicKeySSH,
	})
}

// POST /api/setup/ssh/verify
func (s *Server) handleSetupSSHVerify(c *gin.Context) {
	var req struct {
		Host    string `json:"host"`
		SSHUser string `json:"ssh_user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host is required"})
		return
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}

	// Read private key from DB
	privKeyPEM, err := s.ConfigStore.GetSecret("ssh_private_key")
	if err != nil || privKeyPEM == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no SSH key generated yet"})
		return
	}

	signer, err := ssh.ParsePrivateKey([]byte(privKeyPEM))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored SSH key"})
		return
	}

	sshConfig := &ssh.ClientConfig{
		User:            req.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(req.Host, "22")
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     fmt.Sprintf("SSH connection failed: %v", err),
		})
		return
	}
	defer client.Close()

	// Test a command
	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     "SSH connected but failed to open session",
		})
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput("echo ok")
	if err != nil || string(output) != "ok\n" {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     "SSH connected but command execution failed",
		})
		return
	}

	// Save VPS config
	s.ConfigStore.SetConfig("jodo.host", req.Host)
	s.ConfigStore.SetConfig("jodo.ssh_user", req.SSHUser)

	log.Printf("[setup] SSH verified: %s@%s", req.SSHUser, req.Host)
	c.JSON(http.StatusOK, gin.H{
		"connected": true,
	})
}

// POST /api/setup/config — save kernel URL and other config
func (s *Server) handleSetupConfig(c *gin.Context) {
	var req struct {
		KernelURL string `json:"kernel_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.KernelURL != "" {
		s.ConfigStore.SetConfig("kernel.external_url", req.KernelURL)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/setup/providers — save provider configurations
func (s *Server) handleSetupProviders(c *gin.Context) {
	var req struct {
		Providers []providerSetupReq `json:"providers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	for _, p := range req.Providers {
		if err := s.ConfigStore.SaveProvider(p.Name, p.Enabled, p.APIKey, p.BaseURL, p.MonthlyBudget, p.EmergencyReserve); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save provider %s: %v", p.Name, err)})
			return
		}
		for _, m := range p.Models {
			if err := s.ConfigStore.SaveModel(p.Name, m.ModelKey, m.ModelName, m.InputCostPer1M, m.OutputCostPer1M, m.Capabilities, m.Quality); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save model %s/%s: %v", p.Name, m.ModelKey, err)})
				return
			}
		}
	}

	log.Printf("[setup] saved %d providers", len(req.Providers))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type providerSetupReq struct {
	Name             string          `json:"name"`
	Enabled          bool            `json:"enabled"`
	APIKey           string          `json:"api_key"`
	BaseURL          string          `json:"base_url"`
	MonthlyBudget    float64         `json:"monthly_budget"`
	EmergencyReserve float64         `json:"emergency_reserve"`
	Models           []modelSetupReq `json:"models"`
}

type modelSetupReq struct {
	ModelKey       string   `json:"model_key"`
	ModelName      string   `json:"model_name"`
	InputCostPer1M float64  `json:"input_cost_per_1m"`
	OutputCostPer1M float64 `json:"output_cost_per_1m"`
	Capabilities   []string `json:"capabilities"`
	Quality        int      `json:"quality"`
}

// POST /api/setup/genesis — save genesis config
func (s *Server) handleSetupGenesis(c *gin.Context) {
	var req genesisSetupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	genesis := req.toGenesis()
	if err := s.ConfigStore.SaveGenesis(genesis); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save genesis: %v", err)})
		return
	}

	log.Printf("[setup] genesis saved: %s", genesis.Identity.Name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type genesisSetupReq struct {
	Name              string              `json:"name"`
	Purpose           string              `json:"purpose"`
	SurvivalInstincts []string            `json:"survival_instincts"`
	FirstTasks        []string            `json:"first_tasks"`
	Hints             []string            `json:"hints"`
	CapabilitiesAPI   map[string]string   `json:"capabilities_api"`
	CapabilitiesLocal []string            `json:"capabilities_local"`
}

func (r *genesisSetupReq) toGenesis() *config.Genesis {
	g := &config.Genesis{
		Purpose:           r.Purpose,
		SurvivalInstincts: r.SurvivalInstincts,
		FirstTasks:        r.FirstTasks,
		Hints:             r.Hints,
	}
	g.Identity.Name = r.Name
	if g.Identity.Name == "" {
		g.Identity.Name = "Jodo"
	}
	g.Capabilities.KernelAPI = r.CapabilitiesAPI
	g.Capabilities.Local = r.CapabilitiesLocal
	return g
}

// POST /api/setup/test-provider — test an API key
func (s *Server) handleSetupTestProvider(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	switch req.Provider {
	case "ollama":
		url := req.BaseURL
		if url == "" {
			url = "http://host.docker.internal:11434"
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": fmt.Sprintf("Cannot reach Ollama at %s: %v", url, err)})
			return
		}
		resp.Body.Close()
		c.JSON(http.StatusOK, gin.H{"valid": resp.StatusCode == 200})

	case "claude":
		if req.APIKey == "" {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": "API key is required"})
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, _ := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)
		httpReq.Header.Set("x-api-key", req.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": fmt.Sprintf("API request failed: %v", err)})
			return
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			c.JSON(http.StatusOK, gin.H{"valid": true})
		} else {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": fmt.Sprintf("API returned status %d", resp.StatusCode)})
		}

	case "openai":
		if req.APIKey == "" {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": "API key is required"})
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": fmt.Sprintf("API request failed: %v", err)})
			return
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			c.JSON(http.StatusOK, gin.H{"valid": true})
		} else {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": fmt.Sprintf("API returned status %d", resp.StatusCode)})
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown provider"})
	}
}

// POST /api/setup/birth — mark setup complete and birth Jodo
func (s *Server) handleSetupBirth(c *gin.Context) {
	if s.SetupComplete {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jodo is already born"})
		return
	}

	// Set defaults for any missing config
	defaults := map[string]string{
		"kernel.port":                  "8080",
		"kernel.health_check_interval": "10",
		"kernel.max_restart_attempts":  "3",
		"kernel.audit_log_path":        "/var/log/jodo-audit.jsonl",
		"jodo.port":                    "9001",
		"jodo.app_port":                "9000",
		"jodo.brain_path":              "/opt/jodo/brain",
		"jodo.health_endpoint":         "/health",
		"routing.strategy":             "best_affordable",
	}
	for k, v := range defaults {
		if s.ConfigStore.GetConfig(k) == "" {
			s.ConfigStore.SetConfig(k, v)
		}
	}

	// Set default routing intent preferences if not set
	if s.ConfigStore.GetConfig("routing.intent_preferences") == "" {
		s.ConfigStore.SetConfig("routing.intent_preferences",
			`{"code":["ollama","claude","openai"],"chat":["ollama","openai","claude"],"embed":["ollama","openai"],"quick":["ollama","openai"],"repair":["claude","ollama","openai"]}`)
	}

	// Mark setup complete
	if err := s.ConfigStore.SetConfig("setup_complete", "true"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save setup status"})
		return
	}

	// Birth Jodo
	if s.OnBirth != nil {
		if err := s.OnBirth(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("birth failed: %v", err)})
			return
		}
	}

	log.Println("[setup] Setup complete! Jodo is being born.")
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Jodo is being born!"})
}
