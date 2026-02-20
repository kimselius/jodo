package api

import (
	"fmt"
	"log"

	"jodo-kernel/internal/config"
)

// Shared save functions used by both setup and settings handlers.

// saveConnectionConfig persists VPS host and SSH user to system_config.
func (s *Server) saveConnectionConfig(host, sshUser string) error {
	if host != "" {
		if err := s.ConfigStore.SetConfig("jodo.host", host); err != nil {
			return fmt.Errorf("save jodo.host: %w", err)
		}
	}
	if sshUser != "" {
		if err := s.ConfigStore.SetConfig("jodo.ssh_user", sshUser); err != nil {
			return fmt.Errorf("save jodo.ssh_user: %w", err)
		}
	}
	log.Printf("[config] connection saved: %s@%s", sshUser, host)
	return nil
}

// saveProvidersBulk saves all providers and their models (used during setup).
func (s *Server) saveProvidersBulk(providers []providerSetupReq) error {
	for _, p := range providers {
		if err := s.ConfigStore.SaveProvider(p.Name, p.Enabled, p.APIKey, p.BaseURL, p.MonthlyBudget, p.EmergencyReserve, p.TotalVRAMBytes); err != nil {
			return fmt.Errorf("save provider %s: %w", p.Name, err)
		}
		for _, m := range p.Models {
			if err := s.ConfigStore.SaveModel(p.Name, m.ModelKey, m.ModelName, m.InputCostPer1M, m.OutputCostPer1M, m.Capabilities, m.Quality, m.VRAMEstimateBytes, m.SupportsTools, m.PreferLoaded); err != nil {
				return fmt.Errorf("save model %s/%s: %w", p.Name, m.ModelKey, err)
			}
		}
	}
	log.Printf("[config] saved %d providers", len(providers))
	return nil
}

// saveGenesisConfig validates and persists genesis configuration.
// Returns the saved Genesis for in-memory updates.
func (s *Server) saveGenesisConfig(req genesisSetupReq) (*config.Genesis, error) {
	genesis := req.toGenesis()
	if err := s.ConfigStore.SaveGenesis(genesis); err != nil {
		return nil, fmt.Errorf("save genesis: %w", err)
	}
	log.Printf("[config] genesis saved: %s", genesis.Identity.Name)
	return genesis, nil
}

// saveRoutingPreferences persists intent→model routing preferences.
func (s *Server) saveRoutingPreferences(prefs map[string][]string) error {
	rc := config.RoutingConfig{IntentPreferences: prefs}
	if err := s.ConfigStore.SaveRoutingConfig(rc); err != nil {
		return fmt.Errorf("save routing: %w", err)
	}
	log.Printf("[config] routing preferences saved (%d intents)", len(prefs))
	return nil
}
