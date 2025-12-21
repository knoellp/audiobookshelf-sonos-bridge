package store

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	// Create temp file for test database
	f, err := os.CreateTemp("", "bridge_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := f.Name()
	f.Close()

	db, err := New(dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func TestSessionStore_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSessionStore(db)

	// Create
	session := &Session{
		ID:          "test-session-id",
		ABSTokenEnc: []byte("encrypted-token"),
		ABSUserID:   "user-123",
		ABSUsername: "testuser",
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}

	err := store.Create(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Get
	retrieved, err := store.Get("test-session-id")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected session, got nil")
	}
	if retrieved.ABSUsername != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", retrieved.ABSUsername)
	}

	// Update last used
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	err = store.UpdateLastUsed("test-session-id")
	if err != nil {
		t.Fatalf("failed to update last used: %v", err)
	}

	// List
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	// Delete
	err = store.Delete("test-session-id")
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	// Verify deletion
	retrieved, err = store.Get("test-session-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved != nil {
		t.Error("expected nil after deletion")
	}
}

func TestSessionStore_GetNonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewSessionStore(db)

	session, err := store.Get("non-existent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil {
		t.Error("expected nil for non-existent session")
	}
}

func TestDeviceStore_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDeviceStore(db)

	// Upsert (insert)
	device := &SonosDevice{
		UUID:         "uuid:RINCON_123456",
		Name:         "Living Room",
		IPAddress:    "192.168.1.100",
		LocationURL:  "http://192.168.1.100:1400/xml/device_description.xml",
		Model:        "Sonos One",
		IsReachable:  true,
		DiscoveredAt: time.Now(),
		LastSeenAt:   time.Now(),
	}

	err := store.Upsert(device)
	if err != nil {
		t.Fatalf("failed to upsert device: %v", err)
	}

	// Get
	retrieved, err := store.Get("uuid:RINCON_123456")
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected device, got nil")
	}
	if retrieved.Name != "Living Room" {
		t.Errorf("expected name 'Living Room', got '%s'", retrieved.Name)
	}
	if !retrieved.IsReachable {
		t.Error("expected device to be reachable")
	}

	// Update (upsert existing)
	device.Name = "Kitchen"
	err = store.Upsert(device)
	if err != nil {
		t.Fatalf("failed to update device: %v", err)
	}

	retrieved, _ = store.Get("uuid:RINCON_123456")
	if retrieved.Name != "Kitchen" {
		t.Errorf("expected updated name 'Kitchen', got '%s'", retrieved.Name)
	}

	// SetReachable
	err = store.SetReachable("uuid:RINCON_123456", false)
	if err != nil {
		t.Fatalf("failed to set reachable: %v", err)
	}

	retrieved, _ = store.Get("uuid:RINCON_123456")
	if retrieved.IsReachable {
		t.Error("expected device to be unreachable")
	}

	// List
	devices, err := store.List()
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}

	// ListReachable (should be empty)
	reachable, err := store.ListReachable()
	if err != nil {
		t.Fatalf("failed to list reachable: %v", err)
	}
	if len(reachable) != 0 {
		t.Errorf("expected 0 reachable devices, got %d", len(reachable))
	}

	// Delete
	err = store.Delete("uuid:RINCON_123456")
	if err != nil {
		t.Fatalf("failed to delete device: %v", err)
	}

	retrieved, _ = store.Get("uuid:RINCON_123456")
	if retrieved != nil {
		t.Error("expected nil after deletion")
	}
}

func TestCacheStore_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCacheStore(db)

	// Create
	duration := 3600
	entry := &CacheEntry{
		ItemID:         "item-123",
		SourcePath:     "/media/books/test.m4b",
		SourceSize:     1024000,
		SourceMtime:    time.Now(),
		ProfileVersion: "v3",
		CachePath:      "/cache/item-123/audio.mp3",
		DurationSec:    &duration,
		Status:         CacheStatusPending,
	}

	err := store.Create(entry)
	if err != nil {
		t.Fatalf("failed to create cache entry: %v", err)
	}

	// Get
	retrieved, err := store.Get("item-123")
	if err != nil {
		t.Fatalf("failed to get cache entry: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected cache entry, got nil")
	}
	if retrieved.Status != CacheStatusPending {
		t.Errorf("expected status 'pending', got '%s'", retrieved.Status)
	}
	if retrieved.DurationSec == nil || *retrieved.DurationSec != 3600 {
		t.Error("expected duration 3600")
	}

	// MarkInProgress
	err = store.MarkInProgress("item-123")
	if err != nil {
		t.Fatalf("failed to mark in progress: %v", err)
	}

	retrieved, _ = store.Get("item-123")
	if retrieved.Status != CacheStatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", retrieved.Status)
	}

	// MarkReady
	err = store.MarkReady("item-123", 3700)
	if err != nil {
		t.Fatalf("failed to mark ready: %v", err)
	}

	retrieved, _ = store.Get("item-123")
	if retrieved.Status != CacheStatusReady {
		t.Errorf("expected status 'ready', got '%s'", retrieved.Status)
	}
	if retrieved.DurationSec == nil || *retrieved.DurationSec != 3700 {
		t.Error("expected updated duration 3700")
	}

	// ListByStatus
	ready, err := store.ListByStatus(CacheStatusReady)
	if err != nil {
		t.Fatalf("failed to list by status: %v", err)
	}
	if len(ready) != 1 {
		t.Errorf("expected 1 ready entry, got %d", len(ready))
	}

	// Delete
	err = store.Delete("item-123")
	if err != nil {
		t.Fatalf("failed to delete cache entry: %v", err)
	}

	retrieved, _ = store.Get("item-123")
	if retrieved != nil {
		t.Error("expected nil after deletion")
	}
}

