package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/internal/rpc"
)

// Server represents the API server
type Server struct {
	evaluator *reql.Evaluator
	cluster   *rpc.ClusterManager
	mux       *http.ServeMux
	mu        sync.RWMutex
}

// NewServer creates a new API server
func NewServer(evaluator *reql.Evaluator, cluster *rpc.ClusterManager) *Server {
	s := &Server{
		evaluator: evaluator,
		cluster:   cluster,
		mux:       http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes sets up API routes
func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("/api/health", s.handleHealth)

	// Cluster
	s.mux.HandleFunc("/api/cluster/status", s.handleClusterStatus)
	s.mux.HandleFunc("/api/cluster/members", s.handleClusterMembers)

	// Databases
	s.mux.HandleFunc("/api/databases", s.handleDatabases)
	s.mux.HandleFunc("/api/databases/", s.handleDatabase)

	// Tables
	s.mux.HandleFunc("/api/tables", s.handleTables)
	s.mux.HandleFunc("/api/tables/", s.handleTable)

	// Queries
	s.mux.HandleFunc("/api/query", s.handleQuery)

	// Server info
	s.mux.HandleFunc("/api/server/info", s.handleServerInfo)
	s.mux.HandleFunc("/api/server/stats", s.handleServerStats)
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
		writeJSON(w, http.StatusOK, databases)
	case http.MethodPost:
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

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
