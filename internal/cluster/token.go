package cluster

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JoinToken represents a token for joining the cluster
type JoinToken struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	ClusterID string     `json:"cluster_id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedBy    string     `json:"used_by,omitempty"` // NodeID that used this token
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedBy string     `json:"created_by"` // UserID who created this token
}

// TokenManager manages join tokens
type TokenManager struct {
	tokens    map[string]*JoinToken // token string -> JoinToken
	tokensByID map[string]*JoinToken // token ID -> JoinToken
	clusterID string
	secret    []byte
	mu        sync.RWMutex
}

// NewTokenManager creates a new token manager
func NewTokenManager(clusterID string, secret []byte) *TokenManager {
	return &TokenManager{
		tokens:     make(map[string]*JoinToken),
		tokensByID: make(map[string]*JoinToken),
		clusterID:  clusterID,
		secret:     secret,
	}
}

// GenerateToken generates a new join token
func (tm *TokenManager) GenerateToken(createdBy string, expiresIn time.Duration) (*JoinToken, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Generate unique ID and token
	id := uuid.New().String()
	
	// Create token payload
	payload := fmt.Sprintf("%s:%s:%d", id, tm.clusterID, time.Now().UnixNano())
	
	// Sign the payload
	mac := hmac.New(sha256.New, tm.secret)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	
	// Final token is payload + signature
	token := fmt.Sprintf("%s.%s", payload, signature)

	expiresAt := time.Now().Add(expiresIn)
	if expiresIn == 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default 24h
	}

	joinToken := &JoinToken{
		ID:        id,
		Token:     token,
		ClusterID: tm.clusterID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	}

	tm.tokens[token] = joinToken
	tm.tokensByID[id] = joinToken

	return joinToken, nil
}

// ValidateToken validates a join token and returns the token info
func (tm *TokenManager) ValidateToken(tokenString string) (*JoinToken, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	token, exists := tm.tokens[tokenString]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}

	// Check if token is expired
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Check if token was already used
	if token.UsedBy != "" {
		return nil, fmt.Errorf("token already used")
	}

	// Verify signature
	parts := splitToken(tokenString)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	payload := parts[0]
	signature := parts[1]

	mac := hmac.New(sha256.New, tm.secret)
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if signature != expectedSignature {
		return nil, fmt.Errorf("invalid token signature")
	}

	return token, nil
}

// MarkTokenUsed marks a token as used by a node
func (tm *TokenManager) MarkTokenUsed(tokenString, nodeID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token, exists := tm.tokens[tokenString]
	if !exists {
		return fmt.Errorf("token not found")
	}

	now := time.Now()
	token.UsedBy = nodeID
	token.UsedAt = &now

	return nil
}

// RevokeToken revokes a token
func (tm *TokenManager) RevokeToken(tokenID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token, exists := tm.tokensByID[tokenID]
	if !exists {
		return fmt.Errorf("token not found")
	}

	delete(tm.tokens, token.Token)
	delete(tm.tokensByID, tokenID)

	return nil
}

// ListTokens returns all non-expired, unused tokens
func (tm *TokenManager) ListTokens() []*JoinToken {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tokens := make([]*JoinToken, 0)
	now := time.Now()

	for _, token := range tm.tokens {
		// Skip expired tokens
		if now.After(token.ExpiresAt) {
			continue
		}
		// Skip used tokens
		if token.UsedBy != "" {
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens
}

// CleanupExpiredTokens removes expired tokens
func (tm *TokenManager) CleanupExpiredTokens() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for tokenStr, token := range tm.tokens {
		if now.After(token.ExpiresAt) {
			delete(tm.tokens, tokenStr)
			delete(tm.tokensByID, token.ID)
		}
	}
}

// splitToken splits a token into payload and signature
func splitToken(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}
