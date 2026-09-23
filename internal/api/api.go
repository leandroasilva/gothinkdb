package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/auth"
	"github.com/leandroasilva/gothinkdb/internal/protocol"
	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/internal/rpc"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// MetricsTracker tracks real-time server metrics
type MetricsTracker struct {
	queriesTotal      atomic.Int64
	queriesPerSec     atomic.Int64
	lastQueryTime     atomic.Int64 // unix nano
	startTime         time.Time
	logEntries        []LogEntry
	logMu             sync.RWMutex
	metricsHistory    []MetricsSnapshot
	metricsMu         sync.RWMutex
	maxLogEntries     int
	maxMetricsHistory int
}

// LogEntry represents a server log entry
type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Server    string `json:"server"`
}

// MetricsSnapshot is a point-in-time metrics sample for charts
type MetricsSnapshot struct {
	Time    string `json:"time"`
	Queries int64  `json:"queries"`
	Latency int64  `json:"latency"`
}

// Server represents the API server
type Server struct {
	evaluator      *reql.Evaluator
	cluster        *rpc.ClusterManager
	clusterManager *ClusterManagerWrapper
	wsHub          *WebSocketHub
	auth           *auth.Service
	protocolServer *protocol.Server
	metrics        *MetricsTracker
	mux            *http.ServeMux
	mu             sync.RWMutex
}

// NewServer creates a new API server
func NewServer(evaluator *reql.Evaluator, cluster *rpc.ClusterManager, authService *auth.Service, clusterManager *ClusterManagerWrapper, protocolServer *protocol.Server) *Server {
	hub := NewWebSocketHub()
	go hub.Run()

	metrics := &MetricsTracker{
		startTime:         time.Now(),
		logEntries:        make([]LogEntry, 0, 500),
		metricsHistory:    make([]MetricsSnapshot, 0, 60),
		maxLogEntries:     500,
		maxMetricsHistory: 60,
	}

	s := &Server{
		evaluator:      evaluator,
		cluster:        cluster,
		clusterManager: clusterManager,
		wsHub:          hub,
		auth:           authService,
		protocolServer: protocolServer,
		metrics:        metrics,
		mux:            http.NewServeMux(),
	}

	s.setupRoutes()
	s.startMetricsCollection()
	return s
}

// GetWebSocketHub returns the WebSocket hub
func (s *Server) GetWebSocketHub() *WebSocketHub {
	return s.wsHub
}

// SetProtocolServer sets the protocol server reference for metrics
func (s *Server) SetProtocolServer(ps *protocol.Server) {
	s.protocolServer = ps
}

