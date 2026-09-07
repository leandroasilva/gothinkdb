// Package gothinkdb provides a Go driver for GoThinkDB servers.
//
// Usage:
//
//	conn, err := gothinkdb.Connect(gothinkdb.ConnectOptions{
//	    Host: "localhost",
//	    Port: 28015,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	r := gothinkdb.NewR()
//	res, err := r.Table("users").Filter(map[string]interface{}{"active": true}).Run(conn)
package gothinkdb

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	v1Magic   uint32 = 0x34c2bdc3
	v04Magic  uint32 = 0x400c2d20
	jsonProto uint32 = 0x271ffc41
)

// ConnectOptions configures a connection to a GoThinkDB server.
type ConnectOptions struct {
	Host     string
	Port     int
	DB       string
	User     string
	Password string
	Timeout  time.Duration
}

func (o *ConnectOptions) defaults() {
	if o.Host == "" {
		o.Host = "localhost"
	}
	if o.Port == 0 {
		o.Port = 28015
	}
	if o.DB == "" {
		o.DB = "test"
	}
	if o.User == "" {
		o.User = "admin"
	}
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
}

// Conn represents a connection to a GoThinkDB server.
type Conn struct {
	conn    net.Conn
	opts    ConnectOptions
	token   uint64
	mu      sync.Mutex
	pending map[uint64]chan Response
	reader  *bufio.Reader
	closed  bool
}

// Connect creates a new connection to a GoThinkDB server.
func Connect(opts ConnectOptions) (*Conn, error) {
	opts.defaults()

	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	conn, err := net.DialTimeout("tcp", addr, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c := &Conn{
		conn:    conn,
		opts:    opts,
		token:   0,
		pending: make(map[uint64]chan Response),
		reader:  bufio.NewReader(conn),
	}

	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	go c.readLoop()
	return c, nil
}

// handshake performs the V1.0 protocol handshake.
func (c *Conn) handshake() error {
	// Send magic
	magic := make([]byte, 4)
	binary.LittleEndian.PutUint32(magic, v1Magic)
	if _, err := c.conn.Write(magic); err != nil {
		return err
	}

	// Send auth JSON (null-terminated)
	auth := map[string]interface{}{
		"protocol_version":      1,
		"authentication_method": "SCRAM-SHA-256",
		"authentication":        "",
	}
	authJSON, _ := json.Marshal(auth)
	authJSON = append(authJSON, 0)
	if _, err := c.conn.Write(authJSON); err != nil {
		return err
	}

	// Read response (null-terminated)
	c.conn.SetReadDeadline(time.Now().Add(c.opts.Timeout))
	resp, err := c.readNullTerminated()
	if err != nil {
		return err
	}

	// Parse response
	var authResp map[string]interface{}
	if err := json.Unmarshal(resp, &authResp); err != nil {
		// Try plain text response
		if string(resp) == "SUCCESS" {
			return nil
		}
		return fmt.Errorf("auth response parse error: %s", string(resp))
	}

	if authResp["success"] == true || authResp["authentication"] == "SUCCESS" {
		// Send protocol selection
		proto := make([]byte, 4)
		binary.LittleEndian.PutUint32(proto, jsonProto)
		if _, err := c.conn.Write(proto); err != nil {
			return err
		}

		// Read protocol response
		protoResp, err := c.readNullTerminated()
		if err != nil {
			return err
		}
		if string(protoResp) != "SUCCESS" {
			return fmt.Errorf("protocol handshake failed: %s", string(protoResp))
		}
		c.conn.SetReadDeadline(time.Time{})
		return nil
	}

	if string(resp) == "SUCCESS" {
		return nil
	}

	return fmt.Errorf("authentication failed: %s", string(resp))
}

// readNullTerminated reads bytes until a null byte.
func (c *Conn) readNullTerminated() ([]byte, error) {
	var buf []byte
	for {
		b := make([]byte, 1)
		if _, err := c.conn.Read(b); err != nil {
			return nil, err
		}
		if b[0] == 0 {
			return buf, nil
		}
		buf = append(buf, b[0])
	}
}

// readLoop continuously reads responses from the server.
func (c *Conn) readLoop() {
	for {
		// Read token (8 bytes) + length (4 bytes)
		header := make([]byte, 12)
		if _, err := c.readFull(header); err != nil {
			c.closeAllPending(err)
			return
		}

		token := binary.LittleEndian.Uint64(header[0:8])
		respLen := binary.LittleEndian.Uint32(header[8:12])

		// Read response body
		body := make([]byte, respLen)
		if _, err := c.readFull(body); err != nil {
			c.closeAllPending(err)
			return
		}

		var resp Response
		if err := json.Unmarshal(body, &resp); err != nil {
			c.mu.Lock()
			if ch, ok := c.pending[token]; ok {
				delete(c.pending, token)
				ch <- Response{
					Type:  int(ResponseTypeClientError),
					Error: err.Error(),
				}
			}
			c.mu.Unlock()
			continue
		}
		resp.Token = token

		c.mu.Lock()
		if ch, ok := c.pending[token]; ok {
			delete(c.pending, token)
			ch <- resp
		}
		c.mu.Unlock()
	}
}

// readFull reads exactly len(buf) bytes.
func (c *Conn) readFull(buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		nn, err := c.reader.Read(buf[n:])
		if err != nil {
			return n + nn, err
		}
		n += nn
	}
	return n, nil
}

// closeAllPending rejects all pending queries.
func (c *Conn) closeAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for token, ch := range c.pending {
		ch <- Response{
			Type:  int(ResponseTypeClientError),
			Error: err.Error(),
		}
		delete(c.pending, token)
	}
}

// Query sends a raw query term to the server and returns the response.
func (c *Conn) Query(term interface{}) (Response, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Response{}, fmt.Errorf("connection closed")
	}
	c.token++
	token := c.token
	ch := make(chan Response, 1)
	c.pending[token] = ch
	c.mu.Unlock()

	// Build query: [1, term] (START query type = 1)
	query := []interface{}{1, term}
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return Response{}, err
	}
	queryJSON = append(queryJSON, 0) // null-terminate

	// Send: token (8 bytes) + length (4 bytes) + query
	header := make([]byte, 12)
	binary.LittleEndian.PutUint64(header[0:8], token)
	binary.LittleEndian.PutUint32(header[8:12], uint32(len(queryJSON)))

	if _, err := c.conn.Write(append(header, queryJSON...)); err != nil {
		return Response{}, err
	}

	select {
	case resp := <-ch:
		if resp.Type == int(ResponseTypeRuntimeError) ||
			resp.Type == int(ResponseTypeCompileError) ||
			resp.Type == int(ResponseTypeClientError) {
			return resp, fmt.Errorf("query error: %s", resp.Error)
		}
		return resp, nil
	case <-time.After(c.opts.Timeout):
		c.mu.Lock()
		delete(c.pending, token)
		c.mu.Unlock()
		return Response{}, fmt.Errorf("query timeout")
	}
}

// Close closes the connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.conn.Close()
}

// IsOpen returns true if the connection is open.
func (c *Conn) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// DB returns the default database name.
func (c *Conn) DB() string {
	return c.opts.DB
}
