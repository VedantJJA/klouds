// Package auth handles PASETO token generation and password hashing.
// We use a symmetric PASETO v4 approach for simplicity (single-server deployment).
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrInvalidCreds = errors.New("invalid credentials")
)

// TokenClaims holds the payload of a Klouds auth token.
type TokenClaims struct {
	UserID   string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	IssuedAt int64  `json:"iat"`
	ExpireAt int64  `json:"exp"`
}

// TokenService creates and validates auth tokens.
// Uses HMAC-SHA256 signed JSON tokens (lightweight alternative to full PASETO
// that avoids the heavy dependency while preserving the security model).
type TokenService struct {
	secretKey []byte
	duration  time.Duration
}

// NewTokenService creates a token service with the given secret and token duration.
func NewTokenService(secret string, duration time.Duration) *TokenService {
	// Derive a 32-byte key from the secret using SHA-256
	h := sha256.Sum256([]byte(secret))
	return &TokenService{
		secretKey: h[:],
		duration:  duration,
	}
}

// CreateToken generates a signed token for the given user.
func (ts *TokenService) CreateToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		IssuedAt: now.Unix(),
		ExpireAt: now.Add(ts.duration).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// HMAC-SHA256 sign: nonce + payload
	mac := hmac.New(sha256.New, ts.secretKey)
	mac.Write(nonce)
	mac.Write(payload)
	sig := mac.Sum(nil)

	// Token format: base64(nonce) . base64(payload) . base64(sig)
	token := fmt.Sprintf("%s.%s.%s",
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(payload),
		base64.RawURLEncoding.EncodeToString(sig),
	)

	return token, nil
}

// ValidateToken verifies and parses a token, returning claims if valid.
func (ts *TokenService) ValidateToken(token string) (*TokenClaims, error) {
	// Split into 3 parts
	parts := splitToken(token)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Verify HMAC
	mac := hmac.New(sha256.New, ts.secretKey)
	mac.Write(nonce)
	mac.Write(payload)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, ErrInvalidToken
	}

	// Parse claims
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	// Check expiry
	if time.Now().Unix() > claims.ExpireAt {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

func splitToken(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
