package cluster

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RemoveManager handles graceful node removal
type RemoveManager struct {
	joinManager     *JoinManager
	transferManager *TransferManager
	operations      map[string]*RemoveOperation // nodeID -> operation
	mu              sync.RWMutex
}

// NewRemoveManager creates a new remove manager
func NewRemoveManager(joinManager *JoinManager, transferManager *TransferManager) *RemoveManager {
	return &RemoveManager{
		joinManager:     joinManager,
		transferManager: transferManager,
		operations:      make(map[string]*RemoveOperation),
	}
}

// StartRemoval starts the graceful removal of a node
func (rm *RemoveManager) StartRemoval(nodeID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if node exists
	member, exists := rm.joinManager.GetMember(nodeID)
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Check if already removing
	if _, exists := rm.operations[nodeID]; exists {
		return fmt.Errorf("removal already in progress for node %s", nodeID)
	}

	// Mark node as draining
	if err := rm.joinManager.UpdateMemberStatus(nodeID, NodeStatusDraining); err != nil {
		return fmt.Errorf("failed to mark node as draining: %w", err)
	}

	// Create removal operation
	operation := &RemoveOperation{
		NodeID:    nodeID,
		Status:    "draining",
		Transfers: make([]*TransferTask, 0),
		StartedAt: time.Now(),
	}

	rm.operations[nodeID] = operation

	slog.Info("starting node removal",
		"node_id", nodeID,
		"address", member.Address,
	)

	// Start removal process in background
	go rm.executeRemoval(nodeID)

	return nil
}

// executeRemoval executes the removal process
func (rm *RemoveManager) executeRemoval(nodeID string) {
	// Step 1: Identify data to transfer
	rm.updateOperationStatus(nodeID, "identifying_data")

	// Get member info
	member, exists := rm.joinManager.GetMember(nodeID)
	if !exists {
		rm.failOperation(nodeID, "node not found")
		return
	}

	// Step 2: Create transfer tasks for data on this node
	// In a real implementation, this would query the node for its data
	// For now, we simulate with a placeholder transfer
	transfers := rm.identifyTransfers(nodeID, member)

	rm.mu.Lock()
	if op, exists := rm.operations[nodeID]; exists {
		op.Transfers = transfers
		op.Status = "transferring"
	}
	rm.mu.Unlock()

	// Step 3: Execute transfers
	for _, transfer := range transfers {
		rm.transferManager.StartTransfer(transfer)
	}

	// Step 4: Wait for transfers to complete
	rm.waitForTransfers(nodeID, transfers)

	// Step 5: Send shutdown command to node
	rm.updateOperationStatus(nodeID, "shutting_down")
	rm.sendShutdownCommand(nodeID, member)

	// Step 6: Remove node from cluster
	rm.updateOperationStatus(nodeID, "removing")
	if err := rm.joinManager.RemoveMember(nodeID); err != nil {
		slog.Error("failed to remove node from cluster", "error", err)
		rm.failOperation(nodeID, fmt.Sprintf("failed to remove node: %v", err))
		return
	}

	// Step 7: Mark operation as completed
	rm.mu.Lock()
	if op, exists := rm.operations[nodeID]; exists {
		now := time.Now()
		op.Status = "completed"
		op.CompletedAt = &now
	}
	rm.mu.Unlock()

	slog.Info("node removal completed", "node_id", nodeID)
}

// identifyTransfers identifies data that needs to be transferred
func (rm *RemoveManager) identifyTransfers(nodeID string, member *NodeInfo) []*TransferTask {
	// In a real implementation, this would:
	// 1. Query the node for its databases and tables
	// 2. Check which data has no replicas
	// 3. Create transfer tasks for unique data

	// For now, return empty list (no data to transfer in this simulation)
	return make([]*TransferTask, 0)
}

// waitForTransfers waits for all transfers to complete
func (rm *RemoveManager) waitForTransfers(nodeID string, transfers []*TransferTask) {
	if len(transfers) == 0 {
		return
	}

	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			slog.Error("transfer timeout", "node_id", nodeID)
			rm.failOperation(nodeID, "transfer timeout")
			return
		case <-ticker.C:
			allDone := true
			for _, t := range transfers {
				task := rm.transferManager.GetTransfer(t.ID)
				if task != nil && task.Status != TransferStatusCompleted && task.Status != TransferStatusFailed {
					allDone = false
					break
				}
			}
			if allDone {
				return
			}
		}
	}
}

// sendShutdownCommand sends a shutdown command to the node
func (rm *RemoveManager) sendShutdownCommand(nodeID string, member *NodeInfo) {
	// In a real implementation, this would send an HTTP request to the node
	// telling it to shut down and delete its data
	slog.Info("sending shutdown command to node",
		"node_id", nodeID,
		"address", member.Address,
	)
}

// updateOperationStatus updates the status of a removal operation
func (rm *RemoveManager) updateOperationStatus(nodeID, status string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if op, exists := rm.operations[nodeID]; exists {
		op.Status = status
	}
}

// failOperation marks an operation as failed
func (rm *RemoveManager) failOperation(nodeID, errMsg string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if op, exists := rm.operations[nodeID]; exists {
		op.Status = "failed"
		op.Error = errMsg
		now := time.Now()
		op.CompletedAt = &now
	}
}

// GetRemovalStatus returns the status of a removal operation
func (rm *RemoveManager) GetRemovalStatus(nodeID string) (*RemoveOperation, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	op, exists := rm.operations[nodeID]
	if !exists {
		return nil, fmt.Errorf("no removal operation found for node %s", nodeID)
	}

	return op, nil
}

// ListRemovals returns all removal operations
func (rm *RemoveManager) ListRemovals() []*RemoveOperation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	ops := make([]*RemoveOperation, 0, len(rm.operations))
	for _, op := range rm.operations {
		ops = append(ops, op)
	}
	return ops
}

// CleanupCompletedRemovals removes completed operations older than 1 hour
func (rm *RemoveManager) CleanupCompletedRemovals() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for nodeID, op := range rm.operations {
		if op.Status == "completed" && op.CompletedAt != nil && op.CompletedAt.Before(cutoff) {
			delete(rm.operations, nodeID)
		}
	}
}

// GenerateNodeID generates a unique node ID
func GenerateNodeID() string {
	return uuid.New().String()
}
