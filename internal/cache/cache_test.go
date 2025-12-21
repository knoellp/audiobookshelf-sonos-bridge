package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"audiobookshelf-sonos-bridge/internal/store"
)

func setupTestDB(t *testing.T) (*store.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	return db, func() {
		db.Close()
	}
}

func TestIndex_IsCached(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Test item not in database
	cached, err := idx.IsCached("nonexistent")
	if err != nil {
		t.Fatalf("IsCached failed: %v", err)
	}
	if cached {
		t.Error("expected not cached for nonexistent item")
	}

	// Create entry but not ready
	entry := &store.CacheEntry{
		ItemID:         "item-1",
		SourcePath:     "/media/book.m4b",
		SourceSize:     1000000,
		SourceMtime:    time.Now(),
		ProfileVersion: CurrentCacheVersion,
		CachePath:      idx.GetCachePath("item-1"),
		Status:         store.CacheStatusPending,
	}
	if err := cacheStore.Create(entry); err != nil {
		t.Fatal(err)
	}

	cached, err = idx.IsCached("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if cached {
		t.Error("expected not cached for pending item")
	}

	// Mark as ready
	if err := cacheStore.MarkReady("item-1", 3600); err != nil {
		t.Fatal(err)
	}

	// Still not cached because file doesn't exist
	cached, err = idx.IsCached("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if cached {
		t.Error("expected not cached when file missing")
	}

	// Create the cache file
	cacheDir := filepath.Dir(idx.GetCachePath("item-1"))
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx.GetCachePath("item-1"), []byte("fake mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	// Now should be cached
	cached, err = idx.IsCached("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if !cached {
		t.Error("expected cached when status ready and file exists")
	}
}

func TestIndex_GetCachePath(t *testing.T) {
	idx := NewIndex(nil, "/cache")

	path := idx.GetCachePath("item-123")
	expected := "/cache/item-123/audio.mp3"

	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestIndex_GetStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Non-existent returns pending
	status, err := idx.GetStatus("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if status != store.CacheStatusPending {
		t.Errorf("expected pending, got %s", status)
	}

	// Create entry
	entry := &store.CacheEntry{
		ItemID:         "item-1",
		SourcePath:     "/media/book.m4b",
		SourceSize:     1000000,
		SourceMtime:    time.Now(),
		ProfileVersion: CurrentCacheVersion,
		CachePath:      idx.GetCachePath("item-1"),
		Status:         store.CacheStatusInProgress,
	}
	if err := cacheStore.Create(entry); err != nil {
		t.Fatal(err)
	}

	status, err = idx.GetStatus("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if status != store.CacheStatusInProgress {
		t.Errorf("expected in_progress, got %s", status)
	}
}

func TestIndex_IsStale(t *testing.T) {
	idx := NewIndex(nil, "/cache")
	now := time.Now()

	entry := &store.CacheEntry{
		ItemID:         "item-1",
		SourceSize:     1000,
		SourceMtime:    now,
		ProfileVersion: CurrentCacheVersion,
	}

	// Same values = not stale
	if idx.IsStale(entry, 1000, now) {
		t.Error("expected not stale with same values")
	}

	// Different size = stale
	if !idx.IsStale(entry, 2000, now) {
		t.Error("expected stale with different size")
	}

	// Different mtime = stale
	if !idx.IsStale(entry, 1000, now.Add(time.Second)) {
		t.Error("expected stale with different mtime")
	}
}

func TestIndex_CreateEntry(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	now := time.Now()
	err := idx.CreateEntry("item-1", "/media/book.m4b", 1000000, now)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := idx.GetEntry("item-1")
	if err != nil {
		t.Fatal(err)
	}

	if entry.ItemID != "item-1" {
		t.Errorf("expected item_id 'item-1', got %q", entry.ItemID)
	}
	if entry.SourcePath != "/media/book.m4b" {
		t.Errorf("expected source_path '/media/book.m4b', got %q", entry.SourcePath)
	}
	if entry.Status != store.CacheStatusPending {
		t.Errorf("expected status pending, got %s", entry.Status)
	}
}

func TestIndex_MarkTransitions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Create entry
	err := idx.CreateEntry("item-1", "/media/book.m4b", 1000000, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Mark in progress
	if err := idx.MarkInProgress("item-1"); err != nil {
		t.Fatal(err)
	}

	entry, _ := idx.GetEntry("item-1")
	if entry.Status != store.CacheStatusInProgress {
		t.Errorf("expected in_progress, got %s", entry.Status)
	}

	// Mark ready
	if err := idx.MarkReady("item-1", 3600); err != nil {
		t.Fatal(err)
	}

	entry, _ = idx.GetEntry("item-1")
	if entry.Status != store.CacheStatusReady {
		t.Errorf("expected ready, got %s", entry.Status)
	}
	if entry.DurationSec == nil || *entry.DurationSec != 3600 {
		t.Errorf("expected duration 3600, got %v", entry.DurationSec)
	}
}

func TestIndex_MarkFailed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Create entry
	err := idx.CreateEntry("item-1", "/media/book.m4b", 1000000, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Mark failed
	if err := idx.MarkFailed("item-1", "ffmpeg error"); err != nil {
		t.Fatal(err)
	}

	entry, _ := idx.GetEntry("item-1")
	if entry.Status != store.CacheStatusFailed {
		t.Errorf("expected failed, got %s", entry.Status)
	}
	if entry.ErrorText != "ffmpeg error" {
		t.Errorf("expected error 'ffmpeg error', got %q", entry.ErrorText)
	}
}

func TestIndex_EnsureDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	idx := NewIndex(nil, tmpDir)

	if err := idx.EnsureDirectory("item-123"); err != nil {
		t.Fatal(err)
	}

	expectedDir := filepath.Join(tmpDir, "item-123")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestIndex_CleanupTempFiles(t *testing.T) {
	tmpDir := t.TempDir()
	idx := NewIndex(nil, tmpDir)

	// Create temp files
	itemDir := filepath.Join(tmpDir, "item-1")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(itemDir, "audio.mp3.tmp")
	if err := os.WriteFile(tmpFile, []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create normal file that should not be deleted
	normalFile := filepath.Join(itemDir, "audio.mp3")
	if err := os.WriteFile(normalFile, []byte("mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := idx.CleanupTempFiles(); err != nil {
		t.Fatal(err)
	}

	// Temp file should be gone
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("expected temp file to be deleted")
	}

	// Normal file should remain
	if _, err := os.Stat(normalFile); os.IsNotExist(err) {
		t.Error("expected normal file to remain")
	}
}

func TestWorker_Enqueue(t *testing.T) {
	worker := NewWorker(nil, nil, 2)

	job := Job{ItemID: "item-1", SourcePath: "/media/book.m4b"}

	if !worker.Enqueue(job) {
		t.Error("expected enqueue to succeed")
	}

	if worker.QueueLength() != 1 {
		t.Errorf("expected queue length 1, got %d", worker.QueueLength())
	}
}

func TestWorker_QueueFull(t *testing.T) {
	// Create worker with small buffer
	worker := &Worker{
		jobs:    make(chan Job, 1),
		workers: 1,
	}

	// First job should succeed
	if !worker.Enqueue(Job{ItemID: "item-1"}) {
		t.Error("first enqueue should succeed")
	}

	// Second job should fail (queue full)
	if worker.Enqueue(Job{ItemID: "item-2"}) {
		t.Error("second enqueue should fail when queue is full")
	}
}

func TestTranscoder_EstimateOutputSize(t *testing.T) {
	tc := NewTranscoder()

	// 60 seconds at 128kbps = 60 * 16KB = 960KB
	size := tc.EstimateOutputSize(60)
	expected := int64(60 * 16 * 1024)

	if size != expected {
		t.Errorf("expected %d, got %d", expected, size)
	}
}

func TestExtractDurationFromFFmpegOutput(t *testing.T) {
	tests := []struct {
		output   string
		expected int
	}{
		{
			output:   "Duration: 01:23:45.67, start: 0.000000, bitrate: 128 kb/s",
			expected: 5025, // 1*3600 + 23*60 + 45
		},
		{
			output:   "Duration: 00:05:30.00, start: 0.000000",
			expected: 330, // 5*60 + 30
		},
		{
			output:   "no duration info",
			expected: 0,
		},
	}

	for _, tc := range tests {
		result := ExtractDurationFromFFmpegOutput(tc.output)
		if result != tc.expected {
			t.Errorf("for %q: expected %d, got %d", tc.output, tc.expected, result)
		}
	}
}

func TestIndex_WaitForReady(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Create entry in progress
	entry := &store.CacheEntry{
		ItemID:         "item-1",
		SourcePath:     "/media/book.m4b",
		SourceSize:     1000000,
		SourceMtime:    time.Now(),
		ProfileVersion: CurrentCacheVersion,
		CachePath:      idx.GetCachePath("item-1"),
		Status:         store.CacheStatusInProgress,
	}
	if err := cacheStore.Create(entry); err != nil {
		t.Fatal(err)
	}

	// Mark ready in background after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cacheStore.MarkReady("item-1", 3600)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := idx.waitForReady(ctx, "item-1")
	if err != nil {
		t.Errorf("waitForReady failed: %v", err)
	}
}

func TestIndex_WaitForReady_Failed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tmpDir := t.TempDir()
	cacheStore := store.NewCacheStore(db)
	idx := NewIndex(cacheStore, tmpDir)

	// Create entry in progress
	entry := &store.CacheEntry{
		ItemID:         "item-1",
		SourcePath:     "/media/book.m4b",
		SourceSize:     1000000,
		SourceMtime:    time.Now(),
		ProfileVersion: CurrentCacheVersion,
		CachePath:      idx.GetCachePath("item-1"),
		Status:         store.CacheStatusInProgress,
	}
	if err := cacheStore.Create(entry); err != nil {
		t.Fatal(err)
	}

	// Mark failed in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		cacheStore.MarkFailed("item-1", "test error")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := idx.waitForReady(ctx, "item-1")
	if err == nil {
		t.Error("expected error for failed transcoding")
	}
}
