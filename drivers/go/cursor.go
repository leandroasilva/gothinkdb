package gothinkdb

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Cursor iterates over query results.
type Cursor struct {
	conn  *Conn
	token uint64
	data  []json.RawMessage
	index int
	done  bool
	mu    sync.Mutex
	err   error
}

// NewCursor creates a cursor from a response.
func NewCursor(conn *Conn, resp Response) (*Cursor, error) {
	var data []json.RawMessage

	switch ResponseType(resp.Type) {
	case ResponseTypeSuccessAtom:
		// Single value - wrap in array
		data = []json.RawMessage{resp.Data}
	case ResponseTypeSuccessSequence:
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			// Try wrapping single value
			data = []json.RawMessage{resp.Data}
		}
	case ResponseTypeSuccessPartial:
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			data = []json.RawMessage{resp.Data}
		}
	default:
		return nil, fmt.Errorf("unexpected response type for cursor: %d", resp.Type)
	}

	return &Cursor{
		conn:  conn,
		token: resp.Token,
		data:  data,
		index: 0,
		done:  ResponseType(resp.Type) != ResponseTypeSuccessPartial,
	}, nil
}

// Next returns the next item from the cursor.
// Returns false when no more items are available.
func (c *Cursor) Next(dest interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.err != nil {
		return false
	}

	// If we've consumed all local data
	if c.index >= len(c.data) {
		if c.done {
			return false
		}
		// TODO: fetch more data from server (CONTINUE query)
		return false
	}

	item := c.data[c.index]
	c.index++

	if dest != nil {
		if err := json.Unmarshal(item, dest); err != nil {
			c.err = err
			return false
		}
	}
	return true
}

// All reads all remaining items from the cursor.
func (c *Cursor) All() ([]json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]json.RawMessage, 0, len(c.data)-c.index)
	for c.index < len(c.data) {
		result = append(result, c.data[c.index])
		c.index++
	}
	return result, nil
}

// One reads a single item from the cursor.
func (c *Cursor) One(dest interface{}) error {
	if !c.Next(dest) {
		if c.err != nil {
			return c.err
		}
		return fmt.Errorf("no more rows in cursor")
	}
	return nil
}

// Close closes the cursor and sends a STOP query to the server.
func (c *Cursor) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done = true
	// TODO: send STOP query (type 2) to server
	return nil
}

// IsNil returns true if the cursor has no more data.
func (c *Cursor) IsNil() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done && c.index >= len(c.data)
}
