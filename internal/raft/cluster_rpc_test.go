package raft

import (
	"fmt"
	"testing"
	"time"
)

// TestClusterWithRPC tests a real Raft cluster with RPC communication
func TestClusterWithRPC(t *testing.T) {
	// Create 3-node cluster
	cluster := &TestClusterRPC{
		nodes:      make([]*Raft, 3),
		transports: make([]*Transport, 3),
	}
	defer cluster.Stop()

	// Create nodes with real addresses
	addresses := []string{
		"localhost:9001",
		"localhost:9002",
		"localhost:9003",
	}

	for i := 0; i < 3; i++ {
		nodeID := fmt.Sprintf("node%d", i+1)
		peers := make([]string, 0, 2)
		for j := 0; j < 3; j++ {
			if i != j {
				peers = append(peers, fmt.Sprintf("node%d", j+1))
			}
		}

		config := DefaultConfig(nodeID, peers)
		config.ElectionTimeout = 150 * time.Millisecond
		config.HeartbeatInterval = 50 * time.Millisecond

		raft := New(config)
		cluster.nodes[i] = raft

		// Create transport
		transport := NewTransport(nodeID, addresses[i], raft)
		cluster.transports[i] = transport

		// Set transport
		raft.SetTransport(transport)

		// Add peers to transport
		for j := 0; j < 3; j++ {
			if i != j {
				peerID := fmt.Sprintf("node%d", j+1)
				transport.AddPeer(peerID, addresses[j])
			}
		}
	}

	// Start all transports
	for i := 0; i < 3; i++ {
		if err := cluster.transports[i].Start(); err != nil {
			t.Fatalf("failed to start transport %d: %v", i, err)
		}
	}

	// Start all nodes with staggered start to avoid split vote
	for i := 0; i < 3; i++ {
		cluster.nodes[i].Start()
		time.Sleep(100 * time.Millisecond) // Stagger starts
	}

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Check if we have a leader
	leaderCount := 0
	for i := 0; i < 3; i++ {
		if cluster.nodes[i].IsLeader() {
			leaderCount++
			t.Logf("Leader elected: node%d", i+1)
		}
	}

	if leaderCount == 0 {
		t.Error("no leader elected")
	} else if leaderCount > 1 {
		t.Errorf("multiple leaders elected: %d", leaderCount)
	} else {
		t.Log("✓ Leader election successful")
	}

	// Test log replication
	leaderIdx := -1
	for i := 0; i < 3; i++ {
		if cluster.nodes[i].IsLeader() {
			leaderIdx = i
			break
		}
	}

	if leaderIdx >= 0 {
		// Propose a command
		err := cluster.nodes[leaderIdx].Propose(nil, "test-command")
		if err != nil {
			t.Errorf("failed to propose command: %v", err)
		} else {
			t.Log("✓ Command proposed successfully")
		}

		// Wait for replication
		time.Sleep(500 * time.Millisecond)

		// Check if log was replicated
		leaderLogLen := len(cluster.nodes[leaderIdx].log)
		t.Logf("Leader log length: %d", leaderLogLen)

		for i := 0; i < 3; i++ {
			if i != leaderIdx {
				followerLogLen := len(cluster.nodes[i].log)
				t.Logf("Follower node%d log length: %d", i+1, followerLogLen)
			}
		}
	}
}

// TestClusterRPC represents a test Raft cluster
type TestClusterRPC struct {
	nodes      []*Raft
	transports []*Transport
}

// Stop stops all nodes and transports
func (c *TestClusterRPC) Stop() {
	for i := 0; i < len(c.nodes); i++ {
		if c.nodes[i] != nil {
			c.nodes[i].Stop()
		}
		if c.transports[i] != nil {
			c.transports[i].Stop()
		}
	}
}
