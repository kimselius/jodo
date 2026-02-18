package git

import "fmt"

// CommitResponse is the API response for a commit operation.
type CommitResponse struct {
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
}

// CommitWithMessage commits the current brain state and returns the hash.
func (m *Manager) CommitWithMessage(message string) (*CommitResponse, error) {
	hash, err := m.Commit(message)
	if err != nil {
		return nil, err
	}
	if hash == "" {
		return &CommitResponse{Hash: "", Timestamp: ""}, nil
	}

	// Get timestamp
	output, err := m.sshRunner(fmt.Sprintf(
		`cd %s && git log -1 --format=%%aI`,
		m.cfg.BrainPath,
	))
	if err != nil {
		return &CommitResponse{Hash: hash}, nil
	}

	return &CommitResponse{
		Hash:      hash,
		Timestamp: output,
	}, nil
}
