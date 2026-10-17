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
	"strings"
	"sync"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

const (
	sessionCookieName = "bridge_session"
	sessionIDLength   = 32

	// sessionCookieMaxAge is the browser maximum (Chrome caps at 400 days).
	sessionCookieMaxAge = 400 * 24 * 60 * 60
	// lastUsedThrottle limits how often last_used_at is written.
	lastUsedThrottle = time.Hour
)

// ErrSessionRevoked is returned when automatic re-login failed because
// Audiobookshelf rejected the stored credentials; the session has been deleted.
var ErrSessionRevoked = errors.New("session revoked: stored credentials rejected")

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	absClient    *abs.Client
	sessionStore *store.SessionStore
	sessionKey   []byte // 32 bytes for AES-256

	secureCookies bool
	reauthLocks   sync.Map // session ID -> *sync.Mutex
	revoked       sync.Map // session ID -> struct{} (password rejected by ABS)
}

// SetSecureCookies controls the Secure flag of the session cookie.
func (h *AuthHandler) SetSecureCookies(secure bool) {
	h.secureCookies = secure
}

// setSessionCookie (re)sets the long-lived session cookie.
func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionCookieMaxAge,
	})
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

	// Encrypt the ABS token and password (password enables automatic re-login)
	encryptedToken, err := h.encryptToken(user.BestToken())
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	encryptedPassword, err := h.encryptToken(password)
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
		ID:             sessionID,
		ABSTokenEnc:    encryptedToken,
		ABSPasswordEnc: encryptedPassword,
		ABSUserID:      user.ID,
		ABSUsername:    user.Username,
		CreatedAt:      time.Now(),
		LastUsedAt:     time.Now(),
	}

	if err := h.sessionStore.Create(session); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.setSessionCookie(w, sessionID)

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
// The client re-logs in automatically (once) if ABS answers 401 and the session
// has stored credentials.
func (h *AuthHandler) GetABSClientForSession(session *store.Session) (*abs.Client, error) {
	token, err := h.decryptToken(session.ABSTokenEnc)
	if err != nil {
		return nil, err
	}
	return h.absClient.WithToken(token).WithReauth(h.reauthFunc(session)), nil
}

// ClientForSession is GetABSClientForSession under the name used by background workers.
func (h *AuthHandler) ClientForSession(session *store.Session) (*abs.Client, error) {
	return h.GetABSClientForSession(session)
}

// reauthFunc returns a function that logs in again with the stored credentials
// and persists the new token.
func (h *AuthHandler) reauthFunc(session *store.Session) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		lockIface, _ := h.reauthLocks.LoadOrStore(session.ID, &sync.Mutex{})
		mu := lockIface.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()

		// Another request may have refreshed the token while we waited.
		current, err := h.sessionStore.Get(session.ID)
		if err != nil {
			return "", err
		}
		if current == nil {
			return "", ErrSessionRevoked
		}
		if string(current.ABSTokenEnc) != string(session.ABSTokenEnc) {
			session.ABSTokenEnc = current.ABSTokenEnc
			return h.decryptToken(current.ABSTokenEnc)
		}

		if len(current.ABSPasswordEnc) == 0 {
			return "", abs.ErrUnauthorized
		}
		password, err := h.decryptToken(current.ABSPasswordEnc)
		if err != nil {
			return "", abs.ErrUnauthorized
		}

		user, err := h.absClient.Login(ctx, current.ABSUsername, password)
		if err != nil {
			if errors.Is(err, abs.ErrInvalidCredentials) {
				slog.Warn("stored credentials rejected by ABS, removing session", "session_id", session.ID)
				h.revoked.Store(session.ID, struct{}{})
				h.sessionStore.Delete(session.ID)
				return "", ErrSessionRevoked
			}
			return "", fmt.Errorf("re-login failed: %w", err)
		}
		newToken := user.BestToken()
		if newToken == "" {
			return "", abs.ErrUnauthorized
		}
		tokenEnc, err := h.encryptToken(newToken)
		if err != nil {
			return "", err
		}
		if err := h.sessionStore.UpdateCredentials(session.ID, tokenEnc, current.ABSPasswordEnc); err != nil {
			return "", err
		}
		session.ABSTokenEnc = tokenEnc
		slog.Info("re-authenticated session", "session_id", session.ID, "username", current.ABSUsername)
		return newToken, nil
	}
}

// isAPIRequest reports whether the client expects a status code rather than a redirect.
func isAPIRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") != "" ||
		r.Header.Get("Sec-Fetch-Mode") == "cors" ||
		strings.Contains(r.Header.Get("Accept"), "json")
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
			if isAPIRequest(r) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			target := "/login"
			if c, cerr := r.Cookie(sessionCookieName); cerr == nil && c.Value != "" {
				// A cookie existed but its session is gone.
				if _, wasRevoked := h.revoked.LoadAndDelete(c.Value); wasRevoked {
					target = "/login?error=password_changed"
				} else {
					target = "/login?error=session_expired"
				}
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}

		// Sliding expiration: refresh cookie, throttle DB writes.
		h.setSessionCookie(w, session.ID)
		if time.Since(session.LastUsedAt) > lastUsedThrottle {
			h.sessionStore.UpdateLastUsed(session.ID)
		}

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
