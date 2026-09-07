package reql

import (
	"sync"

	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// ChangeEvent represents a change to a document
type ChangeEvent struct {
	OldValue datum.Datum
	NewValue datum.Datum
}

// Changefeed represents a subscription to table changes
type Changefeed struct {
	Table    string
	Events   chan ChangeEvent
	closed   bool
	mu       sync.Mutex
}

// NewChangefeed creates a new changefeed
func NewChangefeed(table string) *Changefeed {
	return &Changefeed{
		Table:  table,
		Events: make(chan ChangeEvent, 100),
	}
}

// Send sends a change event to the changefeed
func (cf *Changefeed) Send(event ChangeEvent) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if !cf.closed {
		select {
		case cf.Events <- event:
		default:
			// Channel full, drop event
		}
	}
}

// Close closes the changefeed
func (cf *Changefeed) Close() {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if !cf.closed {
		cf.closed = true
		close(cf.Events)
	}
}

// IsClosed returns true if the changefeed is closed
func (cf *Changefeed) IsClosed() bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	return cf.closed
}
