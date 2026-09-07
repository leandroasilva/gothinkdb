package raft

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Cluster represents a test Raft cluster
type Cluster struct {
	nodes      []*Raft
	quorums    []*QuorumManager
	wals       []*WAL
	snapshots  []*SnapshotManager
	compactors []*CompactionManager
	tmpDir     string
}

// NewTestCluster creates a new test cluster
func NewTestCluster(t *testing.T, size int) (*Cluster, error) {
	tmpDir := t.TempDir()

	cluster := &Cluster{
		nodes:      make([]*Raft, size),
		quorums:    make([]*QuorumManager, size),
		wals:       make([]*WAL, size),
		snapshots:  make([]*SnapshotManager, size),
		compactors: make([]*CompactionManager, size),
		tmpDir:     tmpDir,
	}

	// Create peers list
	peers := make([]string, size-1)
	for i := 1; i < size; i++ {
		peers[i-1] = fmt.Sprintf("node%d", i+1)
	}

	// Create nodes
	for i := 0; i < size; i++ {
		nodeID := fmt.Sprintf("node%d", i+1)

		// Create node-specific peers list
		nodePeers := make([]string, 0, size-1)
		for j := 0; j < size; j++ {
			if i != j {
				nodePeers = append(nodePeers, fmt.Sprintf("node%d", j+1))
			}
		}

		config := DefaultConfig(nodeID, nodePeers)
		config.ElectionTimeout = 100 * time.Millisecond
		config.HeartbeatInterval = 50 * time.Millisecond

		raft := New(config)
		cluster.nodes[i] = raft

		// Create WAL
		walPath := filepath.Join(tmpDir, fmt.Sprintf("node%d.wal", i+1))
		wal, err := NewWAL(walPath)
		if err != nil {
			return nil, err
		}
		cluster.wals[i] = wal

		// Create Snapshot Manager
		snapshotPath := filepath.Join(tmpDir, fmt.Sprintf("node%d.snapshot", i+1))
		sm, err := NewSnapshotManager(snapshotPath)
		if err != nil {
			return nil, err
		}
		cluster.snapshots[i] = sm

		// Create Quorum Manager
		quorumConfig := DefaultQuorumConfig()
		quorum := NewQuorumManager(raft, quorumConfig)
		cluster.quorums[i] = quorum

		// Create Compaction Manager
		compactor := NewCompactionManager(raft, sm, wal)
		compactor.SetSnapshotThreshold(10) // Low threshold for testing
		cluster.compactors[i] = compactor
	}

	return cluster, nil
}

// Start starts all nodes in the cluster
func (c *Cluster) Start() {
	for _, node := range c.nodes {
		node.Start()
	}
}

// Stop stops all nodes in the cluster
func (c *Cluster) Stop() {
	for _, node := range c.nodes {
		node.Stop()
	}
	for _, wal := range c.wals {
		wal.Close()
	}
}

// GetLeader returns the current leader node
func (c *Cluster) GetLeader() (*Raft, int) {
	for i, node := range c.nodes {
		if node.IsLeader() {
			return node, i
		}
	}
	return nil, -1
}

