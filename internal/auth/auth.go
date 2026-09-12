package auth

import (
	"context"
	"fmt"
)

// contextKey is a custom type for context keys
type contextKey string

const (
	userContextKey contextKey = "user"
)

// Service provides authentication and authorization operations
type Service struct {
	store *Store
}

// NewService creates a new auth service
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(username, password string) (string, *User, error) {
	user, exists := s.store.GetUserByUsername(username)
	if !exists {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	if !user.CheckPassword(password) {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	token, err := GenerateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, user, nil
}

// GetUserFromToken retrieves user from JWT token
func (s *Service) GetUserFromToken(tokenString string) (*User, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	user, exists := s.store.GetUser(claims.UserID)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// GetStore returns the underlying store
func (s *Service) GetStore() *Store {
	return s.store
}

// ContextWithUser adds user to context
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves user from context
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// RequireUser retrieves user from context or returns error
func RequireUser(ctx context.Context) (*User, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}
	return user, nil
}

// RequireAdmin retrieves admin user from context or returns error
func RequireAdmin(ctx context.Context) (*User, error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	if !user.IsAdmin() {
		return nil, fmt.Errorf("admin access required")
	}
	return user, nil
}
