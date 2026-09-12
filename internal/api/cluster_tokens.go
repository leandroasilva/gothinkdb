package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/auth"
	"github.com/leandroasilva/gothinkdb/internal/cluster"
)

// handleClusterTokens handles token generation and listing (admin only)
func (s *Server) handleClusterTokens(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all valid tokens
		tokens := s.clusterManager.GetTokenManager().ListTokens()
		writeJSON(w, http.StatusOK, tokens)

	case http.MethodPost:
		// Generate a new token
		var req struct {
			ExpiresIn string `json:"expires_in"` // e.g., "24h", "7d"
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		// Parse expiration duration
		expiresIn := 24 * time.Hour // Default 24h
		if req.ExpiresIn != "" {
			d, err := time.ParseDuration(req.ExpiresIn)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expires_in format"})
				return
			}
			expiresIn = d
		}

		// Get current user for created_by
		user, _ := auth.UserFromContext(r.Context())
		createdBy := "unknown"
		if user != nil {
			createdBy = user.Username
		}

		token, err := s.clusterManager.GetTokenManager().GenerateToken(createdBy, expiresIn)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, token)

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleClusterToken handles single token operations (admin only)
func (s *Server) handleClusterToken(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Extract token ID from path: /api/cluster/tokens/{id}
	tokenID := r.URL.Path[len("/api/cluster/tokens/"):]
	if tokenID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token ID required"})
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Revoke token
		if err := s.clusterManager.GetTokenManager().RevokeToken(tokenID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleClusterJoin handles node join requests
func (s *Server) handleClusterJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req cluster.JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate request
	if req.Token == "" || req.NodeID == "" || req.Address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token, node_id, and address are required"})
		return
	}

	// Set default ports if not provided
	if req.ClusterPort == 0 {
		req.ClusterPort = 29015
	}
	if req.HTTPPort == 0 {
		req.HTTPPort = 8080
	}
	if req.DriverPort == 0 {
		req.DriverPort = 28015
	}

	// Handle join request
	resp, err := s.clusterManager.GetJoinManager().HandleJoinRequest(&req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if !resp.Success {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
