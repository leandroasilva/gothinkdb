package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// ReplicationManager manages data replication between nodes
type ReplicationManager struct {
	nodeID     string
	cluster    *ClusterManager
	server     *Server
	mu         sync.RWMutex
	replicas   map[string]*ReplicaInfo
}

// ReplicaInfo represents information about a replica
type ReplicaInfo struct {
	TableID    string    `json:"table_id"`
	NodeID     string    `json:"node_id"`
	Role       string    `json:"role"` // "primary", "secondary"
	LastSync   time.Time `json:"last_sync"`
	Status     string    `json:"status"` // "syncing", "ready", "error"
}

// ReplicateRequestPayload represents a replication request
type ReplicateRequestPayload struct {
	TableID    string      `json:"table_id"`
	Operation  string      `json:"operation"` // "insert", "update", "delete"
	Key        string      `json:"key"`
	Value      interface{} `json:"value,omitempty"`
	OldValue   interface{} `json:"old_value,omitempty"`
	Timestamp  int64       `json:"timestamp"`
}

// ReplicateResponsePayload represents a replication response
type ReplicateResponsePayload struct {
	TableID   string `json:"table_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// NewReplicationManager creates a new replication manager
func NewReplicationManager(nodeID string, cluster *ClusterManager, server *Server) *ReplicationManager {
	return &ReplicationManager{
		nodeID:   nodeID,
		cluster:  cluster,
		server:   server,
		replicas: make(map[string]*ReplicaInfo),
	}
}

// ReplicateToNode replicates an operation to a specific node
func (rm *ReplicationManager) ReplicateToNode(ctx context.Context, targetNodeID string, tableID, operation, key string, value, oldValue interface{}) error {
	node, exists := rm.cluster.GetNode(targetNodeID)
	if !exists {
		return fmt.Errorf("node %s not found in cluster", targetNodeID)
	}

	address := node.Address
	if node.ClusterPort > 0 {
		address = fmt.Sprintf("%s:%d", node.Address, node.ClusterPort)
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	msg := &Message{
		Type:      MsgReplicateRequest,
		From:      rm.nodeID,
		To:        targetNodeID,
		Timestamp: time.Now().Unix(),
		Payload: ReplicateRequestPayload{
			TableID:   tableID,
			Operation: operation,
			Key:       key,
			Value:     value,
			OldValue:  oldValue,
			Timestamp: time.Now().Unix(),
		},
	}

	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send replicate request: %w", err)
	}

	var response Message
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("failed to read replicate response: %w", err)
	}

	if response.Type != MsgReplicateResponse {
		return fmt.Errorf("unexpected response type: %d", response.Type)
	}

	payload, ok := response.Payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid replicate response payload")
	}

	success, _ := payload["success"].(bool)
	if !success {
		errMsg, _ := payload["error"].(string)
		return fmt.Errorf("replication failed: %s", errMsg)
	}

	return nil
}

// ReplicateToAll replicates an operation to all secondary nodes
func (rm *ReplicationManager) ReplicateToAll(ctx context.Context, tableID, operation, key string, value, oldValue interface{}) error {
	members := rm.cluster.GetActiveMembers()
	
	var wg sync.WaitGroup
	errChan := make(chan error, len(members))

	for _, member := range members {
		if member.NodeID == rm.nodeID {
			continue // Skip self
		}

		wg.Add(1)
		go func(nodeID string) {
			defer wg.Done()
			if err := rm.ReplicateToNode(ctx, nodeID, tableID, operation, key, value, oldValue); err != nil {
				errChan <- err
			}
		}(member.NodeID)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		slog.Warn("replication partially failed", "errors", len(errors))
		// For now, just log the errors
		// In production, you might want to implement retry logic or quorum writes
	}

	return nil
}

// HandleReplicateRequest handles an incoming replication request
func (rm *ReplicationManager) HandleReplicateRequest(msg *Message) (*Message, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid replicate request payload")
	}

	tableID, _ := payload["table_id"].(string)
	operation, _ := payload["operation"].(string)
	key, _ := payload["key"].(string)

	slog.Debug("handling replication request", "table_id", tableID, "operation", operation, "key", key, "from", msg.From)

	// Apply the replication operation
	// This would integrate with the storage engine
	// For now, just log and return success

	return &Message{
		Type:      MsgReplicateResponse,
		From:      rm.nodeID,
		To:        msg.From,
		Timestamp: time.Now().Unix(),
		Payload: ReplicateResponsePayload{
			TableID:   tableID,
			Success:   true,
			Timestamp: time.Now().Unix(),
		},
	}, nil
}

// AddReplica adds a replica for a table
func (rm *ReplicationManager) AddReplica(tableID, nodeID, role string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	replicaKey := fmt.Sprintf("%s:%s", tableID, nodeID)
	rm.replicas[replicaKey] = &ReplicaInfo{
		TableID:  tableID,
		NodeID:   nodeID,
		Role:     role,
		LastSync: time.Now(),
		Status:   "ready",
	}
}

// GetReplicas gets all replicas for a table
func (rm *ReplicationManager) GetReplicas(tableID string) []*ReplicaInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var replicas []*ReplicaInfo
	for _, replica := range rm.replicas {
		if replica.TableID == tableID {
			replicas = append(replicas, replica)
		}
	}
	return replicas
}
