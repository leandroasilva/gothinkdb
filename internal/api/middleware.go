package api

import (
	"net/http"
	"strings"

	"github.com/leandroasilva/gothinkdb/internal/auth"
)

// authMiddleware validates JWT token and injects user into context
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractTokenFromHeader(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authorization token required"})
			return
		}

		user, err := s.auth.GetUserFromToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		// Inject user into context
		ctx := auth.ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// adminMiddleware requires admin role
func (s *Server) adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.RequireUser(r.Context())
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}

		if !user.IsAdmin() {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
			return
		}

		next.ServeHTTP(w, r)
	}
}

// databasePermissionMiddleware checks database access permission
func (s *Server) databasePermissionMiddleware(requiredRead, requiredWrite, requiredCreate, requiredDrop bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.RequireUser(r.Context())
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
				return
			}

			// Admin has full access
			if user.IsAdmin() {
				next.ServeHTTP(w, r)
				return
			}

			// Extract database name from query or path
			dbName := r.URL.Query().Get("db")
			if dbName == "" {
				// Try to extract from path for routes like /api/databases/{name}
				path := r.URL.Path
				if strings.HasPrefix(path, "/api/databases/") {
					parts := strings.Split(strings.TrimPrefix(path, "/api/databases/"), "/")
					if len(parts) > 0 && parts[0] != "" {
						dbName = parts[0]
					}
				} else if strings.HasPrefix(path, "/api/tables/") {
					// For tables, we need to check the database from query param
					dbName = r.URL.Query().Get("db")
					if dbName == "" {
						dbName = "test" // default database
					}
				}
			}

			if dbName == "" {
				// If no database specified, allow (will use default)
				next.ServeHTTP(w, r)
				return
			}

			// Check permission
			if !s.auth.GetStore().HasDatabaseAccess(user.ID, dbName, requiredRead, requiredWrite, requiredCreate, requiredDrop) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions for database: " + dbName})
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

// filterDatabasesByPermission filters database list based on user permissions
func (s *Server) filterDatabasesByPermission(r *http.Request, databases []string) []string {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		return nil
	}

	// Admin sees all databases
	if user.IsAdmin() {
		return databases
	}

	// Get accessible databases
	accessible := s.auth.GetStore().GetAccessibleDatabases(user.ID)
	if accessible == nil {
		// nil means all databases (admin)
		return databases
	}

	// Filter databases
	accessibleMap := make(map[string]bool)
	for _, db := range accessible {
		accessibleMap[db] = true
	}

	filtered := make([]string, 0)
	for _, db := range databases {
		if accessibleMap[db] {
			filtered = append(filtered, db)
		}
	}

	return filtered
}
