package protocol

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Server represents the ReQL protocol server
type Server struct {
	listener      net.Listener
	addr          string
	handler       QueryHandler
	quit          chan struct{}
	wg            sync.WaitGroup
	connCount     atomic.Int64
	serverVersion string
}

// QueryHandler handles incoming queries
type QueryHandler interface {
	HandleQuery(ctx context.Context, conn *Connection, query *Query) (*Response, error)
}

// NewServer creates a new protocol server
func NewServer(addr string, handler QueryHandler) *Server {
	return &Server{
		addr:          addr,
		handler:       handler,
		quit:          make(chan struct{}),
		serverVersion: "0.1.0",
	}
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.listener = listener

	slog.Info("protocol server starting", "address", s.addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// Stop stops the server gracefully
func (s *Server) Stop() error {
	close(s.quit)

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
	}

	s.wg.Wait()
	return nil
}

// Addr returns the server's address
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				slog.Error("accept error", "error", err)
				continue
			}
		}

		s.connCount.Add(1)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.connCount.Add(-1)
			s.handleConnection(conn)
		}()
	}
}

func (s *Server) handleConnection(netConn net.Conn) {
	defer netConn.Close()

	conn := NewConnection(netConn)

	slog.Info("new connection", "remote", netConn.RemoteAddr())

	// Perform handshake
	if err := s.handshake(conn); err != nil {
		slog.Error("handshake failed", "remote", netConn.RemoteAddr(), "error", err)
		return
	}

	slog.Info("connection authenticated", "remote", netConn.RemoteAddr(), "version", conn.Version())

	// Handle queries
	s.queryLoop(conn)
}

func (s *Server) handshake(conn *Connection) error {
	// Read version (4 bytes, little-endian)
	var version uint32
	if err := binary.Read(conn, binary.LittleEndian, &version); err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}

	slog.Debug("received version", "version", fmt.Sprintf("0x%x", version), "remote", conn.RemoteAddr())

	// Check version and perform appropriate handshake
	switch version {
	case Version1_0:
		return s.handshakeV1_0(conn)
	case Version0_4, Version0_3:
		return s.handshakeV0_3orV0_4(conn, version)
	case Version0_2, Version0_1:
		return fmt.Errorf("protocol versions V0_1 and V0_2 are no longer supported (PROBUF protocol removed)")
	default:
		return fmt.Errorf("unsupported protocol version: 0x%x", version)
	}
}

// handshakeV1_0 handles V1.0 handshake with SCRAM-SHA-256 authentication
func (s *Server) handshakeV1_0(conn *Connection) error {
	// Step 1: Server sends initial response with version info
	serverResponse := map[string]interface{}{
		"success":              true,
		"max_protocol_version": 0,
		"min_protocol_version": 0,
		"server_version":       s.serverVersion,
	}

	if err := conn.WriteDatum(serverResponse); err != nil {
		return fmt.Errorf("failed to send server response: %w", err)
	}

	// Step 2: Client sends protocol version and authentication method
	clientMsg, err := conn.ReadDatum()
	if err != nil {
		return fmt.Errorf("failed to read client authentication: %w", err)
	}

	// Validate protocol version
	protocolVersion, ok := clientMsg["protocol_version"].(float64)
	if !ok {
		return fmt.Errorf("invalid protocol_version")
	}
	if protocolVersion != 0 {
		return fmt.Errorf("unsupported protocol_version: %v", protocolVersion)
	}

	// Validate authentication method
	authMethod, ok := clientMsg["authentication_method"].(string)
	if !ok {
		return fmt.Errorf("invalid authentication_method")
	}
	if authMethod != "SCRAM-SHA-256" {
		return fmt.Errorf("unsupported authentication_method: %s", authMethod)
	}

	// For now, accept all authentication (simplified)
	// In production, this would implement full SCRAM-SHA-256
	authResponse := map[string]interface{}{
		"success":        true,
		"authentication": "", // SCRAM challenge would go here
	}

	if err := conn.WriteDatum(authResponse); err != nil {
		return fmt.Errorf("failed to send auth response: %w", err)
	}

	// Step 3: Client sends final authentication message
	clientFinal, err := conn.ReadDatum()
	if err != nil {
		return fmt.Errorf("failed to read client final auth: %w", err)
	}

	_ = clientFinal // Would validate SCRAM response

	// Step 4: Server sends final success response
	finalResponse := map[string]interface{}{
		"success":        true,
		"authentication": "", // SCRAM server final would go here
	}

	if err := conn.WriteDatum(finalResponse); err != nil {
		return fmt.Errorf("failed to send final response: %w", err)
	}

	conn.SetVersion(Version1_0)
	return nil
}

