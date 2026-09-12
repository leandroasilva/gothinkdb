package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/auth"
	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/internal/rpc"
)

// Server represents the API server
type Server struct {
	evaluator      *reql.Evaluator
	cluster        *rpc.ClusterManager
	clusterManager *ClusterManagerWrapper
	wsHub          *WebSocketHub
	auth           *auth.Service
	mux            *http.ServeMux
	mu             sync.RWMutex
}

// NewServer creates a new API server
func NewServer(evaluator *reql.Evaluator, cluster *rpc.ClusterManager, authService *auth.Service, clusterManager *ClusterManagerWrapper) *Server {
	hub := NewWebSocketHub()
	go hub.Run()

	s := &Server{
		evaluator:      evaluator,
		cluster:        cluster,
		clusterManager: clusterManager,
		wsHub:          hub,
		auth:           authService,
		mux:            http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// GetWebSocketHub returns the WebSocket hub
func (s *Server) GetWebSocketHub() *WebSocketHub {
	return s.wsHub
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
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	slog.Debug("health check request")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"server": "gothinkdb",
	})
}

// handleClusterStatus handles cluster status requests
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

	// Check database permission for non-admin users
	user, _ := auth.UserFromContext(r.Context())
	if user != nil && !user.IsAdmin() {
		if !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, true, false, false, false) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
			return
		}
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
	name := r.URL.Path[len("/api/tables/"):]
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "table name required"})
		return
	}

	dbName := r.URL.Query().Get("db")
	if dbName == "" {
		dbName = "test"
	}

	// Check database permission for non-admin users
	user, _ := auth.UserFromContext(r.Context())
	if user != nil && !user.IsAdmin() {
		switch r.Method {
		case http.MethodGet:
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, true, false, false, false) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
				return
			}
		case http.MethodPost:
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, false, false, true, false) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
				return
			}
		case http.MethodDelete:
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, false, false, false, true) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
				return
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		table, err := s.evaluator.GetAdmin().GetTable(dbName, name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, table)
	case http.MethodPost:
		if err := s.evaluator.GetAdmin().CreateTable(dbName, name); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	case http.MethodDelete:
		if err := s.evaluator.GetAdmin().DropTable(dbName, name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "dropped"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Execute query
	// This would integrate with the ReQL evaluator
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"result": nil,
	})
}

// handleServerInfo handles server info requests
func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version": "1.0.0",
		"server":  "gothinkdb",
		"status":  "running",
		"uptime":  "N/A",
	})
}

// handleServerStats handles server stats requests
func (s *Server) handleServerStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queries_total":      0,
		"queries_per_second": 0,
		"connections":        0,
		"memory_used":        0,
	})
}

// handleLogs handles log requests
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// Return sample logs for now; will be connected to real log system
	logs := []map[string]interface{}{
		{
			"id":        "1",
			"timestamp": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			"level":     "info",
			"message":   "GoThinkDB server started successfully",
			"server":    "gothinkdb",
		},
		{
			"id":        "2",
			"timestamp": time.Now().Add(-4 * time.Minute).Format(time.RFC3339),
			"level":     "info",
			"message":   "HTTP admin server listening on :8080",
			"server":    "gothinkdb",
		},
		{
			"id":        "3",
			"timestamp": time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
			"level":     "info",
			"message":   "ReQL protocol server listening on :28015",
			"server":    "gothinkdb",
		},
		{
			"id":        "4",
			"timestamp": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			"level":     "debug",
			"message":   "Health check endpoint responding",
			"server":    "gothinkdb",
		},
		{
			"id":        "5",
			"timestamp": time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			"level":     "info",
			"message":   "Dashboard served at /",
			"server":    "gothinkdb",
		},
	}
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
