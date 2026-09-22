package cluster

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TransferManager handles data transfers between nodes
type TransferManager struct {
	transfers map[string]*TransferTask
	mu        sync.RWMutex
}

// NewTransferManager creates a new transfer manager
func NewTransferManager() *TransferManager {
	return &TransferManager{
		transfers: make(map[string]*TransferTask),
	}
}

// StartTransfer starts a data transfer
func (tm *TransferManager) StartTransfer(task *TransferTask) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task.Status = TransferStatusInProgress
	task.StartedAt = time.Now()
	tm.transfers[task.ID] = task

	slog.Info("starting data transfer",
		"transfer_id", task.ID,
		"source", task.SourceNodeID,
		"target", task.TargetNodeID,
		"database", task.Database,
		"table", task.Table,
	)

	// Execute transfer in background
	go tm.executeTransfer(task)
}

// executeTransfer executes the data transfer
func (tm *TransferManager) executeTransfer(task *TransferTask) {
	// In a real implementation, this would:
	// 1. Connect to source node
	// 2. Stream data blocks from source to target
	// 3. Verify data integrity
	// 4. Update progress

	// For now, simulate transfer with progress updates
	steps := 10
	for i := 1; i <= steps; i++ {
		time.Sleep(1 * time.Second) // Simulate work

		tm.mu.Lock()
		task.Progress = (i * 100) / steps
		tm.mu.Unlock()

		slog.Debug("transfer progress",
			"transfer_id", task.ID,
			"progress", task.Progress,
		)
	}

	// Mark as completed
	tm.mu.Lock()
	task.Status = TransferStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	tm.mu.Unlock()

	slog.Info("transfer completed",
		"transfer_id", task.ID,
		"source", task.SourceNodeID,
		"target", task.TargetNodeID,
	)
}

// CreateTransfer creates a new transfer task
func (tm *TransferManager) CreateTransfer(sourceNodeID, targetNodeID, database, table string) *TransferTask {
	return &TransferTask{
		ID:           uuid.New().String(),
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Database:     database,
		Table:        table,
		Status:       TransferStatusPending,
		Progress:     0,
	}
}

// GetTransfer returns a transfer task by ID
func (tm *TransferManager) GetTransfer(transferID string) *TransferTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.transfers[transferID]
}

// ListTransfers returns all transfers
func (tm *TransferManager) ListTransfers() []*TransferTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	transfers := make([]*TransferTask, 0, len(tm.transfers))
	for _, t := range tm.transfers {
		transfers = append(transfers, t)
	}
	return transfers
}

// ListTransfersByNode returns all transfers involving a specific node
func (tm *TransferManager) ListTransfersByNode(nodeID string) []*TransferTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	transfers := make([]*TransferTask, 0)
	for _, t := range tm.transfers {
		if t.SourceNodeID == nodeID || t.TargetNodeID == nodeID {
			transfers = append(transfers, t)
		}
	}
	return transfers
}

// CancelTransfer cancels a transfer
func (tm *TransferManager) CancelTransfer(transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found")
	}

	if task.Status == TransferStatusCompleted {
		return fmt.Errorf("transfer already completed")
	}

	task.Status = TransferStatusFailed
	task.Error = "cancelled"
	now := time.Now()
	task.CompletedAt = &now

	return nil
}

// GetTransferProgress returns the progress of a transfer
func (tm *TransferManager) GetTransferProgress(transferID string) (int, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.transfers[transferID]
	if !exists {
		return 0, fmt.Errorf("transfer not found")
	}

	return task.Progress, nil
}

// CleanupCompletedTransfers removes completed transfers older than 1 hour
func (tm *TransferManager) CleanupCompletedTransfers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for id, task := range tm.transfers {
		if task.Status == TransferStatusCompleted && task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(tm.transfers, id)
		}
	}
}
