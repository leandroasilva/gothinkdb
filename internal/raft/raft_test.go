package raft

import (
	"context"
	"testing"
	"time"
)

func TestRaft_New(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)

	if raft.config.NodeID != "node1" {
		t.Errorf("expected node_id=node1, got %s", raft.config.NodeID)
	}

	if raft.state != Follower {
		t.Errorf("expected state=Follower, got %v", raft.state)
	}

	if raft.currentTerm != 0 {
		t.Errorf("expected term=0, got %d", raft.currentTerm)
	}
}

func TestRaft_State(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)

	if raft.GetState() != Follower {
		t.Errorf("expected state=Follower, got %v", raft.GetState())
	}

	if raft.IsLeader() {
		t.Error("should not be leader initially")
	}
}

func TestRaft_RequestVote(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	ctx := context.Background()

	// Request vote with higher term
	args := &RequestVoteArgs{
		Term:         1,
		CandidateID:  "node2",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	reply := raft.RequestVote(ctx, args)

	if !reply.VoteGranted {
		t.Error("vote should be granted")
	}

	if raft.votedFor != "node2" {
		t.Errorf("expected votedFor=node2, got %s", raft.votedFor)
	}
}

func TestRaft_AppendEntries(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	ctx := context.Background()

	// Append entries from leader
	args := &AppendEntriesArgs{
		Term:         1,
		LeaderID:     "node2",
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries: []*LogEntry{
			{Term: 1, Index: 1, Command: "test1"},
			{Term: 1, Index: 2, Command: "test2"},
		},
		LeaderCommit: 0,
	}

	reply := raft.AppendEntries(ctx, args)

	if !reply.Success {
		t.Error("append should succeed")
	}

	if len(raft.log) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(raft.log))
	}
}

func TestRaft_Propose(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	raft := New(config)
	ctx := context.Background()

	// Try to propose as follower (should fail)
	err := raft.Propose(ctx, "test")
	if err == nil {
		t.Error("propose should fail as follower")
	}

	// Become leader
	raft.mu.Lock()
	raft.state = Leader
	raft.mu.Unlock()

	// Now propose should succeed
	err = raft.Propose(ctx, "test")
	if err != nil {
		t.Errorf("propose should succeed as leader: %v", err)
	}

	if len(raft.log) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(raft.log))
	}
}

func TestRaft_StartStop(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	config.ElectionTimeout = 100 * time.Millisecond
	raft := New(config)

	raft.Start()
	time.Sleep(50 * time.Millisecond)

	raft.Stop()

	// Should stop cleanly
}

func TestRaft_ElectionTimeout(t *testing.T) {
	config := DefaultConfig("node1", []string{"node2", "node3"})
	config.ElectionTimeout = 50 * time.Millisecond
	raft := New(config)

	// Start the raft node
	raft.Start()
	defer raft.Stop()

	// Wait for election timeout
	time.Sleep(200 * time.Millisecond)

	raft.mu.Lock()
	state := raft.state
	raft.mu.Unlock()

	// Should become candidate after timeout
	if state != Candidate {
		t.Errorf("expected state=Candidate after timeout, got %v", state)
	}
}