func TestCacheStore_ResetInProgressToPending(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCacheStore(db)

	// Create entries with different statuses
	entries := []struct {
		itemID string
		status CacheStatus
	}{
		{"item-1", CacheStatusPending},
		{"item-2", CacheStatusInProgress},
		{"item-3", CacheStatusInProgress},
		{"item-4", CacheStatusReady},
	}

	for _, e := range entries {
		entry := &CacheEntry{
			ItemID:         e.itemID,
			SourcePath:     "/media/test.m4b",
			SourceSize:     1024,
			SourceMtime:    time.Now(),
			ProfileVersion: "v3",
			CachePath:      "/cache/" + e.itemID + "/audio.mp3",
			Status:         e.status,
		}
		if err := store.Create(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Reset in_progress to pending
	count, err := store.ResetInProgressToPending()
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 entries reset, got %d", count)
	}

	// Verify
	pending, _ := store.ListByStatus(CacheStatusPending)
	if len(pending) != 3 {
		t.Errorf("expected 3 pending entries, got %d", len(pending))
	}

	inProgress, _ := store.ListByStatus(CacheStatusInProgress)
	if len(inProgress) != 0 {
		t.Errorf("expected 0 in_progress entries, got %d", len(inProgress))
	}
}

func TestPlaybackStore_CRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// First create a session (for foreign key)
	sessionStore := NewSessionStore(db)
	session := &Session{
		ID:          "session-123",
		ABSTokenEnc: []byte("token"),
		ABSUserID:   "user-1",
		ABSUsername: "test",
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}
	sessionStore.Create(session)

	// Create a device (for foreign key)
	deviceStore := NewDeviceStore(db)
	device := &SonosDevice{
		UUID:         "uuid:RINCON_123",
		Name:         "Test",
		IPAddress:    "192.168.1.100",
		LocationURL:  "http://192.168.1.100:1400/xml/device_description.xml",
		IsReachable:  true,
		DiscoveredAt: time.Now(),
		LastSeenAt:   time.Now(),
	}
	deviceStore.Upsert(device)

	store := NewPlaybackStore(db)

	// Create
	playback := &PlaybackSession{
		ID:                  "playback-123",
		SessionID:           "session-123",
		ItemID:              "item-456",
		SonosUUID:           "uuid:RINCON_123",
		StreamToken:         "stream-token-xyz",
		PositionSec:         120,
		DurationSec:         3600,
		IsPlaying:           true,
		StartedAt:           time.Now(),
		LastPositionUpdate:  time.Now(),
		ABSProgressSyncedAt: time.Now(),
	}

	err := store.Create(playback)
	if err != nil {
		t.Fatalf("failed to create playback session: %v", err)
	}

	// Get
	retrieved, err := store.Get("playback-123")
	if err != nil {
		t.Fatalf("failed to get playback session: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected playback session, got nil")
	}
	if retrieved.PositionSec != 120 {
		t.Errorf("expected position 120, got %d", retrieved.PositionSec)
	}
	if !retrieved.IsPlaying {
		t.Error("expected is_playing to be true")
	}

	// GetBySessionID
	retrieved, err = store.GetBySessionID("session-123")
	if err != nil {
		t.Fatalf("failed to get by session ID: %v", err)
	}
	if retrieved == nil || retrieved.ID != "playback-123" {
		t.Error("expected to find playback session by session ID")
	}

	// GetByToken
	retrieved, err = store.GetByToken("stream-token-xyz")
	if err != nil {
		t.Fatalf("failed to get by token: %v", err)
	}
	if retrieved == nil || retrieved.ID != "playback-123" {
		t.Error("expected to find playback session by token")
	}

	// UpdatePosition
	err = store.UpdatePosition("playback-123", 300)
	if err != nil {
		t.Fatalf("failed to update position: %v", err)
	}

	retrieved, _ = store.Get("playback-123")
	if retrieved.PositionSec != 300 {
		t.Errorf("expected position 300, got %d", retrieved.PositionSec)
	}

	// UpdatePlaying
	err = store.UpdatePlaying("playback-123", false)
	if err != nil {
		t.Fatalf("failed to update playing: %v", err)
	}

	retrieved, _ = store.Get("playback-123")
	if retrieved.IsPlaying {
		t.Error("expected is_playing to be false")
	}

	// ListActive (should be empty now)
	active, err := store.ListActive()
	if err != nil {
		t.Fatalf("failed to list active: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active sessions, got %d", len(active))
	}

	// Delete
	err = store.Delete("playback-123")
	if err != nil {
		t.Fatalf("failed to delete playback session: %v", err)
	}

	retrieved, _ = store.Get("playback-123")
	if retrieved != nil {
		t.Error("expected nil after deletion")
	}
}

func TestDatabaseMigrations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Running migrations again should not fail (IF NOT EXISTS)
	err := db.migrate()
	if err != nil {
		t.Fatalf("running migrations twice should not fail: %v", err)
	}
}
