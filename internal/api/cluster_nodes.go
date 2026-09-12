package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/leandroasilva/gothinkdb/internal/auth"
	"github.com/leandroasilva/gothinkdb/internal/cluster"
)

// handleClusterNodes handles cluster node operations
func (s *Server) handleClusterNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List cluster members
		members := s.clusterManager.GetJoinManager().GetMembers()
		writeJSON(w, http.StatusOK, members)

	case http.MethodPost:
		// Require admin access
		_, err := auth.RequireAdmin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}

		// Add a new node (alternative to join with token)
		var req struct {
			NodeID      string `json:"node_id"`
			Address     string `json:"address"`
			ClusterPort int    `json:"cluster_port"`
			HTTPPort    int    `json:"http_port"`
			DriverPort  int    `json:"driver_port"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if req.NodeID == "" || req.Address == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and address are required"})
			return
		}

		// Set default ports
		if req.ClusterPort == 0 {
			req.ClusterPort = 29015
		}
		if req.HTTPPort == 0 {
			req.HTTPPort = 8080
		}
		if req.DriverPort == 0 {
			req.DriverPort = 28015
		}

		// Add member
		if err := s.clusterManager.GetJoinManager().AddMember(
			req.NodeID, req.Address, req.ClusterPort, req.HTTPPort, req.DriverPort,
		); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleClusterNode handles single node operations
func (s *Server) handleClusterNode(w http.ResponseWriter, r *http.Request) {
	// Extract node ID and action from path: /api/cluster/nodes/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/cluster/nodes/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node ID required"})
		return
	}

	nodeID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "remove":
		s.handleRemoveNode(w, r, nodeID)
	case "status":
		s.handleNodeStatus(w, r, nodeID)
	default:
		// GET single node info
		if r.Method == http.MethodGet {
			member, exists := s.clusterManager.GetJoinManager().GetMember(nodeID)
			if !exists {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
				return
			}
			writeJSON(w, http.StatusOK, member)
		} else {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

// handleRemoveNode handles node removal
func (s *Server) handleRemoveNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Start removal process
	if err := s.clusterManager.GetRemoveManager().StartRemoval(nodeID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "removal_started",
		"node_id": nodeID,
	})
}

// handleNodeStatus handles node status requests
func (s *Server) handleNodeStatus(w http.ResponseWriter, r *http.Request, nodeID string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Check if there's a removal operation for this node
	op, err := s.clusterManager.GetRemoveManager().GetRemovalStatus(nodeID)
	if err == nil {
		// There's a removal operation
		writeJSON(w, http.StatusOK, op)
		return
	}

	// No removal operation, return node info
	member, exists := s.clusterManager.GetJoinManager().GetMember(nodeID)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": member.NodeID,
		"status":  member.Status,
		"address": member.Address,
	})
}

// handleClusterBalance handles cluster balance information
func (s *Server) handleClusterBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	balance := s.clusterManager.GetRebalancer().GetClusterBalance()
	writeJSON(w, http.StatusOK, balance)
}

// handleClusterTransfers handles transfer listing
func (s *Server) handleClusterTransfers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	transfers := s.clusterManager.GetTransferManager().ListTransfers()
	writeJSON(w, http.StatusOK, transfers)
}

// ClusterManagerWrapper wraps cluster managers for the API server
type ClusterManagerWrapper struct {
	joinManager     *cluster.JoinManager
	tokenManager    *cluster.TokenManager
	removeManager   *cluster.RemoveManager
	transferManager *cluster.TransferManager
	rebalancer      *cluster.Rebalancer
}

// NewClusterManagerWrapper creates a new cluster manager wrapper
func NewClusterManagerWrapper(clusterID string, secret []byte) *ClusterManagerWrapper {
	tokenManager := cluster.NewTokenManager(clusterID, secret)
	joinManager := cluster.NewJoinManager(tokenManager)
	transferManager := cluster.NewTransferManager()
	removeManager := cluster.NewRemoveManager(joinManager, transferManager)
	rebalancer := cluster.NewRebalancer(joinManager, transferManager)

	return &ClusterManagerWrapper{
		joinManager:     joinManager,
		tokenManager:    tokenManager,
		removeManager:   removeManager,
		transferManager: transferManager,
		rebalancer:      rebalancer,
	}
}

// GetJoinManager returns the join manager
func (cmw *ClusterManagerWrapper) GetJoinManager() *cluster.JoinManager {
	return cmw.joinManager
}

// GetTokenManager returns the token manager
func (cmw *ClusterManagerWrapper) GetTokenManager() *cluster.TokenManager {
	return cmw.tokenManager
}

// GetRemoveManager returns the remove manager
func (cmw *ClusterManagerWrapper) GetRemoveManager() *cluster.RemoveManager {
	return cmw.removeManager
}

// GetTransferManager returns the transfer manager
func (cmw *ClusterManagerWrapper) GetTransferManager() *cluster.TransferManager {
	return cmw.transferManager
}

// GetRebalancer returns the rebalancer
func (cmw *ClusterManagerWrapper) GetRebalancer() *cluster.Rebalancer {
	return cmw.rebalancer
}
