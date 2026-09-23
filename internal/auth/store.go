package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages users and permissions.
//
// When path is empty the store is in-memory only (used by tests/dev). When a
// path is set (NewPersistentStore) every mutation is atomically written to
// <path> and reloaded on boot, so users and per-database permissions survive
// restarts.
type Store struct {
	users       map[string]*User       // userID -> User
	usersByName map[string]string      // username -> userID
	permissions map[string]*Permission // permissionKey -> Permission
	userPerms   map[string][]string    // userID -> []permissionKey
	path        string                 // persistence file (empty = in-memory only)
	mu          sync.RWMutex
}

// NewStore creates a new in-memory auth store
func NewStore() *Store {
	return &Store{
		users:       make(map[string]*User),
		usersByName: make(map[string]string),
		permissions: make(map[string]*Permission),
		userPerms:   make(map[string][]string),
	}
}

// NewPersistentStore creates an auth store backed by <dataDir>/auth.json. The
// file is created if missing and loaded otherwise. Writes are atomic
// (tmp+rename) so a crash never leaves a truncated store.
func NewPersistentStore(dataDir string) (*Store, error) {
	s := NewStore()
	if dataDir == "" {
		return s, nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	s.path = filepath.Join(dataDir, "auth.json")
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// persistedUser is the on-disk representation of a User (includes the bcrypt
// hash and SCRAM credentials, which are never exposed over the API).
type persistedUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Salt         string    `json:"salt"`
	Iterations   int       `json:"iterations"`
	StoredKey    string    `json:"stored_key"`
	ServerKey    string    `json:"server_key"`
}

type persistedStore struct {
	Users       []*persistedUser `json:"users"`
	Permissions []*Permission    `json:"permissions"`
}

// load reads the store from disk (no-op if the file does not exist yet).
func (s *Store) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read auth store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var ps persistedStore
	if err := json.Unmarshal(data, &ps); err != nil {
		return fmt.Errorf("failed to parse auth store: %w", err)
	}
	for _, pu := range ps.Users {
		u := &User{
			ID:           pu.ID,
			Username:     pu.Username,
			PasswordHash: pu.PasswordHash,
			Role:         pu.Role,
			CreatedAt:    pu.CreatedAt,
			UpdatedAt:    pu.UpdatedAt,
			Salt:         pu.Salt,
			Iterations:   pu.Iterations,
			StoredKey:    pu.StoredKey,
			ServerKey:    pu.ServerKey,
		}
		s.users[u.ID] = u
		s.usersByName[u.Username] = u.ID
		if _, ok := s.userPerms[u.ID]; !ok {
			s.userPerms[u.ID] = []string{}
		}
	}
	for _, p := range ps.Permissions {
		key := PermissionKey(p.UserID, p.Database)
		s.permissions[key] = p
		s.userPerms[p.UserID] = append(s.userPerms[p.UserID], key)
	}
	return nil
}

// persist atomically writes the store to disk. It must be called while holding
// the write lock (s.mu).
func (s *Store) persist() {
	if s.path == "" {
		return
	}
	ps := persistedStore{
		Users:       make([]*persistedUser, 0, len(s.users)),
		Permissions: make([]*Permission, 0, len(s.permissions)),
	}
	for _, u := range s.users {
		ps.Users = append(ps.Users, &persistedUser{
			ID:           u.ID,
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			Role:         u.Role,
			CreatedAt:    u.CreatedAt,
			UpdatedAt:    u.UpdatedAt,
			Salt:         u.Salt,
			Iterations:   u.Iterations,
			StoredKey:    u.StoredKey,
			ServerKey:    u.ServerKey,
		})
	}
	for _, p := range s.permissions {
		ps.Permissions = append(ps.Permissions, p)
	}
	data, err := json.MarshalIndent(&ps, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
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
	s.persist()

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
	delete(s.users, user.ID)
	delete(s.usersByName, user.Username)
	s.persist()

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
		existing.UpdatedAt = time.Now()
		s.persist()
		return existing, nil
	}

	// Create new permission
	perm := NewPermission(userID, database, canRead, canWrite, canCreate, canDrop)
	s.permissions[permKey] = perm
	s.userPerms[userID] = append(s.userPerms[userID], permKey)
	s.persist()

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
	s.persist()

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
	return s.EnsureAdmin("admin", "admin")
}

// EnsureAdmin creates an admin user with the given credentials if the username
// does not exist yet. Used at boot to bootstrap the admin from environment
// variables (GOTHINKDB_ADMIN_USER/PASSWORD). Returns (nil, nil) if it already
// exists so the password is never silently overwritten.
func (s *Store) EnsureAdmin(username, password string) (*User, error) {
	s.mu.RLock()
	_, exists := s.usersByName[username]
	s.mu.RUnlock()

	if exists {
		return nil, nil // Admin already exists
	}

	return s.CreateUser(username, password, RoleAdmin)
}

// UpdateUserPassword resets a user's password (bcrypt hash + SCRAM creds).
func (s *Store) UpdateUserPassword(userID, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	if err := user.SetPassword(password); err != nil {
		return err
	}
	s.persist()
	return nil
}
