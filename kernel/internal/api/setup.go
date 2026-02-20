package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"golang.org/x/crypto/ssh"
	"jodo-kernel/internal/config"
	"jodo-kernel/internal/crypto"
)

// GET /api/setup/status
func (s *Server) handleSetupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"setup_complete": s.SetupComplete,
		"jodo_mode":      s.JodoMode,
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

	log.Printf("[setup] SSH verified: %s@%s", req.SSHUser, req.Host)
	c.JSON(http.StatusOK, gin.H{
		"connected": true,
	})
}

// POST /api/setup/step/:name — save config for a setup step.
// Each step saves via one HTTP call when the user clicks "Next".
func (s *Server) handleSetupSaveStep(c *gin.Context) {
	name := c.Param("name")

	switch name {
	case "vps":
		var req struct {
			Host    string `json:"host"`
			SSHUser string `json:"ssh_user"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if err := s.saveConnectionConfig(req.Host, req.SSHUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "server-setup":
		var req struct {
			BrainPath string `json:"brain_path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BrainPath == "" {
			req.BrainPath = "/opt/jodo/brain"
		}
		if err := s.ConfigStore.SetConfig("jodo.brain_path", req.BrainPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[setup] brain path saved: %s", req.BrainPath)

	case "kernel-url":
		var req struct {
			KernelURL string `json:"kernel_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.KernelURL != "" {
			if err := s.ConfigStore.SetConfig("kernel.external_url", req.KernelURL); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			log.Printf("[setup] kernel URL saved: %s", req.KernelURL)
		}

	case "providers":
		var req struct {
			Providers []providerSetupReq `json:"providers"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if err := s.saveProvidersBulk(req.Providers); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "routing":
		var req struct {
			IntentPreferences map[string][]string `json:"intent_preferences"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if err := s.saveRoutingPreferences(req.IntentPreferences); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "genesis":
		var req genesisSetupReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if _, err := s.saveGenesisConfig(req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unknown setup step: %s", name)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type providerSetupReq struct {
	Name             string          `json:"name"`
	Enabled          bool            `json:"enabled"`
	APIKey           string          `json:"api_key"`
	BaseURL          string          `json:"base_url"`
	MonthlyBudget    float64         `json:"monthly_budget"`
	EmergencyReserve float64         `json:"emergency_reserve"`
	TotalVRAMBytes   int64           `json:"total_vram_bytes"`
	Models           []modelSetupReq `json:"models"`
}

type modelSetupReq struct {
	ModelKey          string   `json:"model_key"`
	ModelName         string   `json:"model_name"`
	InputCostPer1M    float64  `json:"input_cost_per_1m"`
	OutputCostPer1M   float64  `json:"output_cost_per_1m"`
	Capabilities      []string `json:"capabilities"`
	Quality           int      `json:"quality"`
	VRAMEstimateBytes int64    `json:"vram_estimate_bytes"`
	SupportsTools     *bool    `json:"supports_tools"`
	PreferLoaded      bool     `json:"prefer_loaded"`
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

// POST /api/setup/birth — mark setup complete and birth Jodo
func (s *Server) handleSetupBirth(c *gin.Context) {
	if s.SetupComplete {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jodo is already born"})
		return
	}

	// Set system defaults (not user-configurable in wizard — user data saved per-step)
	defaults := map[string]string{
		"kernel.port":                  "8080",
		"kernel.health_check_interval": "10",
		"kernel.max_restart_attempts":  "3",
		"kernel.audit_log_path":        "/var/log/jodo-audit.jsonl",
		"jodo.port":                    "9001",
		"jodo.app_port":                "9000",
		"jodo.health_endpoint":         "/health",
	}
	for k, v := range defaults {
		if s.ConfigStore.GetConfig(k) == "" {
			s.ConfigStore.SetConfig(k, v)
		}
	}

	// Ensure routing preferences are populated for all intents with available models.
	// Merges: keeps any existing preferences from the setup wizard, fills in gaps from models.
	builtPrefs := s.buildRoutingFromModels()
	if len(builtPrefs) > 0 {
		existing := make(map[string][]string)
		if existingJSON := s.ConfigStore.GetConfig("routing.intent_preferences"); existingJSON != "" {
			json.Unmarshal([]byte(existingJSON), &existing)
		}

		for intent, models := range builtPrefs {
			if len(existing[intent]) == 0 {
				existing[intent] = models
			}
		}

		prefsJSON, _ := json.Marshal(existing)
		s.ConfigStore.SetConfig("routing.intent_preferences", string(prefsJSON))
		log.Printf("[setup] routing preferences: %s", string(prefsJSON))
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

// POST /api/setup/provision — SSH into VPS and set up the brain directory
func (s *Server) handleSetupProvision(c *gin.Context) {
	var req struct {
		BrainPath string `json:"brain_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.BrainPath = "/opt/jodo/brain"
	}
	if req.BrainPath == "" {
		req.BrainPath = "/opt/jodo/brain"
	}

	// Get SSH credentials (saved by step/vps on the previous Next click)
	host := s.ConfigStore.GetConfig("jodo.host")
	sshUser := s.ConfigStore.GetConfig("jodo.ssh_user")
	if host == "" || sshUser == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VPS connection not configured yet"})
		return
	}

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
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(host, "22")
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"steps":   []gin.H{{"name": "SSH connect", "ok": false, "output": err.Error()}},
		})
		return
	}
	defer client.Close()

	// Run provisioning steps
	type step struct {
		Name string
		Cmd  string
	}
	steps := []step{
		{"Create directory", fmt.Sprintf("mkdir -p %s", req.BrainPath)},
		{"Initialize git", fmt.Sprintf("cd %s && if [ ! -d .git ]; then git init && git config user.name 'Jodo' && git config user.email 'jodo@localhost' && echo 'initialized'; else echo 'already initialized'; fi", req.BrainPath)},
		{"Check Python3", "python3 --version"},
		{"Check pip", "pip3 --version"},
		{"Check git", "git --version"},
	}

	var results []gin.H
	allOk := true
	for _, s := range steps {
		session, err := client.NewSession()
		if err != nil {
			results = append(results, gin.H{"name": s.Name, "ok": false, "output": "failed to open SSH session"})
			allOk = false
			continue
		}
		output, err := session.CombinedOutput(s.Cmd)
		session.Close()

		ok := err == nil
		if !ok {
			allOk = false
		}
		results = append(results, gin.H{"name": s.Name, "ok": ok, "output": strings.TrimSpace(string(output))})
	}

	log.Printf("[setup] provision %s@%s brain=%s success=%v", sshUser, host, req.BrainPath, allOk)
	c.JSON(http.StatusOK, gin.H{"success": allOk, "steps": results})
}

