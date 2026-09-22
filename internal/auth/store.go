package auth

import (
	"fmt"
	"sync"
)

// Store manages users and permissions
type Store struct {
	users       map[string]*User       // userID -> User
	usersByName map[string]string      // username -> userID
	permissions map[string]*Permission // permissionKey -> Permission
	userPerms   map[string][]string    // userID -> []permissionKey
	mu          sync.RWMutex
}

// NewStore creates a new auth store
func NewStore() *Store {
	return &Store{
		users:       make(map[string]*User),
		usersByName: make(map[string]string),
		permissions: make(map[string]*Permission),
		userPerms:   make(map[string][]string),
	}
}

// CreateUser creates a new user
func (s *Store) CreateUser(username, password string, role Role) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if username already exists
	if _, exists := s.usersByName[username]; exists {
		return nil, fmt.Errorf("username %s already exists", username)
	}

	user, err := NewUser(username, password, role)
	if err != nil {
		return nil, err
	}

	s.users[user.ID] = user
	s.usersByName[username] = user.ID
	s.userPerms[user.ID] = []string{}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *Store) GetUser(userID string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	return user, exists
}

// GetUserByUsername retrieves a user by username
func (s *Store) GetUserByUsername(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.usersByName[username]
	if !exists {
		return nil, false
	}

	user, exists := s.users[userID]
	return user, exists
}

// ListUsers returns all users
func (s *Store) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

// DeleteUser deletes a user and their permissions
func (s *Store) DeleteUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Delete user permissions
	for _, permKey := range s.userPerms[userID] {
		delete(s.permissions, permKey)
	}
	delete(s.userPerms, userID)

	// Delete user
	delete(s.users, userID)
	delete(s.usersByName, user.Username)

	return nil
}

// GrantPermission grants a permission to a user
func (s *Store) GrantPermission(userID, database string, canRead, canWrite, canCreate, canDrop bool) (*Permission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user exists
	if _, exists := s.users[userID]; !exists {
		return nil, fmt.Errorf("user not found")
	}

	permKey := PermissionKey(userID, database)

	// Check if permission already exists
	if existing, exists := s.permissions[permKey]; exists {
		// Update existing permission
		existing.CanRead = canRead
		existing.CanWrite = canWrite
		existing.CanCreate = canCreate
		existing.CanDrop = canDrop
		return existing, nil
	}

	// Create new permission
	perm := NewPermission(userID, database, canRead, canWrite, canCreate, canDrop)
	s.permissions[permKey] = perm
	s.userPerms[userID] = append(s.userPerms[userID], permKey)

	return perm, nil
}

// RevokePermission revokes a permission from a user
func (s *Store) RevokePermission(userID, database string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	permKey := PermissionKey(userID, database)

	if _, exists := s.permissions[permKey]; !exists {
		return fmt.Errorf("permission not found")
	}

	delete(s.permissions, permKey)

	// Remove from user's permission list
	perms := s.userPerms[userID]
	for i, key := range perms {
		if key == permKey {
			s.userPerms[userID] = append(perms[:i], perms[i+1:]...)
			break
		}
	}

	return nil
}

// GetUserPermissions returns all permissions for a user
func (s *Store) GetUserPermissions(userID string) []*Permission {
	s.mu.RLock()
	defer s.mu.RUnlock()

	permKeys := s.userPerms[userID]
	perms := make([]*Permission, 0, len(permKeys))

	for _, key := range permKeys {
		if perm, exists := s.permissions[key]; exists {
			perms = append(perms, perm)
		}
	}

	return perms
}

// GetPermission returns a specific permission
func (s *Store) GetPermission(userID, database string) (*Permission, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	permKey := PermissionKey(userID, database)
	perm, exists := s.permissions[permKey]
	return perm, exists
}

// HasDatabaseAccess checks if user has access to a database
func (s *Store) HasDatabaseAccess(userID, database string, read, write, create, drop bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if user is admin
	if user, exists := s.users[userID]; exists && user.IsAdmin() {
		return true
	}

	permKey := PermissionKey(userID, database)
	perm, exists := s.permissions[permKey]
	if !exists {
		return false
	}

	return perm.HasAccess(read, write, create, drop)
}

// GetAccessibleDatabases returns databases the user has access to
func (s *Store) GetAccessibleDatabases(userID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Admin has access to all databases
	if user, exists := s.users[userID]; exists && user.IsAdmin() {
		return nil // nil means all databases
	}

	permKeys := s.userPerms[userID]
	databases := make([]string, 0, len(permKeys))

	for _, key := range permKeys {
		if perm, exists := s.permissions[key]; exists && perm.CanRead {
			databases = append(databases, perm.Database)
		}
	}

	return databases
}

// CreateDefaultAdmin creates the default admin user if it doesn't exist
func (s *Store) CreateDefaultAdmin() (*User, error) {
	s.mu.RLock()
	_, exists := s.usersByName["admin"]
	s.mu.RUnlock()

	if exists {
		return nil, nil // Admin already exists
	}

	return s.CreateUser("admin", "admin", RoleAdmin)
}
