package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"audiobookshelf-sonos-bridge/internal/store"
)

const (
	// CurrentCacheVersion is stored in DB entries but no longer used in paths.
	// Cache invalidation is now done by deleting the cache folder.
	CurrentCacheVersion = "v3"
)

// GetCacheFileName returns the cache filename for the given format.
func GetCacheFileName(format string) string {
	switch format {
	case "mp4":
		return "audio.m4a"
	case "mp3":
		return "audio.mp3"
	case "flac":
		return "audio.flac"
	case "ogg":
		return "audio.ogg"
	case "asf":
		return "audio.wma"
	default:
		return "audio.mp3" // fallback
	}
}

// GetContentType returns the MIME type for the given format.
func GetContentType(format string) string {
	switch format {
	case "mp4":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "asf":
		return "audio/x-ms-wma"
	default:
		return "audio/mpeg" // fallback
	}
}

// Index manages the cache index and provides methods to check and update cache status.
type Index struct {
	store    *store.CacheStore
	cacheDir string
	mu       sync.RWMutex
}

// NewIndex creates a new cache index manager.
func NewIndex(cacheStore *store.CacheStore, cacheDir string) *Index {
	return &Index{
		store:    cacheStore,
		cacheDir: cacheDir,
	}
}

// IsCached checks if an item is cached and valid.
func (idx *Index) IsCached(itemID string) (bool, error) {
	entry, err := idx.store.Get(itemID)
	if err != nil {
		return false, err
	}

	if entry == nil {
		return false, nil
	}

	if entry.Status != store.CacheStatusReady {
		return false, nil
	}

	// Check for segmented cache
	if entry.IsSegmented() {
		return idx.verifySegmentedCache(entry)
	}

	// Get the correct cache path based on the entry's format
	cachePath := idx.GetCachePathFromEntry(entry)

	// Verify cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// verifySegmentedCache checks if all segments exist for a segmented cache entry.
func (idx *Index) verifySegmentedCache(entry *store.CacheEntry) (bool, error) {
	cacheDir := idx.GetCacheDir(entry.ItemID)

	for i := 0; i < entry.SegmentCount; i++ {
		segmentFile := entry.GetSegmentFileName(i)
		segmentPath := filepath.Join(cacheDir, segmentFile)
		if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
			return false, nil
		}
	}

	return true, nil
}

// GetCachePath returns the path to the cached file for an item (default mp3).
// Deprecated: Use GetCachePathWithFormat or GetCachePathFromEntry instead.
func (idx *Index) GetCachePath(itemID string) string {
	return idx.GetCachePathWithFormat(itemID, "mp3")
}

// GetCachePathWithFormat returns the path to the cached file for an item with the given format.
func (idx *Index) GetCachePathWithFormat(itemID, format string) string {
	return filepath.Join(idx.cacheDir, itemID, GetCacheFileName(format))
}

// GetCachePathFromEntry returns the path to the cached file based on the entry's format.
func (idx *Index) GetCachePathFromEntry(entry *store.CacheEntry) string {
	if entry == nil {
		return ""
	}
	format := entry.CacheFormat
	if format == "" {
		format = "mp3" // default for old entries
	}
	return idx.GetCachePathWithFormat(entry.ItemID, format)
}

// GetTempPath returns the temporary path for transcoding (default mp3).
// Deprecated: Use GetTempPathWithFormat instead.
func (idx *Index) GetTempPath(itemID string) string {
	return idx.GetTempPathWithFormat(itemID, "mp3")
}

// GetTempPathWithFormat returns the temporary path for transcoding with the given format.
func (idx *Index) GetTempPathWithFormat(itemID, format string) string {
	return filepath.Join(idx.cacheDir, itemID, GetCacheFileName(format)+".tmp")
}

// GetStatus returns the cache status for an item.
func (idx *Index) GetStatus(itemID string) (store.CacheStatus, error) {
	entry, err := idx.store.Get(itemID)
	if err != nil {
		return "", err
	}

	if entry == nil {
		return store.CacheStatusPending, nil
	}

	return entry.Status, nil
}

// GetEntry returns the full cache entry for an item.
func (idx *Index) GetEntry(itemID string) (*store.CacheEntry, error) {
	return idx.store.Get(itemID)
}

// CreateEntry creates a new pending cache entry.
// Deprecated: Use CreateEntryWithFormat instead.
func (idx *Index) CreateEntry(itemID, sourcePath string, sourceSize int64, sourceMtime time.Time) error {
	return idx.CreateEntryWithFormat(itemID, sourcePath, sourceSize, sourceMtime, "mp3")
}