// buildRoutingFromModels queries saved models and builds intent_preferences
// based on each model's capabilities, sorted by quality (highest first).
func (s *Server) buildRoutingFromModels() map[string][]string {
	rows, err := s.DB.Query(
		`SELECT pm.model_key, pm.provider_name, pm.capabilities, pm.quality
		 FROM provider_models pm
		 JOIN providers p ON p.name = pm.provider_name
		 WHERE p.enabled = true AND pm.enabled = true
		 ORDER BY pm.quality DESC`,
	)
	if err != nil {
		log.Printf("[setup] failed to query models for routing: %v", err)
		return nil
	}
	defer rows.Close()

	type modelEntry struct {
		ref     string // model_key@provider
		quality int
	}

	// Valid intents — only these become routing keys (not raw capabilities like "tools")
	validIntents := map[string]bool{"code": true, "plan": true, "chat": true, "embed": true}

	// Collect models per intent (filtering to valid intents only)
	capModels := make(map[string][]modelEntry)
	for rows.Next() {
		var modelKey, providerName string
		var caps []string
		var quality int
		if err := rows.Scan(&modelKey, &providerName, pq.Array(&caps), &quality); err != nil {
			continue
		}
		ref := modelKey + "@" + providerName
		for _, cap := range caps {
			if validIntents[cap] {
				capModels[cap] = append(capModels[cap], modelEntry{ref: ref, quality: quality})
			}
		}
	}

	// Sort each capability's models by quality descending
	prefs := make(map[string][]string)
	for cap, entries := range capModels {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].quality > entries[j].quality
		})
		refs := make([]string, len(entries))
		for i, e := range entries {
			refs[i] = e.ref
		}
		prefs[cap] = refs
	}

	return prefs
}

// POST /api/setup/discover — model discovery during setup (before setup is complete)
func (s *Server) handleSetupDiscover(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	switch req.Provider {
	case "ollama":
		baseURL := req.BaseURL
		if baseURL == "" {
			baseURL = "http://host.docker.internal:11434"
		}
		s.discoverOllamaModelsWithURL(c, baseURL)
	case "claude":
		if req.APIKey == "" {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "Enter an API key first, then discover models"})
			return
		}
		discoverClaudeModels(c, req.APIKey)
	case "openai":
		if req.APIKey == "" {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "error": "Enter an API key first, then discover models"})
			return
		}
		discoverOpenAIModels(c, req.APIKey)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown provider"})
	}
}

