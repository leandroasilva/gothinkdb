package auth

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role represents user role
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents a system user
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never expose password hash
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// SCRAM-SHA-256 credentials for the ReQL driver handshake. The plaintext
	// password is never stored; these derived values are persisted (see
	// persistedUser) so the driver port can authenticate without bcrypt.
	Salt      string `json:"-"`
	Iterations int   `json:"-"`
	StoredKey string `json:"-"`
	ServerKey string `json:"-"`
}

// NewUser creates a new user with hashed password
func NewUser(username, password string, role Role) (*User, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	u := &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	u.setSCRAM(password)
	return u, nil
}

// setSCRAM derives and stores the SCRAM-SHA-256 credentials for a password.
func (u *User) setSCRAM(password string) {
	u.Salt = GenerateSalt()
	u.Iterations = SCRAMIterations
	u.StoredKey, u.ServerKey = ComputeSCRAM(password, u.Salt, u.Iterations)
}

// SetPassword updates the password (bcrypt hash) and re-derives SCRAM creds.
func (u *User) SetPassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	u.PasswordHash = string(hash)
	u.setSCRAM(password)
	u.UpdatedAt = time.Now()
	return nil
}

// CheckPassword verifies if the provided password matches the hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// ToSafeUser returns user data without sensitive fields
func (u *User) ToSafeUser() map[string]interface{} {
	return map[string]interface{}{
		"id":         u.ID,
		"username":   u.Username,
		"role":       u.Role,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}
}
