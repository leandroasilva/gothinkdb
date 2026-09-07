package raft

import (
	"fmt"
	"sync"
)

// CompactionManager manages log compaction
type CompactionManager struct {
	raft            *Raft
	snapshotManager *SnapshotManager
	wal             *WAL
	mu              sync.Mutex
	
	// Compaction settings
	snapshotThreshold int64  // Compact after this many entries
	lastSnapshotIndex int64  // Index of last snapshot
}

// NewCompactionManager creates a new compaction manager
func NewCompactionManager(raft *Raft, snapshotManager *SnapshotManager, wal *WAL) *CompactionManager {
	return &CompactionManager{
		raft:              raft,
		snapshotManager:   snapshotManager,
		wal:               wal,
		snapshotThreshold: 1000, // Default threshold
		lastSnapshotIndex: 0,
	}
}

// SetSnapshotThreshold sets the snapshot threshold
func (cm *CompactionManager) SetSnapshotThreshold(threshold int64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.snapshotThreshold = threshold
}

// ShouldCompact checks if compaction is needed
func (cm *CompactionManager) ShouldCompact() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.raft.mu.RLock()
	logLength := int64(len(cm.raft.log))
	cm.raft.mu.RUnlock()

	return logLength >= cm.snapshotThreshold
}

// Compact performs log compaction
func (cm *CompactionManager) Compact() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.raft.mu.RLock()
	lastIndex := cm.raft.getLastLogIndex()
	lastTerm := cm.raft.getLastLogTerm()
	log := cm.raft.log
	cm.raft.mu.RUnlock()

	// Create snapshot
	if err := cm.snapshotManager.Create(lastIndex, lastTerm, log); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Truncate WAL
	if err := cm.wal.Truncate(lastIndex); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	// Update last snapshot index
	cm.lastSnapshotIndex = lastIndex

	// Compact in-memory log
	cm.raft.mu.Lock()
	if int64(len(cm.raft.log)) > cm.snapshotThreshold {
		// Keep only recent entries
		keepFrom := int64(len(cm.raft.log)) - cm.snapshotThreshold/2
		cm.raft.log = cm.raft.log[keepFrom:]
	}
	cm.raft.mu.Unlock()

	return nil
}

// RestoreFromSnapshot restores state from a snapshot
func (cm *CompactionManager) RestoreFromSnapshot() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Load snapshot
	snapshot, err := cm.snapshotManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load snapshot: %w", err)
	}

	if snapshot == nil {
		return nil // No snapshot to restore from
	}

	// Restore Raft state
	cm.raft.mu.Lock()
	cm.raft.commitIndex = snapshot.LastIndex
	cm.raft.lastApplied = snapshot.LastIndex
	
	// Restore log from snapshot data
	if logData, ok := snapshot.Data.([]*LogEntry); ok {
		cm.raft.log = logData
	}
	cm.raft.mu.Unlock()

	cm.lastSnapshotIndex = snapshot.LastIndex

	return nil
}

// GetLastSnapshotIndex returns the last snapshot index
func (cm *CompactionManager) GetLastSnapshotIndex() int64 {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.lastSnapshotIndex
}
