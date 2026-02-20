package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Kernel    KernelConfig              `yaml:"kernel"`
	Jodo      JodoConfig                `yaml:"jodo"`
	Database  DatabaseConfig            `yaml:"database"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Routing   RoutingConfig             `yaml:"routing"`
}

type KernelConfig struct {
	Port                int    `yaml:"port"`
	ExternalURL         string `yaml:"external_url"` // how VPS 2 reaches the kernel, e.g. "http://1.2.3.4:8080"
	HealthCheckInterval int    `yaml:"health_check_interval"`
	MaxRestartAttempts  int    `yaml:"max_restart_attempts"`
	LogLevel            string `yaml:"log_level"`
	AuditLogPath        string `yaml:"audit_log_path"`
}

type JodoConfig struct {
	Host            string `yaml:"host"`
	SSHUser         string `yaml:"ssh_user"`
	SSHKeyPath      string `yaml:"ssh_key_path"`
	Port            int    `yaml:"port"`    // seed.py health port (9001)
	AppPort         int    `yaml:"app_port"` // Jodo's app port (9000)
	BrainPath       string `yaml:"brain_path"`
	HealthEndpoint  string `yaml:"health_endpoint"`
	MaxSubagents    int    `yaml:"max_subagents"`
	SubagentTimeout int    `yaml:"subagent_timeout"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type ProviderConfig struct {
	APIKey           string                 `yaml:"api_key"`
	BaseURL          string                 `yaml:"base_url"`
	Models           map[string]ModelConfig `yaml:"models"`
	MonthlyBudget    float64                `yaml:"monthly_budget"`
	EmergencyReserve float64                `yaml:"emergency_reserve"`
	TotalVRAMBytes   int64                  `yaml:"total_vram_bytes"` // 0 = no VRAM tracking
}

type ModelConfig struct {
	Model                 string   `yaml:"model"`                    // actual API model name; if empty, the map key is used
	InputCostPer1MTokens  float64  `yaml:"input_cost_per_1m_tokens"`
	OutputCostPer1MTokens float64  `yaml:"output_cost_per_1m_tokens"`
	Capabilities          []string `yaml:"capabilities"`
	Quality               int      `yaml:"quality"`
	VRAMEstimateBytes     int64    `yaml:"vram_estimate_bytes"` // approx VRAM when loaded
	SupportsTools         *bool    `yaml:"supports_tools"`      // nil = unknown
	PreferLoaded          bool     `yaml:"prefer_loaded"`       // use this model if already in VRAM
}

// ModelName returns the actual model identifier to send to the provider API.
// Uses the explicit Model field if set, otherwise falls back to the map key.
func (m ModelConfig) ModelName(mapKey string) string {
	if m.Model != "" {
		return m.Model
	}
	return mapKey
}

type RoutingConfig struct {
	Strategy          string              `yaml:"strategy"`
	IntentPreferences map[string][]string `yaml:"intent_preferences"`
}

type Genesis struct {
	Identity struct {
		Name    string `yaml:"name" json:"name"`
		Version int    `yaml:"version" json:"version"`
	} `yaml:"identity" json:"identity"`
	Purpose           string   `yaml:"purpose" json:"purpose"`
	SurvivalInstincts []string `yaml:"survival_instincts" json:"survival_instincts"`
	Capabilities      struct {
		KernelAPI map[string]string `yaml:"kernel_api" json:"kernel_api"`
		Local     []string          `yaml:"local" json:"local"`
	} `yaml:"capabilities" json:"capabilities"`
	FirstTasks []string `yaml:"first_tasks" json:"first_tasks"`
	Hints      []string `yaml:"hints" json:"hints"`
}

// ParseModelRef parses a "modelKey@provider" reference string.
// Returns modelKey, providerName, ok. If no "@", treats the whole string as provider name (backward compat).
func ParseModelRef(ref string) (modelKey, providerName string, ok bool) {
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	// Backward compatible: treat as provider name
	return "", ref, false
}

// LoadDatabaseConfig loads only the database configuration from environment variables.
// This is the bootstrap config needed before the DB is available.
func LoadDatabaseConfig() DatabaseConfig {
	port := 5432
	if v := os.Getenv("DB_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &port)
	}

	return DatabaseConfig{
		Host:     envOrDefault("DB_HOST", "postgres"),
		Port:     port,
		User:     envOrDefault("DB_USER", "jodo"),
		Password: os.Getenv("JODO_DB_PASSWORD"),
		Name:     envOrDefault("DB_NAME", "jodo_kernel"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
