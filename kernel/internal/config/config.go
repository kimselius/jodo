package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
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
	Host           string `yaml:"host"`
	SSHUser        string `yaml:"ssh_user"`
	SSHKeyPath     string `yaml:"ssh_key_path"`
	Port           int    `yaml:"port"`    // seed.py health port (9001)
	AppPort        int    `yaml:"app_port"` // Jodo's app port (9000)
	BrainPath      string `yaml:"brain_path"`
	HealthEndpoint string `yaml:"health_endpoint"`
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
}

type ModelConfig struct {
	Model                 string   `yaml:"model"`                    // actual API model name; if empty, the map key is used
	InputCostPer1MTokens  float64  `yaml:"input_cost_per_1m_tokens"`
	OutputCostPer1MTokens float64  `yaml:"output_cost_per_1m_tokens"`
	Capabilities          []string `yaml:"capabilities"`
	Quality               int      `yaml:"quality"`
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
		Name    string `yaml:"name"`
		Version int    `yaml:"version"`
	} `yaml:"identity"`
	Purpose           string   `yaml:"purpose"`
	SurvivalInstincts []string `yaml:"survival_instincts"`
	Capabilities      struct {
		KernelAPI map[string]string `yaml:"kernel_api"`
		Local     []string          `yaml:"local"`
	} `yaml:"capabilities"`
	FirstTasks []string `yaml:"first_tasks"`
	Hints      []string `yaml:"hints"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvVars replaces ${VAR_NAME} with environment variable values.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})
}

// LoadConfig reads config.yaml and expands environment variables.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// LoadGenesis reads genesis.yaml.
func LoadGenesis(path string) (*Genesis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read genesis: %w", err)
	}

	var g Genesis
	if err := yaml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse genesis: %w", err)
	}

	if g.Identity.Name == "" {
		return nil, fmt.Errorf("genesis: identity.name is required")
	}

	return &g, nil
}

func (c *Config) validate() error {
	if c.Kernel.Port == 0 {
		c.Kernel.Port = 8080
	}
	if c.Kernel.HealthCheckInterval == 0 {
		c.Kernel.HealthCheckInterval = 10
	}
	if c.Kernel.MaxRestartAttempts == 0 {
		c.Kernel.MaxRestartAttempts = 3
	}
	if c.Jodo.Host == "" || strings.Contains(c.Jodo.Host, "${") {
		return fmt.Errorf("jodo.host must be set to Jodo's IP address (set JODO_IP env var)")
	}
	if c.Jodo.Port == 0 {
		c.Jodo.Port = 9001
	}
	if c.Jodo.AppPort == 0 {
		c.Jodo.AppPort = 9000
	}
	if c.Jodo.BrainPath == "" {
		c.Jodo.BrainPath = "/opt/jodo/brain"
	}
	if c.Jodo.HealthEndpoint == "" {
		c.Jodo.HealthEndpoint = "/health"
	}
	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}
	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider must be configured")
	}
	return nil
}
