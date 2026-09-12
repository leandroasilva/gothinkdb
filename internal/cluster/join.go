package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// JoinManager handles node join operations
type JoinManager struct {
	tokenManager *TokenManager
	members      map[string]*NodeInfo
	mu           sync.RWMutex
}

// NewJoinManager creates a new join manager
func NewJoinManager(tokenManager *TokenManager) *JoinManager {
	return &JoinManager{
		tokenManager: tokenManager,
		members:      make(map[string]*NodeInfo),
	}
}

// HandleJoinRequest handles a request from a node to join the cluster
func (jm *JoinManager) HandleJoinRequest(req *JoinRequest) (*JoinResponse, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// Validate token
	token, err := jm.tokenManager.ValidateToken(req.Token)
	if err != nil {
		return &JoinResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid token: %v", err),
		}, nil
	}

	// Check if node already exists
	if _, exists := jm.members[req.NodeID]; exists {
		return &JoinResponse{
			Success: false,
			Error:   fmt.Sprintf("node %s already exists in cluster", req.NodeID),
		}, nil
	}

	// Add node to cluster
	nodeInfo := &NodeInfo{
		NodeID:      req.NodeID,
		Address:     req.Address,
		ClusterPort: req.ClusterPort,
		HTTPPort:    req.HTTPPort,
		DriverPort:  req.DriverPort,
		Status:      NodeStatusActive,
		JoinedAt:    time.Now(),
		LastSeen:    time.Now(),
	}

	jm.members[req.NodeID] = nodeInfo

	// Mark token as used
	if err := jm.tokenManager.MarkTokenUsed(req.Token, req.NodeID); err != nil {
		slog.Error("failed to mark token as used", "error", err)
	}

	slog.Info("node joined cluster",
		"node_id", req.NodeID,
		"address", req.Address,
		"cluster_id", token.ClusterID,
	)

	// Build response with current members
	members := make([]*NodeInfo, 0, len(jm.members))
	for _, m := range jm.members {
		members = append(members, m)
	}

	return &JoinResponse{
		Success:   true,
		NodeID:    req.NodeID,
		ClusterID: token.ClusterID,
		Members:   members,
	}, nil
}

// AddMember adds a member to the cluster (called when node registers via API)
func (jm *JoinManager) AddMember(nodeID, address string, clusterPort, httpPort, driverPort int) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if _, exists := jm.members[nodeID]; exists {
		return fmt.Errorf("node %s already exists", nodeID)
	}

	jm.members[nodeID] = &NodeInfo{
		NodeID:      nodeID,
		Address:     address,
		ClusterPort: clusterPort,
		HTTPPort:    httpPort,
		DriverPort:  driverPort,
		Status:      NodeStatusJoining,
		JoinedAt:    time.Now(),
		LastSeen:    time.Now(),
	}

	return nil
}

// RemoveMember removes a member from the cluster
func (jm *JoinManager) RemoveMember(nodeID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if _, exists := jm.members[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(jm.members, nodeID)
	return nil
}

// GetMembers returns all cluster members
func (jm *JoinManager) GetMembers() []*NodeInfo {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	members := make([]*NodeInfo, 0, len(jm.members))
	for _, m := range jm.members {
		members = append(members, m)
	}
	return members
}

// GetMember returns a specific member
func (jm *JoinManager) GetMember(nodeID string) (*NodeInfo, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	member, exists := jm.members[nodeID]
	return member, exists
}

// UpdateMemberStatus updates the status of a member
func (jm *JoinManager) UpdateMemberStatus(nodeID string, status NodeStatus) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	member, exists := jm.members[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	member.Status = status
	member.LastSeen = time.Now()
	return nil
}

// SendJoinRequest sends a join request to an existing cluster node
func SendJoinRequest(leaderAddress string, req *JoinRequest) (*JoinResponse, error) {
	url := fmt.Sprintf("http://%s/api/cluster/join", leaderAddress)
	
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to send join request: %w", err)
	}
	defer resp.Body.Close()

	var joinResp JoinResponse
	if err := json.NewDecoder(resp.Body).Decode(&joinResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &joinResp, nil
}
