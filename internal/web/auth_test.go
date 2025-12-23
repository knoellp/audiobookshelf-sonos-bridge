package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

func setupAuthTest(t *testing.T) (*AuthHandler, *store.DB, *httptest.Server, func()) {
	t.Helper()

	// Create temp database
	f, err := os.CreateTemp("", "auth_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := f.Name()
	f.Close()

	db, err := store.New(dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("failed to create database: %v", err)
	}

	sessionStore := store.NewSessionStore(db)

	// Create mock ABS server
	absServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			var req abs.LoginRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Username == "testuser" && req.Password == "testpass" {
				resp := struct {
					User abs.User `json:"user"`
				}{
					User: abs.User{
						ID:       "user-123",
						Username: "testuser",
						Token:    "abs-token-xyz",
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}))

	absClient := abs.NewClient(absServer.URL)
	authHandler, _ := NewAuthHandler(absClient, sessionStore, "test-secret-key-at-least-32-chars!")

	cleanup := func() {
		absServer.Close()
		db.Close()
		os.Remove(dbPath)
	}

	return authHandler, db, absServer, cleanup
}

func TestHandleLogin_Success(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	// Create form data
	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", "testpass")

	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	authHandler.HandleLogin(rec, req)

	// Should redirect to /libraries
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "/libraries" {
		t.Errorf("expected redirect to /libraries, got %s", location)
	}

	// Should have set session cookie
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "bridge_session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Error("expected session cookie to be set")
	}
	if sessionCookie.HttpOnly != true {
		t.Error("session cookie should be HttpOnly")
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	form := url.Values{}
	form.Set("username", "wrong")
	form.Set("password", "wrong")

	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	authHandler.HandleLogin(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error=invalid_credentials") {
		t.Errorf("expected error in redirect, got %s", location)
	}
}

func TestHandleLogin_MissingCredentials(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	form := url.Values{}
	form.Set("username", "")
	form.Set("password", "")

	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	authHandler.HandleLogin(rec, req)

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error=missing_credentials") {
		t.Errorf("expected missing_credentials error, got %s", location)
	}
}

func TestHandleLogout(t *testing.T) {
	authHandler, db, _, cleanup := setupAuthTest(t)
	defer cleanup()

	// First create a session
	sessionStore := store.NewSessionStore(db)
	session := &store.Session{
		ID:          "test-session-id",
		ABSTokenEnc: []byte("encrypted"),
		ABSUserID:   "user-123",
		ABSUsername: "testuser",
	}
	sessionStore.Create(session)

	// Create request with session cookie
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "bridge_session", Value: "test-session-id"})
	rec := httptest.NewRecorder()

	authHandler.HandleLogout(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %s", location)
	}

	// Session should be deleted from database
	retrieved, _ := sessionStore.Get("test-session-id")
	if retrieved != nil {
		t.Error("session should be deleted")
	}

	// Cookie should be cleared
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "bridge_session" && c.MaxAge >= 0 {
			t.Error("session cookie should be expired")
		}
	}
}

func TestRequireAuth_NoSession(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	handler := authHandler.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without session
	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}
}

func TestRequireAuth_ValidSession(t *testing.T) {
	authHandler, db, _, cleanup := setupAuthTest(t)
	defer cleanup()

	// Create session with properly encrypted token
	sessionStore := store.NewSessionStore(db)
	encryptedToken, err := authHandler.EncryptToken("abs-token-xyz")
	if err != nil {
		t.Fatalf("failed to encrypt token: %v", err)
	}
	session := &store.Session{
		ID:          "valid-session-id",
		ABSTokenEnc: encryptedToken,
		ABSUserID:   "user-123",
		ABSUsername: "testuser",
	}
	sessionStore.Create(session)

	called := false
	handler := authHandler.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Check session is in context
		sess := SessionFromContext(r.Context())
		if sess == nil {
			t.Error("expected session in context")
		}
		if sess.ABSUsername != "testuser" {
			t.Errorf("expected username 'testuser', got '%s'", sess.ABSUsername)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "bridge_session", Value: "valid-session-id"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("protected handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRequireAuth_APIRequest(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	handler := authHandler.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// API request without session
	req := httptest.NewRequest("GET", "/api/something", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 401 for API requests
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestTokenEncryption(t *testing.T) {
	authHandler, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	originalToken := "my-secret-abs-token-12345"

	// Encrypt
	encrypted, err := authHandler.encryptToken(originalToken)
	if err != nil {
		t.Fatalf("failed to encrypt token: %v", err)
	}

	// Should be different from original
	if bytes.Equal(encrypted, []byte(originalToken)) {
		t.Error("encrypted should differ from original")
	}

	// Decrypt
	decrypted, err := authHandler.decryptToken(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt token: %v", err)
	}

	// Should match original
	if decrypted != originalToken {
		t.Errorf("decrypted token doesn't match: %s != %s", decrypted, originalToken)
	}
}

func TestSessionFromContext(t *testing.T) {
	// Test with session
	session := &store.Session{ID: "test", ABSUsername: "user"}
	ctx := context.WithValue(context.Background(), sessionContextKey, session)

	retrieved := SessionFromContext(ctx)
	if retrieved == nil {
		t.Error("expected session from context")
	}
	if retrieved.ABSUsername != "user" {
		t.Errorf("expected username 'user', got '%s'", retrieved.ABSUsername)
	}

	// Test without session
	emptyCtx := context.Background()
	retrieved = SessionFromContext(emptyCtx)
	if retrieved != nil {
		t.Error("expected nil from empty context")
	}
}
