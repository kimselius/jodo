package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	pgvector "github.com/pgvector/pgvector-go"
	"jodo-kernel/internal/llm"
)

// SearchRequest is the input for semantic memory search.
type SearchRequest struct {
	Query string   `json:"query"`
	Limit int      `json:"limit,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// SearchResult is a single memory search result.
type SearchResult struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Similarity float64   `json:"similarity"`
	Tags       []string  `json:"tags"`
	CreatedAt  time.Time `json:"created_at"`
}

// SearchResponse contains the search results and cost.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Cost    float64        `json:"cost"`
}

// Searcher performs semantic similarity searches over memories.
type Searcher struct {
	db    *sql.DB
	proxy *llm.Proxy
}

func NewSearcher(db *sql.DB, proxy *llm.Proxy) *Searcher {
	return &Searcher{db: db, proxy: proxy}
}

// Search performs a semantic similarity search.
func (s *Searcher) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req.Limit == 0 {
		req.Limit = 5
	}

	// Generate embedding for the query
	embedResp, err := s.proxy.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vec := pgvector.NewVector(embedResp.Embedding)

	var rows *sql.Rows
	if len(req.Tags) > 0 {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, content, 1 - (embedding <=> $1) AS similarity, tags, created_at
			 FROM memories
			 WHERE tags && $2
			 ORDER BY embedding <=> $1
			 LIMIT $3`,
			vec, pq.Array(req.Tags), req.Limit,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, content, 1 - (embedding <=> $1) AS similarity, tags, created_at
			 FROM memories
			 ORDER BY embedding <=> $1
			 LIMIT $2`,
			vec, req.Limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Content, &r.Similarity, pq.Array(&r.Tags), &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}

	if results == nil {
		results = []SearchResult{}
	}

	return &SearchResponse{
		Results: results,
		Cost:    embedResp.Cost,
	}, nil
}
