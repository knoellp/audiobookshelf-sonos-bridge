package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"audiobookshelf-sonos-bridge/internal/cache"
	"audiobookshelf-sonos-bridge/internal/store"
)

func TestTokenGenerator_Generate(t *testing.T) {
	gen := NewTokenGenerator("test-secret", time.Hour)

	token, err := gen.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestTokenGenerator_Validate_ValidToken(t *testing.T) {
	gen := NewTokenGenerator("test-secret", time.Hour)

	token, err := gen.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	payload, err := gen.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if payload.ItemID != "item-123" {
		t.Errorf("expected item_id 'item-123', got %q", payload.ItemID)
	}
	if payload.UserID != "user-456" {
		t.Errorf("expected user_id 'user-456', got %q", payload.UserID)
	}
	if payload.SessionID != "session-789" {
		t.Errorf("expected session_id 'session-789', got %q", payload.SessionID)
	}
}

func TestTokenGenerator_Validate_ExpiredToken(t *testing.T) {
	gen := NewTokenGenerator("test-secret", -time.Hour) // Negative TTL = already expired

	token, err := gen.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = gen.Validate(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
	if err.Error() != "token expired" {
		t.Errorf("expected 'token expired' error, got %q", err.Error())
	}
}

func TestTokenGenerator_Validate_InvalidSignature(t *testing.T) {
	gen1 := NewTokenGenerator("secret-1", time.Hour)
	gen2 := NewTokenGenerator("secret-2", time.Hour)

	token, err := gen1.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = gen2.Validate(token)
	if err == nil {
		t.Error("expected error for invalid signature")
	}
	if err.Error() != "invalid token signature" {
		t.Errorf("expected 'invalid token signature' error, got %q", err.Error())
	}
}

func TestTokenGenerator_Validate_InvalidEncoding(t *testing.T) {
	gen := NewTokenGenerator("test-secret", time.Hour)

	_, err := gen.Validate("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid encoding")
	}
}

// setupTestCacheIndex creates a test cache index with database.
func setupTestCacheIndex(t *testing.T, cacheDir string) *cache.Index {
	t.Helper()

	// Create temp database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cacheStore := store.NewCacheStore(db)
	return cache.NewIndex(cacheStore, cacheDir)
}

// createTestCacheEntry creates a cache entry for testing.
func createTestCacheEntry(t *testing.T, idx *cache.Index, itemID, format string) {
	t.Helper()

	if err := idx.CreateEntryWithFormat(itemID, "/test/source.mp3", 1000, time.Now(), format); err != nil {
		t.Fatalf("failed to create cache entry: %v", err)
	}

	// Mark as ready
	if err := idx.MarkReadyWithFormat(itemID, 300, format); err != nil {
		t.Fatalf("failed to mark cache ready: %v", err)
	}
}

func TestHandler_HandleStream_ValidToken(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Create cache index and entry
	cacheIndex := setupTestCacheIndex(t, tmpDir)
	createTestCacheEntry(t, cacheIndex, "item-123", "mp3")

	// Create cache file structure
	itemDir := filepath.Join(tmpDir, "item-123")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	audioFile := filepath.Join(itemDir, "audio.mp3")
	testContent := []byte("fake mp3 content for testing")
	if err := os.WriteFile(audioFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create handler
	tokenGen := NewTokenGenerator("test-secret", time.Hour)
	handler := NewHandler(tokenGen, cacheIndex, "http://localhost:8080")

	// Generate token
	token, err := tokenGen.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatal(err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/stream/"+token+"/audio.mp3", nil)
	w := httptest.NewRecorder()

	handler.HandleStream(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Errorf("expected Content-Type audio/mpeg, got %q", w.Header().Get("Content-Type"))
	}

	if w.Body.String() != string(testContent) {
		t.Errorf("body mismatch")
	}
}

func TestHandler_HandleStream_RangeRequest(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Create cache index and entry
	cacheIndex := setupTestCacheIndex(t, tmpDir)
	createTestCacheEntry(t, cacheIndex, "item-123", "mp3")

	// Create cache file structure
	itemDir := filepath.Join(tmpDir, "item-123")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	audioFile := filepath.Join(itemDir, "audio.mp3")
	testContent := []byte("0123456789ABCDEF") // 16 bytes
	if err := os.WriteFile(audioFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create handler
	tokenGen := NewTokenGenerator("test-secret", time.Hour)
	handler := NewHandler(tokenGen, cacheIndex, "http://localhost:8080")

	// Generate token
	token, err := tokenGen.Generate("item-123", "user-456", "session-789")
	if err != nil {
		t.Fatal(err)
	}

	// Create request with Range header
	req := httptest.NewRequest("GET", "/stream/"+token+"/audio.mp3", nil)
	req.Header.Set("Range", "bytes=5-9")
	w := httptest.NewRecorder()

	handler.HandleStream(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	if w.Header().Get("Content-Range") != "bytes 5-9/16" {
		t.Errorf("expected Content-Range 'bytes 5-9/16', got %q", w.Header().Get("Content-Range"))
	}

	expectedBody := "56789"
	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestHandler_HandleStream_InvalidToken(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache index
	cacheIndex := setupTestCacheIndex(t, tmpDir)

	tokenGen := NewTokenGenerator("test-secret", time.Hour)
	handler := NewHandler(tokenGen, cacheIndex, "http://localhost:8080")

	req := httptest.NewRequest("GET", "/stream/invalid-token/audio.mp3", nil)
	w := httptest.NewRecorder()

	handler.HandleStream(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandler_HandleStream_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache index (no entries)
	cacheIndex := setupTestCacheIndex(t, tmpDir)

	tokenGen := NewTokenGenerator("test-secret", time.Hour)
	handler := NewHandler(tokenGen, cacheIndex, "http://localhost:8080")

	// Generate valid token but no cache entry exists
	token, err := tokenGen.Generate("nonexistent-item", "user-456", "session-789")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/stream/"+token+"/audio.mp3", nil)
	w := httptest.NewRecorder()

	handler.HandleStream(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_GetStreamURL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache index
	cacheIndex := setupTestCacheIndex(t, tmpDir)

	tokenGen := NewTokenGenerator("test-secret", time.Hour)
	handler := NewHandler(tokenGen, cacheIndex, "http://192.168.1.100:8080")

	// Test MP3 format
	url := handler.GetStreamURL("my-token", "mp3")
	expected := "http://192.168.1.100:8080/stream/my-token/audio.mp3"
	if url != expected {
		t.Errorf("mp3: expected %q, got %q", expected, url)
	}

	// Test MP4/M4A format
	url = handler.GetStreamURL("my-token", "mp4")
	expected = "http://192.168.1.100:8080/stream/my-token/audio.m4a"
	if url != expected {
		t.Errorf("mp4: expected %q, got %q", expected, url)
	}
}
