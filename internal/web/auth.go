package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

const (
	sessionCookieName = "bridge_session"
	sessionIDLength   = 32
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	absClient    *abs.Client
	sessionStore *store.SessionStore
	sessionKey   []byte // 32 bytes for AES-256
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(absClient *abs.Client, sessionStore *store.SessionStore, sessionSecret string) (*AuthHandler, error) {
	// Derive 32-byte key from secret
	key := deriveKey(sessionSecret)

	return &AuthHandler{
		absClient:    absClient,
		sessionStore: sessionStore,
		sessionKey:   key,
	}, nil
}

// deriveKey creates a 32-byte key from a secret string.
func deriveKey(secret string) []byte {
	// Simple key derivation: take first 32 bytes or pad with zeros
	key := make([]byte, 32)
	copy(key, []byte(secret))
	return key
}

// HandleLogin processes login form submissions.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		// Redirect back to login with error
		http.Redirect(w, r, "/login?error=missing_credentials", http.StatusSeeOther)
		return
	}

	// Login to Audiobookshelf
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, err := h.absClient.Login(ctx, username, password)
	if err != nil {
		if errors.Is(err, abs.ErrInvalidCredentials) {
			http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/login?error=server_error", http.StatusSeeOther)
		return
	}

	// Encrypt the ABS token
	encryptedToken, err := h.encryptToken(user.Token)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create session in database
	session := &store.Session{
		ID:          sessionID,
		ABSTokenEnc: encryptedToken,
		ABSUserID:   user.ID,
		ABSUsername: user.Username,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}

	if err := h.sessionStore.Create(session); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 365, // 1 year
	})

	// Redirect to libraries
	http.Redirect(w, r, "/libraries", http.StatusSeeOther)
}

// HandleLogout clears the session and redirects to login.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session ID from cookie
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		// Delete session from database
		h.sessionStore.Delete(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete immediately
	})

	// Redirect to login
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// GetSession retrieves and validates the current session.
// Returns nil if no valid session exists.
func (h *AuthHandler) GetSession(r *http.Request) (*store.Session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, nil
	}

	session, err := h.sessionStore.Get(cookie.Value)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetABSToken decrypts and returns the ABS token for a session.
func (h *AuthHandler) GetABSToken(session *store.Session) (string, error) {
	return h.decryptToken(session.ABSTokenEnc)
}

// GetABSClientForSession returns an ABS client configured with the session's token.
func (h *AuthHandler) GetABSClientForSession(session *store.Session) (*abs.Client, error) {
	token, err := h.decryptToken(session.ABSTokenEnc)
	if err != nil {
		return nil, err
	}
	return h.absClient.WithToken(token), nil
}

// RequireAuth is middleware that requires a valid session.
func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.GetSession(r)
		if err != nil {
			slog.Error("session lookup failed", "error", err, "path", r.URL.Path)
			// Treat session errors as "no session" - redirect to login
			session = nil
		}

		// Validate session token can be decrypted (catches mismatched session secrets)
		if session != nil {
			_, err := h.decryptToken(session.ABSTokenEnc)
			if err != nil {
				slog.Warn("session token invalid, clearing session", "error", err, "session_id", session.ID)
				// Delete the invalid session from database
				h.sessionStore.Delete(session.ID)
				// Clear the invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     sessionCookieName,
					Value:    "",
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   -1,
				})
				session = nil
			}
		}

		if session == nil {
			// Redirect to login for browser requests
			if r.Header.Get("Accept") == "application/json" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Update last used timestamp
		h.sessionStore.UpdateLastUsed(session.ID)

		// Add session to request context
		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SessionFromContext retrieves the session from the request context.
func SessionFromContext(ctx context.Context) *store.Session {
	session, _ := ctx.Value(sessionContextKey).(*store.Session)
	return session
}

// Context key for session
type contextKey string

const sessionContextKey contextKey = "session"

// encryptToken encrypts a token using AES-256-GCM.
func (h *AuthHandler) encryptToken(token string) ([]byte, error) {
	block, err := aes.NewCipher(h.sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return ciphertext, nil
}

// EncryptToken encrypts a token using AES-256-GCM.
// Exported for testing purposes.
func (h *AuthHandler) EncryptToken(token string) ([]byte, error) {
	return h.encryptToken(token)
}

// DecryptToken decrypts a token using AES-256-GCM.
// This method satisfies the TokenDecrypter interface.
func (h *AuthHandler) DecryptToken(encrypted []byte) (string, error) {
	return h.decryptToken(encrypted)
}

// decryptToken decrypts a token using AES-256-GCM.
func (h *AuthHandler) decryptToken(encrypted []byte) (string, error) {
	block, err := aes.NewCipher(h.sessionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(encrypted) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// generateSessionID creates a cryptographically random session ID.
func generateSessionID() (string, error) {
	bytes := make([]byte, sessionIDLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