// CreateEntryWithFormat creates a new pending cache entry with a specific format.
// If an entry already exists, it is deleted first (for version migration).
func (idx *Index) CreateEntryWithFormat(itemID, sourcePath string, sourceSize int64, sourceMtime time.Time, format string) error {
	// Delete existing entry if any (handles version migration)
	_ = idx.store.Delete(itemID)

	entry := &store.CacheEntry{
		ItemID:         itemID,
		SourcePath:     sourcePath,
		SourceSize:     sourceSize,
		SourceMtime:    sourceMtime,
		ProfileVersion: CurrentCacheVersion,
		CachePath:      idx.GetCachePathWithFormat(itemID, format),
		CacheFormat:    format,
		Status:         store.CacheStatusPending,
	}

	return idx.store.Create(entry)
}

// MarkInProgress marks an entry as in progress.
func (idx *Index) MarkInProgress(itemID string) error {
	return idx.store.MarkInProgress(itemID)
}

// MarkReady marks an entry as ready with the given duration.
// Deprecated: Use MarkReadyWithFormat instead.
func (idx *Index) MarkReady(itemID string, durationSec int) error {
	return idx.store.MarkReady(itemID, durationSec)
}

// MarkReadyWithFormat marks an entry as ready with the given duration and format.
func (idx *Index) MarkReadyWithFormat(itemID string, durationSec int, format string) error {
	return idx.store.MarkReadyWithFormat(itemID, durationSec, format)
}

// MarkReadyWithSegments marks an entry as ready with segment information.
func (idx *Index) MarkReadyWithSegments(itemID string, durationSec int, format string, segmentCount int, segmentDurationSec int) error {
	return idx.store.MarkReadyWithSegments(itemID, durationSec, format, segmentCount, segmentDurationSec)
}

// GetCacheDir returns the cache directory path for an item.
func (idx *Index) GetCacheDir(itemID string) string {
	return filepath.Join(idx.cacheDir, itemID)
}

// MarkFailed marks an entry as failed with an error message.
func (idx *Index) MarkFailed(itemID string, errorText string) error {
	return idx.store.MarkFailed(itemID, errorText)
}

// Delete removes a cache entry.
func (idx *Index) Delete(itemID string) error {
	return idx.store.Delete(itemID)
}

// IsStale checks if a cache entry is stale (source changed).
func (idx *Index) IsStale(entry *store.CacheEntry, currentSize int64, currentMtime time.Time) bool {
	if entry.SourceSize != currentSize {
		return true
	}

	if !entry.SourceMtime.Equal(currentMtime) {
		return true
	}

	return false
}

// EnsureDirectory creates the cache directory for an item.
func (idx *Index) EnsureDirectory(itemID string) error {
	dir := filepath.Join(idx.cacheDir, itemID)
	return os.MkdirAll(dir, 0755)
}

// EnsureCached blocks until the item is cached or returns an error.
func (idx *Index) EnsureCached(ctx context.Context, itemID string, transcode func(ctx context.Context, itemID string) error) error {
	idx.mu.Lock()

	// Check current status
	entry, err := idx.store.Get(itemID)
	if err != nil {
		idx.mu.Unlock()
		return fmt.Errorf("failed to get cache entry: %w", err)
	}

	// Already cached
	if entry != nil && entry.Status == store.CacheStatusReady {
		// Verify file exists
		if _, err := os.Stat(entry.CachePath); err == nil {
			idx.mu.Unlock()
			return nil
		}
		// File missing, need to re-transcode
	}

	// Already in progress - wait for it
	if entry != nil && entry.Status == store.CacheStatusInProgress {
		idx.mu.Unlock()
		return idx.waitForReady(ctx, itemID)
	}

	// Start transcoding
	idx.mu.Unlock()

	// Transcode
	if err := transcode(ctx, itemID); err != nil {
		return fmt.Errorf("transcoding failed: %w", err)
	}

	return nil
}

// waitForReady waits until an item is ready or times out.
func (idx *Index) waitForReady(ctx context.Context, itemID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			entry, err := idx.store.Get(itemID)
			if err != nil {
				return err
			}

			if entry == nil {
				return fmt.Errorf("cache entry disappeared")
			}

			switch entry.Status {
			case store.CacheStatusReady:
				return nil
			case store.CacheStatusFailed:
				return fmt.Errorf("transcoding failed: %s", entry.ErrorText)
			case store.CacheStatusPending, store.CacheStatusInProgress:
				continue
			}
		}
	}
}

// GetPendingItems returns items waiting to be transcoded.
func (idx *Index) GetPendingItems() ([]*store.CacheEntry, error) {
	return idx.store.ListByStatus(store.CacheStatusPending)
}

// CleanupTempFiles removes any leftover temporary files.
func (idx *Index) CleanupTempFiles() error {
	pattern := filepath.Join(idx.cacheDir, "*", "*.tmp")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		os.Remove(match)
	}

	return nil
}
