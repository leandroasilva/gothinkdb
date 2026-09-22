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
	jsonProto uint32 = 0x7e6970c7
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
	conn   net.Conn
	opts   ConnectOptions
	token  uint64
	mu     sync.Mutex
	reader *bufio.Reader
	closed bool
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
		conn:   conn,
		opts:   opts,
		token:  0,
		reader: bufio.NewReader(conn),
	}

	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	return c, nil
}

// handshake performs the V0.4 protocol handshake.
func (c *Conn) handshake() error {
	// Send V0.4 version magic
	magic := make([]byte, 4)
	binary.LittleEndian.PutUint32(magic, v04Magic)
	if _, err := c.conn.Write(magic); err != nil {
		return err
	}

	// Send auth key size (0 = no auth key)
	authKeySize := make([]byte, 4)
	binary.LittleEndian.PutUint32(authKeySize, 0)
	if _, err := c.conn.Write(authKeySize); err != nil {
		return err
	}

	// Send wire protocol (JSON)
	proto := make([]byte, 4)
	binary.LittleEndian.PutUint32(proto, jsonProto)
	if _, err := c.conn.Write(proto); err != nil {
		return err
	}

	// Read SUCCESS response (null-terminated)
	c.conn.SetReadDeadline(time.Now().Add(c.opts.Timeout))
	resp, err := c.readNullTerminated()
	if err != nil {
		return err
	}
	c.conn.SetReadDeadline(time.Time{})

	if string(resp) != "SUCCESS" {
		return fmt.Errorf("handshake failed: %s", string(resp))
	}

	return nil
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

// Query sends a raw query term to the server and returns the response.
func (c *Conn) Query(term interface{}) (Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return Response{}, fmt.Errorf("connection closed")
	}

	c.token++
	token := c.token

	// Build query JSON: {"token": N, "type": 1, "query": term}
	queryObj := map[string]interface{}{
		"token": token,
		"type":  1, // QueryStart
		"query": term,
	}
	queryJSON, err := json.Marshal(queryObj)
	if err != nil {
		return Response{}, err
	}

	// Send: length (4 bytes LE) + JSON data
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(queryJSON)))

	if _, err := c.conn.Write(append(header, queryJSON...)); err != nil {
		return Response{}, err
	}

	// Read response: length (4 bytes LE) + JSON data
	respHeader := make([]byte, 4)
	if _, err := c.readFull(respHeader); err != nil {
		return Response{}, fmt.Errorf("failed to read response header: %w", err)
	}

	respLen := binary.LittleEndian.Uint32(respHeader)
	if respLen > 64*1024*1024 {
		return Response{}, fmt.Errorf("response too large: %d bytes", respLen)
	}

	body := make([]byte, respLen)
	if _, err := c.readFull(body); err != nil {
		return Response{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return Response{}, fmt.Errorf("failed to parse response: %w", err)
	}
	resp.Token = token

	if resp.Type == int(ResponseTypeRuntimeError) ||
		resp.Type == int(ResponseTypeCompileError) ||
		resp.Type == int(ResponseTypeClientError) {
		errMsg := ""
		if len(resp.Notes) > 0 {
			errMsg = resp.Notes[0]
		} else if resp.Data != nil {
			var dataArr []interface{}
			if err := json.Unmarshal(resp.Data, &dataArr); err == nil && len(dataArr) > 0 {
				errMsg = fmt.Sprintf("%v", dataArr[0])
			}
		}
		if errMsg == "" {
			errMsg = string(resp.Data)
		}
		return resp, fmt.Errorf("query error: %s", errMsg)
	}

	return resp, nil
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