// setupRoutes sets up API routes
func (s *Server) setupRoutes() {
	// Health check (no auth required)
	s.mux.HandleFunc("/api/health", s.handleHealth)

	// Auth endpoints
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/me", s.authMiddleware(s.handleMe))

	// User management (admin only)
	s.mux.HandleFunc("/api/users", s.authMiddleware(s.adminMiddleware(s.handleUsers)))
	s.mux.HandleFunc("/api/users/", s.authMiddleware(s.handleUserRouter))

	// Cluster (auth required)
	s.mux.HandleFunc("/api/cluster/status", s.authMiddleware(s.handleClusterStatus))
	s.mux.HandleFunc("/api/cluster/members", s.authMiddleware(s.handleClusterMembers))
	s.mux.HandleFunc("/api/cluster/join", s.handleClusterJoin)
	s.mux.HandleFunc("/api/cluster/tokens", s.authMiddleware(s.handleClusterTokens))
	s.mux.HandleFunc("/api/cluster/tokens/", s.authMiddleware(s.handleClusterToken))
	s.mux.HandleFunc("/api/cluster/nodes", s.authMiddleware(s.handleClusterNodes))
	s.mux.HandleFunc("/api/cluster/nodes/", s.authMiddleware(s.handleClusterNode))
	s.mux.HandleFunc("/api/cluster/balance", s.authMiddleware(s.handleClusterBalance))
	s.mux.HandleFunc("/api/cluster/transfers", s.authMiddleware(s.handleClusterTransfers))

	// Databases (auth required, permission checked)
	s.mux.HandleFunc("/api/databases", s.authMiddleware(s.handleDatabases))
	s.mux.HandleFunc("/api/databases/", s.authMiddleware(s.handleDatabase))

	// Tables (auth required, permission checked)
	s.mux.HandleFunc("/api/tables", s.authMiddleware(s.handleTables))
	s.mux.HandleFunc("/api/tables/", s.authMiddleware(s.handleTable))

	// Queries (auth required)
	s.mux.HandleFunc("/api/query", s.authMiddleware(s.handleQuery))

	// Server info (auth required)
	s.mux.HandleFunc("/api/server/info", s.authMiddleware(s.handleServerInfo))
	s.mux.HandleFunc("/api/server/stats", s.authMiddleware(s.handleServerStats))

	// Logs (auth required)
	s.mux.HandleFunc("/api/logs", s.authMiddleware(s.handleLogs))

	// Metrics (auth required)
	s.mux.HandleFunc("/api/metrics", s.authMiddleware(s.handleMetrics))
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// startMetricsCollection starts background goroutines for metrics collection
func (s *Server) startMetricsCollection() {
	// Add initial log entries
	s.addLogEntry("info", "GoThinkDB server started successfully")
	s.addLogEntry("info", fmt.Sprintf("HTTP admin server listening on :8080"))
	s.addLogEntry("info", fmt.Sprintf("ReQL protocol server listening on :28015"))

	// Start metrics sampling every second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		var lastQueries int64
		for range ticker.C {
			// Include protocol server queries
			var protocolQueries int64
			if s.protocolServer != nil {
				protocolQueries = s.protocolServer.QueryCount()
			}
			current := s.metrics.queriesTotal.Load() + protocolQueries
			qps := current - lastQueries
			lastQueries = current
			s.metrics.queriesPerSec.Store(qps)

			// Calculate latency estimate (time since last query)
			var latency int64 = 0
			lastQ := s.metrics.lastQueryTime.Load()
			if lastQ > 0 {
				elapsed := time.Since(time.Unix(0, lastQ))
				latency = elapsed.Milliseconds()
			}

			snapshot := MetricsSnapshot{
				Time:    time.Now().Format("15:04:05"),
				Queries: current,
				Latency: latency,
			}

			s.metrics.metricsMu.Lock()
			s.metrics.metricsHistory = append(s.metrics.metricsHistory, snapshot)
			if len(s.metrics.metricsHistory) > s.metrics.maxMetricsHistory {
				s.metrics.metricsHistory = s.metrics.metricsHistory[1:]
			}
			s.metrics.metricsMu.Unlock()
		}
	}()
}

// addLogEntry adds a log entry to the in-memory buffer
func (s *Server) addLogEntry(level, message string) {
	s.metrics.logMu.Lock()
	defer s.metrics.logMu.Unlock()

	entry := LogEntry{
		ID:        fmt.Sprintf("%d", len(s.metrics.logEntries)+1),
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Server:    "gothinkdb",
	}
	s.metrics.logEntries = append(s.metrics.logEntries, entry)

	// Trim to max entries
	if len(s.metrics.logEntries) > s.metrics.maxLogEntries {
		s.metrics.logEntries = s.metrics.logEntries[len(s.metrics.logEntries)-s.metrics.maxLogEntries:]
	}
}

// RecordQuery records a query execution for metrics
func (s *Server) RecordQuery() {
	s.metrics.queriesTotal.Add(1)
	s.metrics.lastQueryTime.Store(time.Now().UnixNano())
}

// RecordLog records a log entry from external sources
func (s *Server) RecordLog(level, message string) {
	s.addLogEntry(level, message)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	slog.Debug("health check request")
	uptime := time.Since(s.metrics.startTime).Round(time.Second).String()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"server": "gothinkdb",
		"uptime": uptime,
	})
}

// handleMetrics handles metrics requests for chart data
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.metrics.metricsMu.RLock()
	history := make([]MetricsSnapshot, len(s.metrics.metricsHistory))
	copy(history, s.metrics.metricsHistory)
	s.metrics.metricsMu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"history":         history,
		"queries_total":   s.metrics.queriesTotal.Load(),
		"queries_per_sec": s.metrics.queriesPerSec.Load(),
		"memory_used_mb":  memStats.Alloc / 1024 / 1024,
		"connections":     s.getConnectionCount(),
	})
}

// getConnectionCount returns the number of active protocol connections
func (s *Server) getConnectionCount() int64 {
	if s.protocolServer != nil {
		return s.protocolServer.ConnectionCount()
	}
	return 0
}
func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	if s.cluster == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "standalone",
			"cluster_id": "",
			"members":    0,
			"leader":     "",
		})
		return
	}

	state := s.cluster.GetClusterState()
	writeJSON(w, http.StatusOK, state)
}

// handleClusterMembers handles cluster members requests
func (s *Server) handleClusterMembers(w http.ResponseWriter, r *http.Request) {
	if s.cluster == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	members := s.cluster.GetMembers()
	writeJSON(w, http.StatusOK, members)
}

