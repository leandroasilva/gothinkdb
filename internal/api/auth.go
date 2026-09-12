package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/leandroasilva/gothinkdb/internal/auth"
)

// handleLogin handles user login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	token, user, err := s.auth.Login(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user.ToSafeUser(),
	})
}

// handleMe returns current user info
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	user, err := auth.RequireUser(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Get user permissions
	permissions := s.auth.GetStore().GetUserPermissions(user.ID)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":        user.ToSafeUser(),
		"permissions": permList,
	})
}

// extractTokenFromHeader extracts JWT token from Authorization header
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}
