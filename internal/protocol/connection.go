package protocol

import (
	"bufio"
	"net"
	"time"
)

// Connection wraps a net.Conn with protocol-specific functionality
type Connection struct {
	net.Conn
	reader  *bufio.Reader
	version uint32
}

// NewConnection creates a new Connection
func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

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