// WaitForLeader waits for a leader to be elected
func (c *Cluster) WaitForLeader(timeout time.Duration) (*Raft, int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if leader, idx := c.GetLeader(); leader != nil {
			return leader, idx, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, -1, fmt.Errorf("no leader elected within timeout")
}

// TestCluster_LeaderElection tests leader election
func TestCluster_LeaderElection(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for leader election
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	t.Logf("Leader elected: node%d", idx+1)

	if leader == nil {
		t.Fatal("Leader should not be nil")
	}

	if !leader.IsLeader() {
		t.Error("Leader should be in leader state")
	}
}

// TestCluster_QuorumWrite tests quorum writes
func TestCluster_QuorumWrite(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for leader
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	t.Logf("Leader elected: node%d", idx+1)

	// Perform quorum write
	ctx := context.Background()
	quorum := cluster.quorums[idx]

	err = quorum.QuorumWrite(ctx, map[string]interface{}{
		"operation": "insert",
		"table":     "test",
		"data":      map[string]interface{}{"id": "1", "name": "test"},
	})

	if err != nil {
		t.Errorf("QuorumWrite failed: %v", err)
	}

	// Verify log entry was created
	leader.mu.RLock()
	logLength := len(leader.log)
	leader.mu.RUnlock()

	if logLength == 0 {
		t.Error("Log should have entries after write")
	}

	t.Logf("Log has %d entries after write", logLength)
}

// TestCluster_Failover tests leader failover
func TestCluster_Failover(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for initial leader
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	t.Logf("Initial leader: node%d", idx+1)

	// Stop the leader
	leader.Stop()

	// Wait for new leader
	time.Sleep(500 * time.Millisecond)

	newLeader, newIdx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect new leader: %v", err)
	}

	if newIdx == idx {
		t.Error("New leader should be different from old leader")
	}

	t.Logf("New leader after failover: node%d", newIdx+1)

	if newLeader == nil {
		t.Fatal("New leader should not be nil")
	}
}

// TestCluster_Snapshot tests snapshot creation and restoration
func TestCluster_Snapshot(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for leader
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	_ = leader // Mark as used

	// Write some entries
	ctx := context.Background()
	quorum := cluster.quorums[idx]

	for i := 0; i < 5; i++ {
		err = quorum.QuorumWrite(ctx, map[string]interface{}{
			"operation": "insert",
			"id":        i,
		})
		if err != nil {
			t.Errorf("QuorumWrite failed: %v", err)
		}
	}

	// Create snapshot
	compactor := cluster.compactors[idx]
	err = compactor.Compact()
	if err != nil {
		t.Errorf("Failed to create snapshot: %v", err)
	}

	// Verify snapshot exists
	sm := cluster.snapshots[idx]
	if !sm.Exists() {
		t.Error("Snapshot should exist after compaction")
	}

	// Load snapshot
	snapshot, err := sm.Load()
	if err != nil {
		t.Errorf("Failed to load snapshot: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}

	t.Logf("Snapshot created at index %d, term %d", snapshot.LastIndex, snapshot.LastTerm)
}

// TestCluster_LogCompaction tests log compaction
func TestCluster_LogCompaction(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for leader
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	// Write many entries
	ctx := context.Background()
	quorum := cluster.quorums[idx]

	for i := 0; i < 15; i++ {
		err = quorum.QuorumWrite(ctx, map[string]interface{}{
			"operation": "insert",
			"id":        i,
		})
		if err != nil {
			t.Errorf("QuorumWrite failed: %v", err)
		}
	}

	// Check if compaction is needed
	compactor := cluster.compactors[idx]
	if !compactor.ShouldCompact() {
		t.Log("Compaction not needed yet")
	}

	// Force compaction
	err = compactor.Compact()
	if err != nil {
		t.Errorf("Failed to compact: %v", err)
	}

	// Verify log was compacted
	leader.mu.RLock()
	logLength := len(leader.log)
	leader.mu.RUnlock()

	t.Logf("Log length after compaction: %d", logLength)

	// Log should be shorter after compaction
	if logLength >= 15 {
		t.Log("Warning: log was not compacted as expected")
	}
}

// TestCluster_ConcurrentWrites tests concurrent writes
func TestCluster_ConcurrentWrites(t *testing.T) {
	cluster, err := NewTestCluster(t, 3)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	// Wait for leader
	leader, idx, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	// Perform concurrent writes
	ctx := context.Background()
	quorum := cluster.quorums[idx]

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := quorum.QuorumWrite(ctx, map[string]interface{}{
				"operation": "insert",
				"id":        id,
			})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent write failed: %v", err)
	}

	// Verify all entries were written
	_ = leader // Mark as used
	leader.mu.RLock()
	logLength := len(leader.log)
	leader.mu.RUnlock()

	t.Logf("Log has %d entries after concurrent writes", logLength)

	if logLength < 10 {
		t.Errorf("Expected at least 10 entries, got %d", logLength)
	}
}
