package growth

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// Logger records milestones and events in the growth_log table.
type Logger struct {
	db        *sql.DB
	OnEvent   func(event, note, gitHash string) // optional callback for real-time broadcasting
}

func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log records a growth event.
func (l *Logger) Log(event, note, gitHash string, metadata map[string]interface{}) {
	metaJSON := "{}"
	if metadata != nil {
		if data, err := json.Marshal(metadata); err == nil {
			metaJSON = string(data)
		}
	}

	_, err := l.db.Exec(
		`INSERT INTO growth_log (event, note, git_hash, metadata) VALUES ($1, $2, $3, $4::jsonb)`,
		event, note, gitHash, metaJSON,
	)
	if err != nil {
		log.Printf("[growth] failed to log event %q: %v", event, err)
		return
	}

	if l.OnEvent != nil {
		l.OnEvent(event, note, gitHash)
	}
}

// Milestone is a growth log entry.
type Milestone struct {
	ID        int                    `json:"id"`
	Event     string                 `json:"event"`
	Note      string                 `json:"note"`
	GitHash   string                 `json:"git_hash,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// Recent returns the most recent growth milestones.
func (l *Logger) Recent(limit int) ([]Milestone, error) {
	if limit == 0 {
		limit = 20
	}

	rows, err := l.db.Query(
		`SELECT id, event, note, COALESCE(git_hash, ''), metadata, created_at
		 FROM growth_log ORDER BY created_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []Milestone
	for rows.Next() {
		var m Milestone
		var metaJSON string
		if err := rows.Scan(&m.ID, &m.Event, &m.Note, &m.GitHash, &metaJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(metaJSON), &m.Metadata)
		milestones = append(milestones, m)
	}

	if milestones == nil {
		milestones = []Milestone{}
	}
	return milestones, nil
}
