package raft

import (
	"context"
	"testing"
)

func TestConsistencyLevel_String(t *testing.T) {
	tests := []struct {
		level ConsistencyLevel
		want  string
	}{
		{One, "one"},
		{Quorum, "quorum"},
		{All, "all"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("ConsistencyLevel.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestQuorumManager_GetQuorumSize(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	quorumConfig := DefaultQuorumConfig()
	qm := NewQuorumManager(raft, quorumConfig)

	// 3 nodes total (node1 + node2 + node3)
	// Quorum should be 2
	if got := qm.GetQuorumSize(); got != 2 {
		t.Errorf("GetQuorumSize() = %v, want 2", got)
	}
}

func TestQuorumManager_CheckQuorum(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	quorumConfig := DefaultQuorumConfig()
	qm := NewQuorumManager(raft, quorumConfig)

	// Quorum size is 2
	if !qm.CheckQuorum(2) {
		t.Error("CheckQuorum(2) should return true")
	}

	if !qm.CheckQuorum(3) {
		t.Error("CheckQuorum(3) should return true")
	}

	if qm.CheckQuorum(1) {
		t.Error("CheckQuorum(1) should return false")
	}
}

func TestQuorumManager_QuorumWrite(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	quorumConfig := DefaultQuorumConfig()
	qm := NewQuorumManager(raft, quorumConfig)
	ctx := context.Background()

	// Try to write as follower (should fail)
	err := qm.QuorumWrite(ctx, "test")
	if err == nil {
		t.Error("QuorumWrite should fail as follower")
	}

	// Become leader
	raft.mu.Lock()
	raft.state = Leader
	raft.mu.Unlock()

	// Now write should succeed
	err = qm.QuorumWrite(ctx, "test")
	if err != nil {
		t.Errorf("QuorumWrite should succeed as leader: %v", err)
	}
}