// handleDatabases handles database list requests
func (s *Server) handleDatabases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		databases := s.evaluator.GetAdmin().ListDatabases()
		// Filter databases based on user permissions
		databases = s.filterDatabasesByPermission(r, databases)
		writeJSON(w, http.StatusOK, databases)
	case http.MethodPost:
		// Only admin can create databases
		user, _ := auth.UserFromContext(r.Context())
		if user == nil || !user.IsAdmin() {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.evaluator.GetAdmin().CreateDatabase(req.Name); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		s.RecordLog("info", fmt.Sprintf("Database '%s' created by user '%s'", req.Name, user.Username))
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleDatabase handles single database requests
func (s *Server) handleDatabase(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/api/databases/"):]
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "database name required"})
		return
	}

	// Sub-resource: GET /api/databases/{name}/size returns the approximate
	// database size in MB (used by the panel for quota enforcement).
	if strings.HasSuffix(name, "/size") {
		dbName := strings.TrimSuffix(name, "/size")
		if dbName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "database name required"})
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if user != nil && !user.IsAdmin() && !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, true, false, false, false) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
			return
		}
		sizeBytes, err := s.evaluator.GetAdmin().DatabaseSizeBytes(dbName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		const mb = int64(1024 * 1024)
		sizeMb := (sizeBytes + mb - 1) / mb // round up to whole MB
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"dbName":    dbName,
			"sizeBytes": sizeBytes,
			"sizeMb":    sizeMb,
		})
		return
	}

	// Check database permission for non-admin users
	user, _ := auth.UserFromContext(r.Context())
	if user != nil && !user.IsAdmin() {
		switch r.Method {
		case http.MethodGet:
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, name, true, false, false, false) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + name})
				return
			}
		case http.MethodDelete:
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, name, false, false, false, true) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + name})
				return
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		db, exists := s.evaluator.GetAdmin().GetDatabase(name)
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "database not found"})
			return
		}
		writeJSON(w, http.StatusOK, db)
	case http.MethodDelete:
		if err := s.evaluator.GetAdmin().DropDatabase(name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		username := "unknown"
		if user != nil {
			username = user.Username
		}
		s.RecordLog("info", fmt.Sprintf("Database '%s' dropped by user '%s'", name, username))
		writeJSON(w, http.StatusOK, map[string]string{"status": "dropped"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleTables handles table list requests
func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	dbName := r.URL.Query().Get("db")
	if dbName == "" {
		dbName = "test"
	}

	tables, err := s.evaluator.GetAdmin().ListTables(dbName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tables)
}

// handleTable handles single table requests
func (s *Server) handleTable(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/tables/"):]
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "table name required"})
		return
	}

	dbName := r.URL.Query().Get("db")
	if dbName == "" {
		dbName = "test"
	}

	// Check if this is a /api/tables/{db}/{table}/docs or /api/tables/{db}/{table}/indexes request
	parts := splitPath(path)
	if len(parts) == 3 && (parts[2] == "docs" || parts[2] == "indexes") {
		actualDB := parts[0]
		tableName := parts[1]
		subResource := parts[2]

		table, err := s.evaluator.GetAdmin().GetTable(actualDB, tableName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}

		if subResource == "docs" {
			docs := make([]interface{}, 0)
			for _, doc := range table.AllDocs() {
				converted, err := datumToInterface(doc)
				if err == nil {
					docs = append(docs, converted)
				}
			}
			writeJSON(w, http.StatusOK, docs)
			return
		}

		if subResource == "indexes" {
			writeJSON(w, http.StatusOK, table.IndexNames())
			return
		}
	}

	// Single-segment path: /api/tables/{name}?db={db}
	tableName := parts[0]

	switch r.Method {
	case http.MethodGet:
		table, err := s.evaluator.GetAdmin().GetTable(dbName, tableName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		resp := map[string]interface{}{
			"Name":      table.Name,
			"doc_count": table.DocCount(),
			"indexes":   table.IndexNames(),
			"shards":    1,
			"replicas":  1,
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if err := s.evaluator.GetAdmin().CreateTable(dbName, tableName); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	case http.MethodDelete:
		if err := s.evaluator.GetAdmin().DropTable(dbName, tableName); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "dropped"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// splitPath splits a URL path into segments, ignoring empty segments
func splitPath(path string) []string {
	var parts []string
	for _, p := range splitString(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// splitString splits a string by a separator character
func splitString(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// datumToInterface converts a datum.Datum to interface{} for JSON serialization
func datumToInterface(d datum.Datum) (interface{}, error) {
	switch d.Type() {
	case datum.Null:
		return nil, nil
	case datum.Bool:
		return d.Bool(), nil
	case datum.Num:
		return d.Num(), nil
	case datum.Str:
		return d.Str(), nil
	case datum.Array:
		arr := d.Array()
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			converted, err := datumToInterface(item)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case datum.Object:
		obj := d.Object()
		result := make(map[string]interface{})
		for _, key := range obj.Keys() {
			val, _ := obj.Get(key)
			converted, err := datumToInterface(val)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	case datum.Binary:
		return d.Binary(), nil
	case datum.Time:
		return d.Time(), nil
	case datum.Geometry:
		return d.Geometry(), nil
	default:
		return nil, nil
	}
}

// handleQuery handles query execution requests
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Query interface{} `json:"query"`
		DB    string      `json:"db"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Record query for metrics
	s.RecordQuery()

	// Resolve the default database for this request and enforce per-database
	// permissions using the authenticated user (from the JWT context). The
	// evaluator no longer keeps a global current DB, so concurrent requests are
	// isolated from each other. Admins bypass authorization.
	defaultDB := req.DB
	if defaultDB == "" {
		defaultDB = "widgettrace" // Data Explorer default
	}
	ctx := r.Context()
	if user, _ := auth.UserFromContext(ctx); user != nil && !user.IsAdmin() {
		store := s.auth.GetStore()
		uid := user.ID
		ctx = reql.WithSession(ctx, defaultDB, func(db string, read, write, create, drop bool) error {
			if store.HasDatabaseAccess(uid, db, read, write, create, drop) {
				return nil
			}
			return fmt.Errorf("access denied: no permission on database %q", db)
		})
	} else {
		ctx = reql.WithSession(ctx, defaultDB, nil)
	}

	// If query is a string, parse it as a ReQL expression
	queryArg := req.Query
	if queryStr, ok := queryArg.(string); ok {
		parsed, err := reql.ParseReQL(queryStr)
		if err != nil {
			slog.Error("query parse failed", "error", err)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "error",
				"result": nil,
				"error":  fmt.Sprintf("parse error: %v", err),
			})
			return
		}
		queryArg = parsed
	}

	// Execute query using the ReQL evaluator
	result, err := s.evaluator.Evaluate(ctx, queryArg)
	if err != nil {
		slog.Error("query execution failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "error",
			"result": nil,
			"error":  err.Error(),
		})
		return
	}

	// Convert datum result to interface{}
	responseData, _ := datumToInterface(result)

	// If result is a table reference, fetch all documents
	if respMap, ok := responseData.(map[string]interface{}); ok {
		if reqlType, ok := respMap["$reql_type$"].(string); ok && reqlType == "TABLE" {
			tableName, _ := respMap["table"].(string)
			dbName, _ := respMap["db"].(string)
			if dbName == "" {
				dbName = "test"
			}
			table, err := s.evaluator.GetAdmin().GetTable(dbName, tableName)
			if err == nil {
				docs := make([]interface{}, 0)
				for _, doc := range table.AllDocs() {
					converted, err := datumToInterface(doc)
					if err == nil {
						docs = append(docs, converted)
					}
				}
				responseData = docs
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"result": responseData,
	})
}

// handleServerInfo handles server info requests
func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.metrics.startTime).Round(time.Second).String()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version": "0.1.0",
		"server":  "gothinkdb",
		"status":  "running",
		"uptime":  uptime,
	})
}

// handleServerStats handles server stats requests
func (s *Server) handleServerStats(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queries_total":      s.metrics.queriesTotal.Load(),
		"queries_per_second": s.metrics.queriesPerSec.Load(),
		"connections":        s.getConnectionCount(),
		"memory_used":        memStats.Alloc / 1024 / 1024,
	})
}

// handleLogs handles log requests
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	s.metrics.logMu.RLock()
	logs := make([]LogEntry, len(s.metrics.logEntries))
	copy(logs, s.metrics.logEntries)
	s.metrics.logMu.RUnlock()
	writeJSON(w, http.StatusOK, logs)
}

// handleUserRouter routes user-related requests
func (s *Server) handleUserRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	parts := strings.Split(path, "/")

	// Check if this is a permissions request
	if len(parts) >= 2 && parts[1] == "permissions" {
		if len(parts) >= 3 && parts[2] != "" {
			// /api/users/{id}/permissions/{database}
			s.handleUserPermission(w, r)
		} else {
			// /api/users/{id}/permissions
			s.handleUserPermissions(w, r)
		}
	} else {
		// /api/users/{id}
		s.handleUser(w, r)
	}
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
