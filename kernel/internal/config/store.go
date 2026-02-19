package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/lib/pq"
	"jodo-kernel/internal/crypto"
)

// DBStore reads and writes configuration from the database.
// It produces the same Config and Genesis structs that downstream code expects.
type DBStore struct {
	db        *sql.DB
	encryptor *crypto.Encryptor
}

// NewDBStore creates a new database-backed config store.
func NewDBStore(db *sql.DB, enc *crypto.Encryptor) *DBStore {
	return &DBStore{db: db, encryptor: enc}
}

// --- system_config helpers ---

// SetConfig stores a key-value pair in system_config (upsert).
func (s *DBStore) SetConfig(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO system_config (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

// GetConfig retrieves a value from system_config. Returns "" if not found.
func (s *DBStore) GetConfig(key string) string {
	var value string
	err := s.db.QueryRow(`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

// GetConfigInt retrieves an integer value from system_config with a default fallback.
func (s *DBStore) GetConfigInt(key string, fallback int) int {
	v := s.GetConfig(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}

// IsSetupComplete returns true if the setup wizard has been completed.
func (s *DBStore) IsSetupComplete() bool {
	return s.GetConfig("setup_complete") == "true"
}

// --- secrets helpers ---

// SaveSecret encrypts and stores a secret.
func (s *DBStore) SaveSecret(key, plaintext string) error {
	encrypted, err := s.encryptor.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO secrets (key, value_encrypted, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value_encrypted = $2, updated_at = NOW()`,
		key, encrypted,
	)
	return err
}

// GetSecret decrypts and returns a secret. Returns "" if not found.
func (s *DBStore) GetSecret(key string) (string, error) {
	var encrypted []byte
	err := s.db.QueryRow(`SELECT value_encrypted FROM secrets WHERE key = $1`, key).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.encryptor.Decrypt(encrypted)
}

// --- provider CRUD ---

// SaveProvider stores a provider config. If apiKey is non-empty, it's encrypted.
func (s *DBStore) SaveProvider(name string, enabled bool, apiKey, baseURL string, monthlyBudget, emergencyReserve float64) error {
	var apiKeyEnc []byte
	if apiKey != "" {
		var err error
		apiKeyEnc, err = s.encryptor.Encrypt(apiKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
	}

	_, err := s.db.Exec(
		`INSERT INTO providers (name, enabled, api_key_encrypted, base_url, monthly_budget, emergency_reserve, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (name) DO UPDATE SET
		   enabled = $2, api_key_encrypted = COALESCE($3, providers.api_key_encrypted),
		   base_url = $4, monthly_budget = $5, emergency_reserve = $6, updated_at = NOW()`,
		name, enabled, apiKeyEnc, nullIfEmpty(baseURL), monthlyBudget, emergencyReserve,
	)
	return err
}

// SaveModel stores a model config for a provider.
func (s *DBStore) SaveModel(providerName string, modelKey, modelName string, inputCost, outputCost float64, capabilities []string, quality int) error {
	_, err := s.db.Exec(
		`INSERT INTO provider_models (provider_name, model_key, model_name, input_cost_per_1m, output_cost_per_1m, capabilities, quality)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (provider_name, model_key) DO UPDATE SET
		   model_name = $3, input_cost_per_1m = $4, output_cost_per_1m = $5, capabilities = $6, quality = $7`,
		providerName, modelKey, modelName, inputCost, outputCost, pq.Array(capabilities), quality,
	)
	return err
}

// GetModel returns a single model's info.
func (s *DBStore) GetModel(providerName, modelKey string) (*ModelInfo, error) {
	var m ModelInfo
	err := s.db.QueryRow(
		`SELECT model_key, model_name, input_cost_per_1m, output_cost_per_1m, capabilities, quality, enabled
		 FROM provider_models WHERE provider_name = $1 AND model_key = $2`,
		providerName, modelKey,
	).Scan(&m.ModelKey, &m.ModelName, &m.InputCostPer1M, &m.OutputCostPer1M, pq.Array(&m.Capabilities), &m.Quality, &m.Enabled)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// SetModelEnabled toggles a model's enabled status.
func (s *DBStore) SetModelEnabled(providerName, modelKey string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE provider_models SET enabled = $3 WHERE provider_name = $1 AND model_key = $2`,
		providerName, modelKey, enabled,
	)
	return err
}

// DeleteModel removes a specific model from a provider.
func (s *DBStore) DeleteModel(providerName, modelKey string) error {
	_, err := s.db.Exec(
		`DELETE FROM provider_models WHERE provider_name = $1 AND model_key = $2`,
		providerName, modelKey,
	)
	return err
}

// --- genesis CRUD ---

// SaveGenesis upserts the genesis (single-row table).
func (s *DBStore) SaveGenesis(g *Genesis) error {
	capAPI, _ := json.Marshal(g.Capabilities.KernelAPI)

	_, err := s.db.Exec(
		`INSERT INTO genesis (id, name, version, purpose, survival_instincts, capabilities_api, capabilities_local, first_tasks, hints, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   name = $1, version = $2, purpose = $3, survival_instincts = $4,
		   capabilities_api = $5, capabilities_local = $6, first_tasks = $7, hints = $8, updated_at = NOW()`,
		g.Identity.Name, g.Identity.Version, g.Purpose,
		pq.Array(g.SurvivalInstincts), capAPI,
		pq.Array(g.Capabilities.Local), pq.Array(g.FirstTasks), pq.Array(g.Hints),
	)
	return err
}

// LoadGenesis reads the genesis from the database.
func (s *DBStore) LoadGenesis() (*Genesis, error) {
	var g Genesis
	var capAPIRaw []byte
	var instincts, local, tasks, hints []string

	err := s.db.QueryRow(
		`SELECT name, version, purpose, survival_instincts, capabilities_api, capabilities_local, first_tasks, hints FROM genesis WHERE id = 1`,
	).Scan(
		&g.Identity.Name, &g.Identity.Version, &g.Purpose,
		pq.Array(&instincts), &capAPIRaw,
		pq.Array(&local), pq.Array(&tasks), pq.Array(&hints),
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("genesis not found in database")
	}
	if err != nil {
		return nil, fmt.Errorf("load genesis: %w", err)
	}

	g.SurvivalInstincts = instincts
	g.Capabilities.Local = local
	g.FirstTasks = tasks
	g.Hints = hints

	if capAPIRaw != nil {
		json.Unmarshal(capAPIRaw, &g.Capabilities.KernelAPI)
	}

	return &g, nil
}

// --- full config loading ---

// LoadFullConfig reads all configuration from the database and returns a Config struct
// compatible with what downstream code (LLM proxy, process manager, etc.) expects.
func (s *DBStore) LoadFullConfig(dbCfg DatabaseConfig) (*Config, error) {
	cfg := &Config{
		Database: dbCfg, // DB config stays from env
	}

	// Kernel config
	cfg.Kernel = KernelConfig{
		Port:                s.GetConfigInt("kernel.port", 8080),
		ExternalURL:         s.GetConfig("kernel.external_url"),
		HealthCheckInterval: s.GetConfigInt("kernel.health_check_interval", 10),
		MaxRestartAttempts:  s.GetConfigInt("kernel.max_restart_attempts", 3),
		LogLevel:            s.GetConfig("kernel.log_level"),
		AuditLogPath:        s.GetConfig("kernel.audit_log_path"),
	}

	// Jodo config
	cfg.Jodo = JodoConfig{
		Host:           s.GetConfig("jodo.host"),
		SSHUser:        s.GetConfig("jodo.ssh_user"),
		SSHKeyPath:     "", // SSH key now comes from DB secrets, not a file path
		Port:           s.GetConfigInt("jodo.port", 9001),
		AppPort:        s.GetConfigInt("jodo.app_port", 9000),
		BrainPath:      s.GetConfig("jodo.brain_path"),
		HealthEndpoint: s.GetConfig("jodo.health_endpoint"),
	}
	if cfg.Jodo.BrainPath == "" {
		cfg.Jodo.BrainPath = "/opt/jodo/brain"
	}
	if cfg.Jodo.HealthEndpoint == "" {
		cfg.Jodo.HealthEndpoint = "/health"
	}
	cfg.Jodo.MaxSubagents = s.GetConfigInt("jodo.max_subagents", 3)
	cfg.Jodo.SubagentTimeout = s.GetConfigInt("jodo.subagent_timeout", 300)

	// Providers
	cfg.Providers = make(map[string]ProviderConfig)
	rows, err := s.db.Query(`SELECT name, enabled, api_key_encrypted, base_url, monthly_budget, emergency_reserve FROM providers WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("load providers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var enabled bool
		var apiKeyEnc []byte
		var baseURL sql.NullString
		var budget, reserve float64

		if err := rows.Scan(&name, &enabled, &apiKeyEnc, &baseURL, &budget, &reserve); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}

		var apiKey string
		if len(apiKeyEnc) > 0 {
			apiKey, err = s.encryptor.Decrypt(apiKeyEnc)
			if err != nil {
				log.Printf("[config] warning: failed to decrypt API key for %s: %v", name, err)
			}
		}

		pc := ProviderConfig{
			APIKey:           apiKey,
			BaseURL:          baseURL.String,
			MonthlyBudget:    budget,
			EmergencyReserve: reserve,
			Models:           make(map[string]ModelConfig),
		}

		// Load models for this provider
		modelRows, err := s.db.Query(
			`SELECT model_key, model_name, input_cost_per_1m, output_cost_per_1m, capabilities, quality
			 FROM provider_models WHERE provider_name = $1 AND enabled = true`, name,
		)
		if err != nil {
			return nil, fmt.Errorf("load models for %s: %w", name, err)
		}

		for modelRows.Next() {
			var mk, mn string
			var ic, oc float64
			var caps []string
			var q int
			if err := modelRows.Scan(&mk, &mn, &ic, &oc, pq.Array(&caps), &q); err != nil {
				modelRows.Close()
				return nil, fmt.Errorf("scan model: %w", err)
			}
			pc.Models[mk] = ModelConfig{
				Model:                 mn,
				InputCostPer1MTokens:  ic,
				OutputCostPer1MTokens: oc,
				Capabilities:          caps,
				Quality:               q,
			}
		}
		modelRows.Close()

		cfg.Providers[name] = pc
	}

	// Routing config
	cfg.Routing = RoutingConfig{
		Strategy: s.GetConfig("routing.strategy"),
	}
	if cfg.Routing.Strategy == "" {
		cfg.Routing.Strategy = "best_affordable"
	}

	intentPrefJSON := s.GetConfig("routing.intent_preferences")
	if intentPrefJSON != "" {
		json.Unmarshal([]byte(intentPrefJSON), &cfg.Routing.IntentPreferences)
	}

	return cfg, nil
}

// --- import from YAML (auto-migration) ---

// ImportConfig imports a YAML-loaded Config into the database.
func (s *DBStore) ImportConfig(cfg *Config) error {
	// Kernel settings
	settings := map[string]string{
		"kernel.port":                  fmt.Sprintf("%d", cfg.Kernel.Port),
		"kernel.external_url":          cfg.Kernel.ExternalURL,
		"kernel.health_check_interval": fmt.Sprintf("%d", cfg.Kernel.HealthCheckInterval),
		"kernel.max_restart_attempts":  fmt.Sprintf("%d", cfg.Kernel.MaxRestartAttempts),
		"kernel.log_level":             cfg.Kernel.LogLevel,
		"kernel.audit_log_path":        cfg.Kernel.AuditLogPath,
		"jodo.host":                    cfg.Jodo.Host,
		"jodo.ssh_user":                cfg.Jodo.SSHUser,
		"jodo.port":                    fmt.Sprintf("%d", cfg.Jodo.Port),
		"jodo.app_port":                fmt.Sprintf("%d", cfg.Jodo.AppPort),
		"jodo.brain_path":              cfg.Jodo.BrainPath,
		"jodo.health_endpoint":         cfg.Jodo.HealthEndpoint,
		"routing.strategy":             cfg.Routing.Strategy,
	}

	for k, v := range settings {
		if v != "" && v != "0" {
			if err := s.SetConfig(k, v); err != nil {
				return fmt.Errorf("import config %s: %w", k, err)
			}
		}
	}

	// Intent preferences as JSON
	if len(cfg.Routing.IntentPreferences) > 0 {
		ipJSON, _ := json.Marshal(cfg.Routing.IntentPreferences)
		s.SetConfig("routing.intent_preferences", string(ipJSON))
	}

	// Providers
	for name, pc := range cfg.Providers {
		if err := s.SaveProvider(name, true, pc.APIKey, pc.BaseURL, pc.MonthlyBudget, pc.EmergencyReserve); err != nil {
			return fmt.Errorf("import provider %s: %w", name, err)
		}
		for mk, mc := range pc.Models {
			modelName := mc.Model
			if modelName == "" {
				modelName = mk
			}
			if err := s.SaveModel(name, mk, modelName, mc.InputCostPer1MTokens, mc.OutputCostPer1MTokens, mc.Capabilities, mc.Quality); err != nil {
				return fmt.Errorf("import model %s/%s: %w", name, mk, err)
			}
		}
	}

	// Import SSH key from file if it exists
	if cfg.Jodo.SSHKeyPath != "" {
		// We try, but don't fail migration if SSH key can't be read
		// (it may not be accessible from the new setup)
		log.Printf("[config] note: SSH key at %s not imported (will need to be regenerated in UI)", cfg.Jodo.SSHKeyPath)
	}

	return nil
}

// ImportGenesis imports a YAML-loaded Genesis into the database.
func (s *DBStore) ImportGenesis(g *Genesis) error {
	return s.SaveGenesis(g)
}

// --- provider listing for API ---

// ProviderInfo is the API-safe representation of a provider (key masked).
type ProviderInfo struct {
	Name             string        `json:"name"`
	Enabled          bool          `json:"enabled"`
	HasAPIKey        bool          `json:"has_api_key"`
	BaseURL          string        `json:"base_url"`
	MonthlyBudget    float64       `json:"monthly_budget"`
	EmergencyReserve float64       `json:"emergency_reserve"`
	Models           []ModelInfo   `json:"models"`
}

// ModelInfo is the API representation of a model.
type ModelInfo struct {
	ModelKey          string   `json:"model_key"`
	ModelName         string   `json:"model_name"`
	InputCostPer1M    float64  `json:"input_cost_per_1m"`
	OutputCostPer1M   float64  `json:"output_cost_per_1m"`
	Capabilities      []string `json:"capabilities"`
	Quality           int      `json:"quality"`
	Enabled           bool     `json:"enabled"`
}

// ListProviders returns all providers with their models (API keys masked).
func (s *DBStore) ListProviders() ([]ProviderInfo, error) {
	rows, err := s.db.Query(`SELECT name, enabled, api_key_encrypted IS NOT NULL, base_url, monthly_budget, emergency_reserve FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []ProviderInfo
	for rows.Next() {
		var p ProviderInfo
		var baseURL sql.NullString
		if err := rows.Scan(&p.Name, &p.Enabled, &p.HasAPIKey, &baseURL, &p.MonthlyBudget, &p.EmergencyReserve); err != nil {
			return nil, err
		}
		p.BaseURL = baseURL.String

		// Load models
		modelRows, err := s.db.Query(
			`SELECT model_key, model_name, input_cost_per_1m, output_cost_per_1m, capabilities, quality, enabled
			 FROM provider_models WHERE provider_name = $1 ORDER BY model_key`, p.Name,
		)
		if err != nil {
			return nil, err
		}
		for modelRows.Next() {
			var m ModelInfo
			if err := modelRows.Scan(&m.ModelKey, &m.ModelName, &m.InputCostPer1M, &m.OutputCostPer1M, pq.Array(&m.Capabilities), &m.Quality, &m.Enabled); err != nil {
				modelRows.Close()
				return nil, err
			}
			p.Models = append(p.Models, m)
		}
		modelRows.Close()

		providers = append(providers, p)
	}
	return providers, nil
}

// --- routing config ---

// GetRoutingConfig returns the current routing config.
func (s *DBStore) GetRoutingConfig() RoutingConfig {
	rc := RoutingConfig{
		Strategy: s.GetConfig("routing.strategy"),
	}
	if rc.Strategy == "" {
		rc.Strategy = "best_affordable"
	}
	ipJSON := s.GetConfig("routing.intent_preferences")
	if ipJSON != "" {
		json.Unmarshal([]byte(ipJSON), &rc.IntentPreferences)
	}
	return rc
}

// SaveRoutingConfig saves the routing config.
func (s *DBStore) SaveRoutingConfig(rc RoutingConfig) error {
	if err := s.SetConfig("routing.strategy", rc.Strategy); err != nil {
		return err
	}
	if len(rc.IntentPreferences) > 0 {
		ipJSON, _ := json.Marshal(rc.IntentPreferences)
		return s.SetConfig("routing.intent_preferences", string(ipJSON))
	}
	return nil
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// MaskKey returns a masked version of an API key for display (e.g., "sk-...abc123").
func MaskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "..." + key[len(key)-4:]
}
