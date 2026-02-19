package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	pgvector "github.com/pgvector/pgvector-go"
	"jodo-kernel/internal/llm"
)

// Store manages Jodo's semantic memories in pgvector.
type Store struct {
	db    *sql.DB
	proxy *llm.Proxy
}

func NewStore(db *sql.DB, proxy *llm.Proxy) *Store {
	return &Store{db: db, proxy: proxy}
}

// StoreRequest is the input for storing a new memory.
type StoreRequest struct {
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Source  string   `json:"source,omitempty"`
}

// StoreResponse is returned after storing a memory.
type StoreResponse struct {
	ID                  string  `json:"id"`
	EmbeddingDimensions int     `json:"embedding_dimensions"`
	Cost                float64 `json:"cost"`
	Stored              bool    `json:"stored"`
}

// Store persists a memory with its embedding vector.
func (s *Store) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error) {
	// Generate embedding
	embedResp, err := s.proxy.Embed(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	id := uuid.New().String()
	vec := pgvector.NewVector(embedResp.Embedding)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO memories (id, content, embedding, tags, source) VALUES ($1, $2, $3, $4, $5)`,
		id, req.Content, vec, pq.Array(req.Tags), req.Source,
	)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}

	log.Printf("[memory] stored %s (%d dims, cost $%.6f)", id[:8], len(embedResp.Embedding), embedResp.Cost)

	return &StoreResponse{
		ID:                  id,
		EmbeddingDimensions: len(embedResp.Embedding),
		Cost:                embedResp.Cost,
		Stored:              true,
	}, nil
}

// Count returns the total number of stored memories.
func (s *Store) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count)
	return count, err
}

// MemoryEntry is a single stored memory (no embedding).
type MemoryEntry struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"created_at"`
}

// List returns recent memories, newest first.
func (s *Store) List(limit, offset int) ([]MemoryEntry, error) {
	if limit == 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT id, content, COALESCE(tags, '{}'), COALESCE(source, ''), created_at
		 FROM memories ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.Content, pq.Array(&m.Tags), &m.Source, &createdAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.CreatedAt = createdAt.Format(time.RFC3339)
		entries = append(entries, m)
	}

	if entries == nil {
		entries = []MemoryEntry{}
	}
	return entries, nil
}
