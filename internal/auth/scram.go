package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// SCRAM-SHA-256 (RFC 5802) support for the ReQL driver handshake.
//
// The RethinkDB V1.0 protocol authenticates clients with SCRAM-SHA-256. We
// store, per user, a random salt plus the derived StoredKey/ServerKey so the
// plaintext password is never needed to verify a handshake (and never leaves
// the server). The HTTP admin login keeps using bcrypt (see user.go).

const (
	// SCRAMIterations is the PBKDF2 iteration count used for SCRAM-SHA-256.
	SCRAMIterations = 4096
	scramSaltLen    = 16
	scramNonceLen   = 18
)

// GenerateSalt returns a new random salt, base64-encoded.
func GenerateSalt() string {
	b := make([]byte, scramSaltLen)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is fatal for secure auth; fall back to a fixed
		// length zero-filled salt only so the server still boots in odd envs.
		return base64.StdEncoding.EncodeToString(make([]byte, scramSaltLen))
	}
	return base64.StdEncoding.EncodeToString(b)
}

func generateNonce() string {
	b := make([]byte, scramNonceLen)
	if _, err := rand.Read(b); err != nil {
		return base64.StdEncoding.EncodeToString(make([]byte, scramNonceLen))
	}
	return base64.StdEncoding.EncodeToString(b)
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// ComputeSCRAM derives the base64-encoded StoredKey and ServerKey for a
// password given a base64 salt and iteration count.
func ComputeSCRAM(password, saltB64 string, iterations int) (storedKey, serverKey string) {
	if iterations <= 0 {
		iterations = SCRAMIterations
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		salt = []byte(saltB64)
	}
	salted := pbkdf2.Key([]byte(password), salt, iterations, sha256.Size, sha256.New)
	clientKey := hmacSHA256(salted, []byte("Client Key"))
	stored := sha256.Sum256(clientKey)
	serverK := hmacSHA256(salted, []byte("Server Key"))
	return base64.StdEncoding.EncodeToString(stored[:]), base64.StdEncoding.EncodeToString(serverK)
}

// SCRAMServer holds the server-side state of a single SCRAM-SHA-256 exchange.
type SCRAMServer struct {
	user            *User
	clientFirstBare string
	serverFirst     string
	combinedNonce   string
	storedKey       []byte
	serverKey       []byte
}

// ParseClientFirst parses a client-first message of the form
// "n,,n=<user>,r=<client-nonce>" and returns the username, the client nonce and
// the "bare" part (without the GS2 header) used to build the auth message.
func ParseClientFirst(msg string) (username, clientNonce, clientFirstBare string, err error) {
	idx := strings.Index(msg, ",,")
	if idx < 0 {
		return "", "", "", fmt.Errorf("invalid client-first message: missing GS2 header")
	}
	clientFirstBare = msg[idx+2:]
	for _, p := range strings.Split(clientFirstBare, ",") {
		switch {
		case strings.HasPrefix(p, "n="):
			username = p[2:]
		case strings.HasPrefix(p, "r="):
			clientNonce = p[2:]
		}
	}
	if username == "" || clientNonce == "" {
		return "", "", "", fmt.Errorf("invalid client-first message: missing username or nonce")
	}
	return username, clientNonce, clientFirstBare, nil
}

// NewSCRAMServer prepares the server-first message for an existing user.
func NewSCRAMServer(user *User, clientFirstBare, clientNonce string) (*SCRAMServer, error) {
	if user == nil || user.Salt == "" || user.StoredKey == "" || user.ServerKey == "" {
		return nil, fmt.Errorf("user has no SCRAM credentials")
	}
	storedKey, err := base64.StdEncoding.DecodeString(user.StoredKey)
	if err != nil {
		return nil, fmt.Errorf("invalid stored key: %w", err)
	}
	serverKey, err := base64.StdEncoding.DecodeString(user.ServerKey)
	if err != nil {
		return nil, fmt.Errorf("invalid server key: %w", err)
	}
	iterations := user.Iterations
	if iterations <= 0 {
		iterations = SCRAMIterations
	}
	combinedNonce := clientNonce + generateNonce()
	serverFirst := fmt.Sprintf("r=%s,s=%s,i=%d", combinedNonce, user.Salt, iterations)
	return &SCRAMServer{
		user:            user,
		clientFirstBare: clientFirstBare,
		serverFirst:     serverFirst,
		combinedNonce:   combinedNonce,
		storedKey:       storedKey,
		serverKey:       serverKey,
	}, nil
}

// ServerFirst returns the server-first message to send to the client.
func (s *SCRAMServer) ServerFirst() string { return s.serverFirst }

// VerifyClientFinal validates the client-final message
// ("c=biws,r=<nonce>,p=<proof>") and returns the server-final message ("v=...").
func (s *SCRAMServer) VerifyClientFinal(clientFinal string) (string, error) {
	var nonce, proof string
	for _, p := range strings.Split(clientFinal, ",") {
		switch {
		case strings.HasPrefix(p, "r="):
			nonce = p[2:]
		case strings.HasPrefix(p, "p="):
			proof = p[2:]
		}
	}
	if nonce != s.combinedNonce {
		return "", fmt.Errorf("nonce mismatch")
	}
	if proof == "" {
		return "", fmt.Errorf("missing client proof")
	}
	clientProof, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		return "", fmt.Errorf("invalid proof encoding: %w", err)
	}
	cut := strings.Index(clientFinal, ",p=")
	if cut < 0 {
		return "", fmt.Errorf("invalid client-final message")
	}
	clientFinalWithoutProof := clientFinal[:cut]
	authMessage := s.clientFirstBare + "," + s.serverFirst + "," + clientFinalWithoutProof

	clientSignature := hmacSHA256(s.storedKey, []byte(authMessage))
	if len(clientProof) != len(clientSignature) {
		return "", fmt.Errorf("invalid credentials")
	}
	recoveredClientKey := make([]byte, len(clientProof))
	for i := range clientProof {
		recoveredClientKey[i] = clientProof[i] ^ clientSignature[i]
	}
	hashed := sha256.Sum256(recoveredClientKey)
	if !hmac.Equal(hashed[:], s.storedKey) {
		return "", fmt.Errorf("invalid credentials")
	}

	serverSignature := hmacSHA256(s.serverKey, []byte(authMessage))
	return "v=" + base64.StdEncoding.EncodeToString(serverSignature), nil
}
