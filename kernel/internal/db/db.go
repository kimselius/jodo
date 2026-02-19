package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"jodo-kernel/internal/config"
)

// Connect opens a connection to postgres, retrying for up to 30 seconds
// while Postgres starts up (common in Docker).
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Retry — Postgres may still be starting in Docker
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		if err := db.Ping(); err == nil {
			return db, nil
		} else {
			lastErr = err
			log.Printf("[db] waiting for postgres (attempt %d/15): %v", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}

	db.Close()
	return nil, fmt.Errorf("ping database after 30s: %w", lastErr)
}

// RunMigrations creates the required tables if they don't exist.
func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,

		`CREATE TABLE IF NOT EXISTS memories (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			embedding vector(1024),
			tags TEXT[] DEFAULT '{}',
			source VARCHAR(100),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS budget_usage (
			id SERIAL PRIMARY KEY,
			provider VARCHAR(50) NOT NULL,
			model VARCHAR(100) NOT NULL,
			intent VARCHAR(50),
			tokens_in INTEGER,
			tokens_out INTEGER,
			cost DECIMAL(10, 6),
			requested_by VARCHAR(50) DEFAULT 'jodo',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS growth_log (
			id SERIAL PRIMARY KEY,
			event VARCHAR(100) NOT NULL,
			note TEXT,
			git_hash VARCHAR(40),
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS health_checks (
			id SERIAL PRIMARY KEY,
			status VARCHAR(20) NOT NULL,
			response_time_ms INTEGER,
			details JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS chat_messages (
			id SERIAL PRIMARY KEY,
			source VARCHAR(20) NOT NULL,
			message TEXT NOT NULL,
			galla INTEGER,
			read_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// Migration: add read_at if table already exists without it
		`DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'chat_messages' AND column_name = 'read_at'
			) THEN
				ALTER TABLE chat_messages ADD COLUMN read_at TIMESTAMPTZ;
			END IF;
		END $$`,

		// Configuration tables (config-in-DB)
		`CREATE TABLE IF NOT EXISTS system_config (
			key        VARCHAR(100) PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS secrets (
			key              VARCHAR(100) PRIMARY KEY,
			value_encrypted  BYTEA NOT NULL,
			created_at       TIMESTAMPTZ DEFAULT NOW(),
			updated_at       TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS providers (
			name              VARCHAR(50) PRIMARY KEY,
			enabled           BOOLEAN NOT NULL DEFAULT true,
			api_key_encrypted BYTEA,
			base_url          TEXT,
			monthly_budget    DECIMAL(10,2) DEFAULT 0,
			emergency_reserve DECIMAL(10,2) DEFAULT 0,
			created_at        TIMESTAMPTZ DEFAULT NOW(),
			updated_at        TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS provider_models (
			id                 SERIAL PRIMARY KEY,
			provider_name      VARCHAR(50) NOT NULL REFERENCES providers(name) ON DELETE CASCADE,
			model_key          VARCHAR(100) NOT NULL,
			model_name         VARCHAR(100) NOT NULL,
			input_cost_per_1m  DECIMAL(10,4) DEFAULT 0,
			output_cost_per_1m DECIMAL(10,4) DEFAULT 0,
			capabilities       TEXT[] DEFAULT '{}',
			quality            INTEGER DEFAULT 5,
			enabled            BOOLEAN DEFAULT true,
			UNIQUE(provider_name, model_key)
		)`,

		`CREATE TABLE IF NOT EXISTS genesis (
			id                 INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			name               VARCHAR(100) NOT NULL DEFAULT 'Jodo',
			version            INTEGER NOT NULL DEFAULT 0,
			purpose            TEXT NOT NULL DEFAULT '',
			survival_instincts TEXT[] DEFAULT '{}',
			capabilities_api   JSONB DEFAULT '{}',
			capabilities_local TEXT[] DEFAULT '{}',
			first_tasks        TEXT[] DEFAULT '{}',
			hints              TEXT[] DEFAULT '{}',
			updated_at         TIMESTAMPTZ DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	// Create indexes (use IF NOT EXISTS via DO block for idempotency)
	indexes := []string{
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'memories_embedding_idx') THEN
				CREATE INDEX memories_embedding_idx ON memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
			END IF;
		END $$`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'budget_usage_provider_created_idx') THEN
				CREATE INDEX budget_usage_provider_created_idx ON budget_usage (provider, created_at);
			END IF;
		END $$`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'budget_usage_created_idx') THEN
				CREATE INDEX budget_usage_created_idx ON budget_usage (created_at);
			END IF;
		END $$`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'chat_messages_created_idx') THEN
				CREATE INDEX chat_messages_created_idx ON chat_messages (created_at);
			END IF;
		END $$`,
	}

	for _, idx := range indexes {
		// ivfflat index requires rows to exist; ignore errors on empty tables
		db.Exec(idx)
	}

	return nil
}

// PruneOldHealthChecks removes health checks older than 24 hours.
func PruneOldHealthChecks(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM health_checks WHERE created_at < NOW() - INTERVAL '24 hours'`)
	return err
}
