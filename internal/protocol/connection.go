package protocol

import (
	"bufio"
	"encoding/json"
	"net"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/auth"
)

// Connection wraps a net.Conn with protocol-specific functionality
type Connection struct {
	net.Conn
	reader  *bufio.Reader
	version uint32
	user      *auth.User
	defaultDB string
}

// NewConnection creates a new Connection
func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

// SetUser associates the authenticated user with this connection.
func (c *Connection) SetUser(u *auth.User) { c.user = u }

// User returns the authenticated user (nil if unauthenticated).
func (c *Connection) User() *auth.User { return c.user }

// SetDefaultDB sets the connection's default database (per-connection scope).
func (c *Connection) SetDefaultDB(db string) { c.defaultDB = db }

// DefaultDB returns the connection's default database.
func (c *Connection) DefaultDB() string { return c.defaultDB }

// SetVersion sets the protocol version for this connection
func (c *Connection) SetVersion(version uint32) {
	c.version = version
}

// Version returns the protocol version
func (c *Connection) Version() uint32 {
	return c.version
}

// ReadNullTerminated reads a null-terminated string
func (c *Connection) ReadNullTerminated() (string, error) {
	var result []byte
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			break
		}
		result = append(result, b)
	}
	return string(result), nil
}

// SetReadDeadline sets the read deadline
func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

// WriteDatum writes a null-terminated JSON datum to the connection
func (c *Connection) WriteDatum(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// Append null terminator
	data = append(data, 0)
	_, err = c.Conn.Write(data)
	return err
}

// ReadDatum reads a null-terminated JSON datum from the connection
func (c *Connection) ReadDatum() (map[string]interface{}, error) {
	var result []byte
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 0 {
			break
		}
		result = append(result, b)
		if len(result) > 2048 {
			return nil, ErrDatumTooLarge
		}
	}

	var datum map[string]interface{}
	if err := json.Unmarshal(result, &datum); err != nil {
		return nil, err
	}
	return datum, nil
}
