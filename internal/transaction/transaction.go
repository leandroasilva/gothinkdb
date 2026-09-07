package transaction

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TransactionState represents the state of a transaction
type TransactionState int

const (
	TransactionActive TransactionState = iota
	TransactionCommitted
	TransactionAborted
)

// String returns the string representation of transaction state
func (s TransactionState) String() string {
	switch s {
	case TransactionActive:
		return "active"
	case TransactionCommitted:
		return "committed"
	case TransactionAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

// Operation represents a transaction operation
type Operation struct {
	Type      string      `json:"type"` // "insert", "update", "delete"
	Table     string      `json:"table"`
	Key       string      `json:"key"`
	Value     interface{} `json:"value,omitempty"`
	OldValue  interface{} `json:"old_value,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// Transaction represents a database transaction
type Transaction struct {
	ID         string           `json:"id"`
	State      TransactionState `json:"state"`
	Operations []*Operation     `json:"operations"`
	StartTime  time.Time        `json:"start_time"`
	CommitTime time.Time        `json:"commit_time,omitempty"`
	mu         sync.RWMutex
}

// NewTransaction creates a new transaction
func NewTransaction(id string) *Transaction {
	return &Transaction{
		ID:         id,
		State:      TransactionActive,
		Operations: make([]*Operation, 0),
		StartTime:  time.Now(),
	}
}

// AddOperation adds an operation to the transaction
func (t *Transaction) AddOperation(op *Operation) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransactionActive {
		return fmt.Errorf("transaction is not active")
	}

	op.Timestamp = time.Now().Unix()
	t.Operations = append(t.Operations, op)
	return nil
}

// Commit commits the transaction
func (t *Transaction) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransactionActive {
		return fmt.Errorf("transaction is not active")
	}

	t.State = TransactionCommitted
	t.CommitTime = time.Now()
	return nil
}

// Abort aborts the transaction
func (t *Transaction) Abort() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransactionActive {
		return fmt.Errorf("transaction is not active")
	}

	t.State = TransactionAborted
	t.Operations = nil // Clear operations
	return nil
}

// GetOperations returns all operations in the transaction
func (t *Transaction) GetOperations() []*Operation {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Operations
}

// TransactionManager manages transactions
type TransactionManager struct {
	transactions map[string]*Transaction
	mu           sync.RWMutex
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		transactions: make(map[string]*Transaction),
	}
}

// Begin begins a new transaction
func (tm *TransactionManager) Begin(id string) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.transactions[id]; exists {
		return nil, fmt.Errorf("transaction %s already exists", id)
	}

	txn := NewTransaction(id)
	tm.transactions[id] = txn
	return txn, nil
}

// Get returns a transaction by ID
func (tm *TransactionManager) Get(id string) (*Transaction, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	txn, exists := tm.transactions[id]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", id)
	}

	return txn, nil
}

// Commit commits a transaction
func (tm *TransactionManager) Commit(ctx context.Context, id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.transactions[id]
	if !exists {
		return fmt.Errorf("transaction %s not found", id)
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	// Apply operations to storage
	// This would integrate with the storage engine
	for _, op := range txn.Operations {
		// Apply operation
		_ = op
	}

	// Remove from active transactions
	delete(tm.transactions, id)
	return nil
}

// Abort aborts a transaction
func (tm *TransactionManager) Abort(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.transactions[id]
	if !exists {
		return fmt.Errorf("transaction %s not found", id)
	}

	if err := txn.Abort(); err != nil {
		return err
	}

	// Remove from active transactions
	delete(tm.transactions, id)
	return nil
}

// GetActiveTransactions returns all active transactions
func (tm *TransactionManager) GetActiveTransactions() []*Transaction {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var active []*Transaction
	for _, txn := range tm.transactions {
		if txn.State == TransactionActive {
			active = append(active, txn)
		}
	}
	return active
}
