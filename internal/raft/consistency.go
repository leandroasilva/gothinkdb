package raft

import (
	"context"
	"fmt"
	"sync"
)

// ConsistencyLevel represents the consistency level for operations
type ConsistencyLevel int

const (
	// One requires only one node to acknowledge
	One ConsistencyLevel = iota
	// Quorum requires majority of nodes to acknowledge
	Quorum
	// All requires all nodes to acknowledge
	All
)

// String returns the string representation of consistency level
func (c ConsistencyLevel) String() string {
	switch c {
	case One:
		return "one"
	case Quorum:
		return "quorum"
	case All:
		return "all"
	default:
		return "unknown"
	}
}

// QuorumConfig represents quorum configuration
type QuorumConfig struct {
	ReadConsistency  ConsistencyLevel `json:"read_consistency"`
	WriteConsistency ConsistencyLevel `json:"write_consistency"`
}

// DefaultQuorumConfig returns default quorum configuration
func DefaultQuorumConfig() *QuorumConfig {
	return &QuorumConfig{
		ReadConsistency:  Quorum,
		WriteConsistency: Quorum,
	}
}

// QuorumManager manages quorum operations
type QuorumManager struct {
	raft       *Raft
	config     *QuorumConfig
	mu         sync.RWMutex
}

// NewQuorumManager creates a new quorum manager
func NewQuorumManager(raft *Raft, config *QuorumConfig) *QuorumManager {
	return &QuorumManager{
		raft:   raft,
		config: config,
	}
}

// QuorumWrite performs a quorum write operation
func (qm *QuorumManager) QuorumWrite(ctx context.Context, command interface{}) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if !qm.raft.IsLeader() {
		return fmt.Errorf("not leader")
	}

	// Propose the command
	if err := qm.raft.Propose(ctx, command); err != nil {
		return err
	}

	// Wait for quorum acknowledgment
	// In a real implementation, this would wait for majority of nodes
	// For now, we assume the leader's acknowledgment is sufficient
	return nil
}

// QuorumRead performs a quorum read operation
func (qm *QuorumManager) QuorumRead(ctx context.Context, key string) (interface{}, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	// For quorum reads, we need to read from the leader
	// In a real implementation, this would verify the leader is still valid
	// by contacting a quorum of nodes
	
	// For now, just return from local state
	return nil, nil
}

// GetQuorumSize returns the quorum size for the cluster
func (qm *QuorumManager) GetQuorumSize() int {
	totalNodes := len(qm.raft.config.Peers) + 1 // +1 for self
	return (totalNodes / 2) + 1
}

// CheckQuorum checks if we have quorum acknowledgment
func (qm *QuorumManager) CheckQuorum(ackCount int) bool {
	return ackCount >= qm.GetQuorumSize()
}
