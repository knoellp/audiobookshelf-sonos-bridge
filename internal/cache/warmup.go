package cache

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

// WarmupConfig configures the cache warmup job.
type WarmupConfig struct {
	Interval      time.Duration
	MaxConcurrent int
}

// DefaultWarmupConfig provides sensible defaults for cache warmup.
var DefaultWarmupConfig = WarmupConfig{
	Interval:      1 * time.Hour,
	MaxConcurrent: 2,
}

// TokenDecrypter decrypts ABS tokens from session storage.
type TokenDecrypter interface {
	DecryptToken(encrypted []byte) (string, error)
}

// WarmupJob handles background cache warming.
type WarmupJob struct {
	index        *Index
	worker       *Worker
	absClient    *abs.Client
	sessionStore *store.SessionStore
	tokenDecrypt TokenDecrypter
	pathMapper   func(string) string
	config       WarmupConfig
	cancel       context.CancelFunc
}

// NewWarmupJob creates a new cache warmup job.
func NewWarmupJob(
	index *Index,
	worker *Worker,
	absClient *abs.Client,
	sessionStore *store.SessionStore,
	tokenDecrypt TokenDecrypter,
	pathMapper func(string) string,
	config WarmupConfig,
) *WarmupJob {
	return &WarmupJob{
		index:        index,
		worker:       worker,
		absClient:    absClient,
		sessionStore: sessionStore,
		tokenDecrypt: tokenDecrypt,
		pathMapper:   pathMapper,
		config:       config,
	}
}

// Start begins the warmup job.
func (j *WarmupJob) Start(ctx context.Context) {
	ctx, j.cancel = context.WithCancel(ctx)

	// Run initial warmup after a short delay
	go func() {
		// Wait for system to stabilize
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}

		j.run(ctx)

		// Then run on interval
		ticker := time.NewTicker(j.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.run(ctx)
			}
		}
	}()

	slog.Info("cache warmup job started", "interval", j.config.Interval)
}

// Stop stops the warmup job.
func (j *WarmupJob) Stop() {
	if j.cancel != nil {
		j.cancel()
	}
	slog.Info("cache warmup job stopped")
}

// run performs a single warmup pass.
func (j *WarmupJob) run(ctx context.Context) {
	slog.Info("starting cache warmup run")

	// Get a session with an active ABS token
	sessions, err := j.sessionStore.ListActive()
	if err != nil {
		slog.Error("failed to get active sessions", "error", err)
		return
	}

	if len(sessions) == 0 {
		slog.Debug("no active sessions, skipping warmup")
		return
	}

	// Use the first active session's token
	session := sessions[0]

	// Decrypt the token
	token, err := j.tokenDecrypt.DecryptToken(session.ABSTokenEnc)
	if err != nil {
		slog.Error("failed to decrypt token", "error", err)
		return
	}

	client := j.absClient.WithToken(token)

	// Get libraries
	libraries, err := client.GetLibraries(ctx)
	if err != nil {
		slog.Error("failed to get libraries", "error", err)
		return
	}

	queued := 0
	for _, lib := range libraries {
		if lib.MediaType != "book" {
			continue // Only warm audiobooks
		}

		queued += j.warmLibrary(ctx, client, lib.ID)
	}

	slog.Info("cache warmup run complete", "queued", queued)
}

// warmLibrary paginates through all items in a library and queues uncached ones.
func (j *WarmupJob) warmLibrary(ctx context.Context, client *abs.Client, libraryID string) int {
	const pageSize = 50
	const earlyExitThreshold = 20 // stop after N consecutive cached items

	queued := 0
	consecutiveCached := 0
	page := 0

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return queued
		default:
		}

		items, err := client.GetLibraryItems(ctx, libraryID, abs.ItemsOptions{
			Limit: pageSize,
			Page:  page,
			Sort:  "addedAt",
			Desc:  true, // Newest first
		})
		if err != nil {
			slog.Error("failed to get library items", "library_id", libraryID, "page", page, "error", err)
			return queued
		}

		if len(items.Results) == 0 {
			break // No more items
		}

		for _, item := range items.Results {
			// Check if already cached
			cached, err := j.index.IsCached(item.ID)
			if err != nil {
				continue
			}

			if cached {
				consecutiveCached++
				if consecutiveCached >= earlyExitThreshold {
					slog.Info("warmup early exit: reached consecutive cached threshold",
						"library_id", libraryID,
						"threshold", earlyExitThreshold,
						"queued", queued,
						"page", page)
					return queued
				}
				continue
			}

			// Reset consecutive counter on uncached item
			consecutiveCached = 0

			// Check current status
			status, _ := j.index.GetStatus(item.ID)
			if status == store.CacheStatusInProgress {
				continue // Already being processed
			}

			// Retry failed items only if last attempt was > 1 hour ago
			if status == store.CacheStatusFailed {
				entry, _ := j.index.GetEntry(item.ID)
				if entry != nil && time.Since(entry.UpdatedAt) < 1*time.Hour {
					continue // Too recent, skip for now
				}
			}

			// Get full item details for multi-file support
			fullItem, err := client.GetItem(ctx, item.ID)
			if err != nil {
				slog.Warn("failed to get item details for warmup", "item_id", item.ID, "error", err)
				continue
			}

			if len(fullItem.Media.AudioFiles) == 0 {
				continue
			}

			// Sort audio files by index and map paths
			audioFiles := make([]abs.AudioFile, len(fullItem.Media.AudioFiles))
			copy(audioFiles, fullItem.Media.AudioFiles)
			sort.Slice(audioFiles, func(i, k int) bool {
				return audioFiles[i].Index < audioFiles[k].Index
			})

			sourcePaths := make([]string, 0, len(audioFiles))
			for _, af := range audioFiles {
				if af.Metadata.Path == "" {
					continue
				}
				sourcePaths = append(sourcePaths, j.pathMapper(af.Metadata.Path))
			}
			if len(sourcePaths) == 0 {
				continue
			}

			// Create cache entry if needed
			entry, _ := j.index.GetEntry(item.ID)
			if entry == nil {
				if err := j.index.CreateEntry(item.ID, sourcePaths[0], 0, time.Now()); err != nil {
					slog.Warn("failed to create cache entry", "item_id", item.ID, "error", err)
					continue
				}
			}

			// Queue for transcoding with all source paths
			job := Job{
				ItemID:      item.ID,
				SourcePaths: sourcePaths,
			}

			if j.worker.Enqueue(job) {
				queued++
				slog.Debug("queued item for cache warmup",
					"item_id", item.ID,
					"files", len(sourcePaths))
			}
		}

		// Check if we've reached the last page
		if len(items.Results) < pageSize {
			break
		}

		page++
	}

	return queued
}

// CleanupStale cleans up stale in_progress entries on startup.
func (j *WarmupJob) CleanupStale(ctx context.Context) error {
	slog.Info("cleaning up stale in_progress entries")

	// Get all in_progress entries
	entries, err := j.index.store.ListByStatus(store.CacheStatusInProgress)
	if err != nil {
		return err
	}

	cleaned := 0
	for _, entry := range entries {
		// Reset to pending so they can be re-queued
		if err := j.index.store.MarkFailed(entry.ItemID, "interrupted"); err != nil {
			slog.Warn("failed to reset stale entry", "item_id", entry.ItemID, "error", err)
			continue
		}
		cleaned++
	}

	// Also clean up any temp files
	if err := j.index.CleanupTempFiles(); err != nil {
		slog.Warn("failed to cleanup temp files", "error", err)
	}

	slog.Info("cleaned up stale entries", "count", cleaned)
	return nil
}
