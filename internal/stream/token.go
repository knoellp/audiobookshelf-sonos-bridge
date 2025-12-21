package stream

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// TokenPayload contains the data encoded in a stream token.
type TokenPayload struct {
	ItemID    string    `json:"item_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// TokenGenerator creates and validates HMAC-signed stream tokens.
type TokenGenerator struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenGenerator creates a new token generator.
func NewTokenGenerator(secret string, ttl time.Duration) *TokenGenerator {
	return &TokenGenerator{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// Generate creates a new signed token for streaming.
func (g *TokenGenerator) Generate(itemID, userID, sessionID string) (string, error) {
	payload := TokenPayload{
		ItemID:    itemID,
		UserID:    userID,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(g.ttl),
	}

	// Encode payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HMAC signature
	sig := g.sign(payloadBytes)

	// Combine payload and signature
	token := Token{
		Payload:   payloadBytes,
		Signature: sig,
	}

	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	// Base64 URL encode for use in URLs
	return base64.URLEncoding.EncodeToString(tokenBytes), nil
}

// Token is the wire format for stream tokens.
type Token struct {
	Payload   []byte `json:"p"`
	Signature []byte `json:"s"`
}

// Validate verifies a token and returns its payload.
func (g *TokenGenerator) Validate(tokenStr string) (*TokenPayload, error) {
	// Decode base64
	tokenBytes, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, errors.New("invalid token encoding")
	}

	// Parse token structure
	var token Token
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		return nil, errors.New("invalid token format")
	}

	// Verify signature
	expectedSig := g.sign(token.Payload)
	if !hmac.Equal(token.Signature, expectedSig) {
		return nil, errors.New("invalid token signature")
	}

	// Parse payload
	var payload TokenPayload
	if err := json.Unmarshal(token.Payload, &payload); err != nil {
		return nil, errors.New("invalid payload format")
	}

	// Check expiration
	if time.Now().After(payload.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	return &payload, nil
}

// sign creates an HMAC-SHA256 signature.
func (g *TokenGenerator) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write(data)
	return mac.Sum(nil)
}
