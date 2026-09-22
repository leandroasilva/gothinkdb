package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/leandroasilva/gothinkdb/internal/auth"
)

// handleUsers handles user list and creation (admin only)
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	admin, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	_ = admin // admin is authenticated

	switch r.Method {
	case http.MethodGet:
		users := s.auth.GetStore().ListUsers()
		userList := make([]map[string]interface{}, 0, len(users))
		for _, u := range users {
			userList = append(userList, u.ToSafeUser())
		}
		writeJSON(w, http.StatusOK, userList)

	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if req.Username == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
			return
		}

		role := auth.RoleUser
		if req.Role == "admin" {
			role = auth.RoleAdmin
		}

		user, err := s.auth.GetStore().CreateUser(req.Username, req.Password, role)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, user.ToSafeUser())

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleUser handles single user operations (admin only)
func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	// Require admin access
	_, err := auth.RequireAdmin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	// Extract user ID from path: /api/users/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	userID := strings.Split(path, "/")[0]

	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		user, exists := s.auth.GetStore().GetUser(userID)
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusOK, user.ToSafeUser())

	case http.MethodDelete:
		if err := s.auth.GetStore().DeleteUser(userID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
