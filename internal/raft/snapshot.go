package raft

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Snapshot represents a Raft snapshot
type Snapshot struct {
	LastIndex  int64       `json:"last_index"`
	LastTerm   int64       `json:"last_term"`
	Data       interface{} `json:"data"`
	Timestamp  int64       `json:"timestamp"`
}

// SnapshotManager manages Raft snapshots
type SnapshotManager struct {
	path     string
	mu       sync.Mutex
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(path string) (*SnapshotManager, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	return &SnapshotManager{
		path: path,
	}, nil
}

// Create creates a new snapshot
func (sm *SnapshotManager) Create(lastIndex, lastTerm int64, data interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot := &Snapshot{
		LastIndex: lastIndex,
		LastTerm:  lastTerm,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Marshal snapshot
	jsonData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sm.path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return nil
}

// Load loads the latest snapshot
func (sm *SnapshotManager) Load() (*Snapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Read from file
	jsonData, err := os.ReadFile(sm.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No snapshot exists
		}
		return nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	// Unmarshal snapshot
	var snapshot Snapshot
	if err := json.Unmarshal(jsonData, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

// Delete deletes the snapshot
func (sm *SnapshotManager) Delete() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := os.Remove(sm.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	return nil
}

// Exists checks if a snapshot exists
func (sm *SnapshotManager) Exists() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, err := os.Stat(sm.path)
	return err == nil
}
