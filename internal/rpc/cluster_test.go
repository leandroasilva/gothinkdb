package rpc

import (
	"testing"
	"time"
)

func TestClusterManager_AddNode(t *testing.T) {
	cm := NewClusterManager("node1")

	err := cm.AddNode("node2", "localhost", 29015)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	node, exists := cm.GetNode("node2")
	if !exists {
		t.Fatal("node should exist")
	}

	if node.NodeID != "node2" {
		t.Errorf("expected node_id=node2, got %s", node.NodeID)
	}

	if node.Status != "active" {
		t.Errorf("expected status=active, got %s", node.Status)
	}
}

func TestClusterManager_RemoveNode(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.AddNode("node2", "localhost", 29015)
	err := cm.RemoveNode("node2")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	_, exists := cm.GetNode("node2")
	if exists {
		t.Error("node should not exist after removal")
	}
}

func TestClusterManager_GetMembers(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.AddNode("node2", "localhost", 29015)
	cm.AddNode("node3", "localhost", 29016)

	members := cm.GetMembers()
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestClusterManager_GetActiveMembers(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.AddNode("node2", "localhost", 29015)
	cm.AddNode("node3", "localhost", 29016)

	cm.UpdateNodeStatus("node3", "inactive")

	activeMembers := cm.GetActiveMembers()
	if len(activeMembers) != 1 {
		t.Errorf("expected 1 active member, got %d", len(activeMembers))
	}
}

func TestClusterManager_SetLeader(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.SetLeader("node1")

	leader := cm.GetLeader()
	if leader != "node1" {
		t.Errorf("expected leader=node1, got %s", leader)
	}

	term := cm.GetTerm()
	if term != 1 {
		t.Errorf("expected term=1, got %d", term)
	}
}

func TestClusterManager_MarkInactiveNodes(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.AddNode("node2", "localhost", 29015)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Mark nodes inactive if not seen in 5ms
	cm.MarkInactiveNodes(5 * time.Millisecond)

	node, _ := cm.GetNode("node2")
	if node.Status != "inactive" {
		t.Errorf("expected status=inactive, got %s", node.Status)
	}
}

func TestClusterManager_GetClusterState(t *testing.T) {
	cm := NewClusterManager("node1")

	cm.AddNode("node2", "localhost", 29015)
	cm.SetLeader("node1")

	state := cm.GetClusterState()

	if state.ClusterID == "" {
		t.Error("cluster_id should not be empty")
	}

	// Note: node1 is the local node, node2 was added, so we have 1 member in the map
	if len(state.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(state.Members))
	}

	if state.Leader != "node1" {
		t.Errorf("expected leader=node1, got %s", state.Leader)
	}
}
