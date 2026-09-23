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

	"github.com/leandroasilva/gothinkdb/internal/auth"
	"github.com/leandroasilva/gothinkdb/internal/reql"
)

// Server represents the ReQL protocol server
type Server struct {
	listener      net.Listener
	addr          string
	handler       QueryHandler
	store         *auth.Store
	quit          chan struct{}
	wg            sync.WaitGroup
	connCount     atomic.Int64
	queryCount    atomic.Int64
	serverVersion string
}

// QueryHandler handles incoming queries
type QueryHandler interface {
	HandleQuery(ctx context.Context, conn *Connection, query *Query) (*Response, error)
}

// NewServer creates a new protocol server. The auth store is used to
// authenticate driver connections (SCRAM-SHA-256) and to enforce per-database
// permissions on every query.
func NewServer(addr string, handler QueryHandler, store *auth.Store) *Server {
	return &Server{
		addr:          addr,
		handler:       handler,
		store:         store,
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

	// client-first-message: "n,,n=<user>,r=<client-nonce>"
	clientFirst, _ := clientMsg["authentication"].(string)
	if clientFirst == "" {
		return s.authError(conn, "missing SCRAM client-first message")
	}
	username, clientNonce, clientFirstBare, err := auth.ParseClientFirst(clientFirst)
	if err != nil {
		return s.authError(conn, err.Error())
	}
	if s.store == nil {
		return s.authError(conn, "authentication not configured")
	}
	user, found := s.store.GetUserByUsername(username)
	if !found {
		// Do not reveal whether the user exists; use a generic failure.
		return s.authError(conn, "invalid username or password")
	}
	scram, err := auth.NewSCRAMServer(user, clientFirstBare, clientNonce)
	if err != nil {
		return s.authError(conn, "invalid username or password")
	}

	// server-first-message: "r=<combined-nonce>,s=<salt>,i=<iterations>"
	authResponse := map[string]interface{}{
		"success":        true,
		"authentication": scram.ServerFirst(),
	}
	if err := conn.WriteDatum(authResponse); err != nil {
		return fmt.Errorf("failed to send auth response: %w", err)
	}

	// client-final-message: "c=biws,r=<combined-nonce>,p=<proof>"
	clientFinalMsg, err := conn.ReadDatum()
	if err != nil {
		return fmt.Errorf("failed to read client final auth: %w", err)
	}
	clientFinal, _ := clientFinalMsg["authentication"].(string)
	serverFinal, err := scram.VerifyClientFinal(clientFinal)
	if err != nil {
		return s.authError(conn, "invalid username or password")
	}

	// server-final-message: "v=<server-signature>"
	finalResponse := map[string]interface{}{
		"success":        true,
		"authentication": serverFinal,
	}
	if err := conn.WriteDatum(finalResponse); err != nil {
		return fmt.Errorf("failed to send final response: %w", err)
	}

	// Bind the authenticated user to the connection and set its default DB.
	conn.SetUser(user)
	conn.SetDefaultDB("test")
	conn.SetVersion(Version1_0)
	return nil
}

// authError sends a SCRAM/auth failure response and returns an error so the
// connection is closed by the caller.
func (s *Server) authError(conn *Connection, msg string) error {
	_ = conn.WriteDatum(map[string]interface{}{
		"success":    false,
		"error":      msg,
		"error_code": 20, // ERROR_AUTH_FAILURE in the RethinkDB protocol
	})
	return fmt.Errorf("authentication failed: %s", msg)
}

// handshakeV0_3orV0_4 handles V0_3 and V0_4 handshake with plaintext auth key.
// These legacy versions authenticate with a shared auth key that cannot be
// tied to a per-client identity, so we reject them: clients must use the V1.0
// SCRAM-SHA-256 handshake to get per-connection identity and isolation.
func (s *Server) handshakeV0_3orV0_4(conn *Connection, version uint32) error {
	return fmt.Errorf("protocol version 0x%x is not supported: use the V1.0 SCRAM-SHA-256 handshake", version)
}

// sessionContext binds the connection's authenticated identity to a context so
// the ReQL evaluator can enforce per-database access and resolve unqualified
// db/table references against the connection's default DB (instead of a global).
// If the connection is unauthenticated (e.g. tests) no authorizer is attached
// and access is unrestricted.
func (s *Server) sessionContext(ctx context.Context, conn *Connection) context.Context {
	user := conn.User()
	if user == nil || s.store == nil {
		return reql.WithSession(ctx, conn.DefaultDB(), nil)
	}
	userID := user.ID
	store := s.store
	authz := reql.Authorizer(func(db string, read, write, create, drop bool) error {
		if store.HasDatabaseAccess(userID, db, read, write, create, drop) {
			return nil
		}
		return fmt.Errorf("access denied: user %q has no permission on database %q", user.Username, db)
	})
	return reql.WithSession(ctx, conn.DefaultDB(), authz)
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
			s.queryCount.Add(1)
			baseCtx, cancel := context.WithCancel(context.Background())
			ctx := s.sessionContext(baseCtx, conn)

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

// QueryCount returns the total number of queries processed
func (s *Server) QueryCount() int64 {
	return s.queryCount.Load()
}

// IncrementQueryCount increments the query counter
func (s *Server) IncrementQueryCount() {
	s.queryCount.Add(1)
}
