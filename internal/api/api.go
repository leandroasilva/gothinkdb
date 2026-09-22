package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/internal/rpc"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// Server represents the API server
type Server struct {
	evaluator *reql.Evaluator
	cluster   *rpc.ClusterManager
	wsHub     *WebSocketHub
	mux       *http.ServeMux
	mu        sync.RWMutex
}

// NewServer creates a new API server
func NewServer(evaluator *reql.Evaluator, cluster *rpc.ClusterManager) *Server {
	hub := NewWebSocketHub()
	go hub.Run()

	s := &Server{
		evaluator: evaluator,
		cluster:   cluster,
		wsHub:     hub,
		mux:       http.NewServeMux(),
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

	// Logs
	s.mux.HandleFunc("/api/logs", s.handleLogs)
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
		// /api/tables/{db}/{table}/{docs|indexes}
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

	// Single-segment or two-segment path: /api/tables/{name}?db={db}
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

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
