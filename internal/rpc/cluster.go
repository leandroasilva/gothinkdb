package rpc

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ClusterManager manages cluster membership and state
type ClusterManager struct {
	nodeID     string
	clusterID  string
	members    map[string]*NodeInfo
	leader     string
	term       int64
	mu         sync.RWMutex
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(nodeID string) *ClusterManager {
	return &ClusterManager{
		nodeID:    nodeID,
		clusterID: uuid.New().String(),
		members:   make(map[string]*NodeInfo),
		term:      0,
	}
}

// AddNode adds a node to the cluster
func (cm *ClusterManager) AddNode(nodeID, address string, clusterPort int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.members[nodeID]; exists {
		return fmt.Errorf("node %s already exists in cluster", nodeID)
	}

	cm.members[nodeID] = &NodeInfo{
		NodeID:      nodeID,
		Address:     address,
		ClusterPort: clusterPort,
		Status:      "active",
		LastSeen:    time.Now(),
		JoinedAt:    time.Now(),
	}

	return nil
}

// RemoveNode removes a node from the cluster
func (cm *ClusterManager) RemoveNode(nodeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.members[nodeID]; !exists {
		return fmt.Errorf("node %s does not exist in cluster", nodeID)
	}

	delete(cm.members, nodeID)
	return nil
}

// UpdateNodeStatus updates the status of a node
func (cm *ClusterManager) UpdateNodeStatus(nodeID, status string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if node, exists := cm.members[nodeID]; exists {
		node.Status = status
		node.LastSeen = time.Now()
	}
}

// GetNode gets information about a specific node
func (cm *ClusterManager) GetNode(nodeID string) (*NodeInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.members[nodeID]
	return node, exists
}

// GetMembers returns all members of the cluster
func (cm *ClusterManager) GetMembers() []*NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	members := make([]*NodeInfo, 0, len(cm.members))
	for _, member := range cm.members {
		members = append(members, member)
	}
	return members
}

// GetActiveMembers returns all active members of the cluster
func (cm *ClusterManager) GetActiveMembers() []*NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	members := make([]*NodeInfo, 0)
	for _, member := range cm.members {
		if member.Status == "active" {
			members = append(members, member)
		}
	}
	return members
}

// GetClusterID returns the cluster ID
func (cm *ClusterManager) GetClusterID() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.clusterID
}

// SetLeader sets the cluster leader
func (cm *ClusterManager) SetLeader(nodeID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.leader = nodeID
	cm.term++
}

// GetLeader returns the cluster leader
func (cm *ClusterManager) GetLeader() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.leader
}

// GetTerm returns the current term
func (cm *ClusterManager) GetTerm() int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.term
}

// GetClusterState returns the current cluster state
func (cm *ClusterManager) GetClusterState() *ClusterState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	membersCopy := make(map[string]*NodeInfo)
	for k, v := range cm.members {
		membersCopy[k] = v
	}

	return &ClusterState{
		ClusterID: cm.clusterID,
		Members:   membersCopy,
		Leader:    cm.leader,
		Term:      cm.term,
		UpdatedAt: time.Now(),
	}
}

// MarkInactiveNodes marks nodes as inactive if they haven't been seen recently
func (cm *ClusterManager) MarkInactiveNodes(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	for _, node := range cm.members {
		if node.LastSeen.Before(cutoff) && node.Status == "active" {
			node.Status = "inactive"
		}
	}
}