// handshakeV0_3orV0_4 handles V0_3 and V0_4 handshake with plaintext auth key
func (s *Server) handshakeV0_3orV0_4(conn *Connection, version uint32) error {
	// Read auth key size (4 bytes, little-endian)
	var authKeySize uint32
	if err := binary.Read(conn, binary.LittleEndian, &authKeySize); err != nil {
		return fmt.Errorf("failed to read auth key size: %w", err)
	}

	if authKeySize > 2048 {
		return fmt.Errorf("auth key too large: %d bytes", authKeySize)
	}

	// Read auth key
	authKey := make([]byte, authKeySize)
	if _, err := io.ReadFull(conn, authKey); err != nil {
		return fmt.Errorf("failed to read auth key: %w", err)
	}

	// For now, accept all auth keys (empty or not)
	_ = authKey

	// Read wire protocol (4 bytes, little-endian)
	var wireProtocol uint32
	if err := binary.Read(conn, binary.LittleEndian, &wireProtocol); err != nil {
		return fmt.Errorf("failed to read wire protocol: %w", err)
	}

	// Validate wire protocol
	if wireProtocol != ProtocolJSON {
		return fmt.Errorf("unsupported wire protocol: 0x%x (only JSON is supported)", wireProtocol)
	}

	// Send success response
	if _, err := conn.Write([]byte("SUCCESS\000")); err != nil {
		return fmt.Errorf("failed to send success: %w", err)
	}

	conn.SetVersion(version)
	return nil
}

func (s *Server) queryLoop(conn *Connection) {
	queries := make(map[int64]context.CancelFunc)
	var queriesMu sync.Mutex

	for {
		select {
		case <-s.quit:
			return
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

		// Read query length (4 bytes, little-endian)
		var length uint32
		if err := binary.Read(conn, binary.LittleEndian, &length); err != nil {
			if err == io.EOF {
				slog.Info("connection closed", "remote", conn.RemoteAddr())
				return
			}
			slog.Error("failed to read query length", "error", err, "remote", conn.RemoteAddr())
			return
		}

		// Sanity check length
		if length > 64*1024*1024 { // 64MB max
			slog.Error("query too large", "length", length, "remote", conn.RemoteAddr())
			return
		}

		// Read query data
		data := make([]byte, length)
		if _, err := io.ReadFull(conn, data); err != nil {
			slog.Error("failed to read query", "error", err, "remote", conn.RemoteAddr())
			return
		}

		// Parse query
		query, err := ParseQuery(data)
		if err != nil {
			slog.Error("failed to parse query", "error", err, "remote", conn.RemoteAddr())
			s.sendError(conn, 0, ResponseClientError, err.Error())
			continue
		}

		// Handle query
		switch query.Type {
		case QueryStart:
			ctx, cancel := context.WithCancel(context.Background())

			queriesMu.Lock()
			queries[query.Token] = cancel
			queriesMu.Unlock()

			go func() {
				defer func() {
					queriesMu.Lock()
					delete(queries, query.Token)
					queriesMu.Unlock()
					cancel()
				}()

				resp, err := s.handler.HandleQuery(ctx, conn, query)
				if err != nil {
					s.sendError(conn, query.Token, ResponseRuntimeError, err.Error())
					return
				}

				s.sendResponse(conn, query.Token, resp)
			}()

		case QueryContinue:
			// TODO: Implement cursor continuation
			s.sendResponse(conn, query.Token, &Response{
				Type: ResponseSuccessSeq,
				Data: []interface{}{},
			})

		case QueryStop:
			queriesMu.Lock()
			if cancel, ok := queries[query.Token]; ok {
				cancel()
				delete(queries, query.Token)
			}
			queriesMu.Unlock()

			s.sendResponse(conn, query.Token, &Response{
				Type: ResponseWaitComplete,
			})

		case QueryNoReplyWait:
			// Wait for all queries to complete
			s.sendResponse(conn, query.Token, &Response{
				Type: ResponseWaitComplete,
			})

		case QueryServerInfo:
			s.sendResponse(conn, query.Token, &Response{
				Type: ResponseSuccessAtom,
				Data: []interface{}{
					map[string]interface{}{
						"id":      "gothinkdb-server",
						"name":    "gothinkdb",
						"version": s.serverVersion,
					},
				},
			})

		default:
			s.sendError(conn, query.Token, ResponseClientError, fmt.Sprintf("Unknown query type: %d", query.Type))
		}
	}
}

func (s *Server) sendResponse(conn *Connection, token int64, resp *Response) {
	data, err := resp.Marshal()
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		return
	}

	// Write response length (4 bytes)
	var length uint32 = uint32(len(data))
	if err := binary.Write(conn, binary.LittleEndian, length); err != nil {
		slog.Error("failed to write response length", "error", err)
		return
	}

	// Write response data
	if _, err := conn.Write(data); err != nil {
		slog.Error("failed to write response", "error", err)
		return
	}
}

func (s *Server) sendError(conn *Connection, token int64, errType int64, message string) {
	resp := &Response{
		Type:  errType,
		Data:  []interface{}{message},
		Notes: []string{},
	}
	s.sendResponse(conn, token, resp)
}

// ConnectionCount returns the number of active connections
func (s *Server) ConnectionCount() int64 {
	return s.connCount.Load()
}
