package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/leandroasilva/gothinkdb/internal/auth"
)

// handleUserPermissions handles user permissions (admin only)
func (s *Server) handleUserPermissions(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Extract user ID from path: /api/users/{id}/permissions
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "permissions" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	userID := parts[0]

	// Check if user exists
	if _, exists := s.auth.GetStore().GetUser(userID); !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		permissions := s.auth.GetStore().GetUserPermissions(userID)
		permList := make([]map[string]interface{}, 0, len(permissions))
		for _, p := range permissions {
			permList = append(permList, map[string]interface{}{
				"id":         p.ID,
				"database":   p.Database,
				"can_read":   p.CanRead,
				"can_write":  p.CanWrite,
				"can_create": p.CanCreate,
				"can_drop":   p.CanDrop,
			})
		}
		writeJSON(w, http.StatusOK, permList)

	case http.MethodPost:
		var req struct {
			Database  string `json:"database"`
			CanRead   bool   `json:"can_read"`
			CanWrite  bool   `json:"can_write"`
			CanCreate bool   `json:"can_create"`
			CanDrop   bool   `json:"can_drop"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if req.Database == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "database is required"})
			return
		}

		perm, err := s.auth.GetStore().GrantPermission(userID, req.Database, req.CanRead, req.CanWrite, req.CanCreate, req.CanDrop)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id":         perm.ID,
			"database":   perm.Database,
			"can_read":   perm.CanRead,
			"can_write":  perm.CanWrite,
			"can_create": perm.CanCreate,
			"can_drop":   perm.CanDrop,
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleUserPermission handles single permission operations (admin only)
func (s *Server) handleUserPermission(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Extract user ID and database from path: /api/users/{id}/permissions/{database}
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "permissions" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	userID := parts[0]
	database := parts[3]

	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if err := s.auth.GetStore().RevokePermission(userID, database); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
