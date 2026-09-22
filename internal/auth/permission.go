package auth

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Permission represents user access to a specific database
type Permission struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Database  string    `json:"database"`
	CanRead   bool      `json:"can_read"`
	CanWrite  bool      `json:"can_write"`
	CanCreate bool      `json:"can_create"`
	CanDrop   bool      `json:"can_drop"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewPermission creates a new permission
func NewPermission(userID, database string, canRead, canWrite, canCreate, canDrop bool) *Permission {
	now := time.Now()
	return &Permission{
		ID:        uuid.New().String(),
		UserID:    userID,
		Database:  database,
		CanRead:   canRead,
		CanWrite:  canWrite,
		CanCreate: canCreate,
		CanDrop:   canDrop,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// HasAccess checks if permission grants the requested access
func (p *Permission) HasAccess(read, write, create, drop bool) bool {
	if read && !p.CanRead {
		return false
	}
	if write && !p.CanWrite {
		return false
	}
	if create && !p.CanCreate {
		return false
	}
	if drop && !p.CanDrop {
		return false
	}
	return true
}

// PermissionKey returns a unique key for user+database combination
func PermissionKey(userID, database string) string {
	return fmt.Sprintf("%s:%s", userID, database)
}
