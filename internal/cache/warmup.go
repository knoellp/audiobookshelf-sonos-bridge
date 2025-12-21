package cache

import (
	"context"
	"log/slog"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

// WarmupConfig configures the cache warmup job.
type WarmupConfig struct {
	Interval       time.Duration
	BatchSize      int
	MaxConcurrent  int
}

// DefaultWarmupConfig provides sensible defaults for cache warmup.
var DefaultWarmupConfig = WarmupConfig{
	Interval:      1 * time.Hour,
	BatchSize:     10,
	MaxConcurrent: 2,
}

// TokenDecrypter decrypts ABS tokens from session storage.
type TokenDecrypter interface {
	DecryptToken(encrypted []byte) (string, error)
}

// WarmupJob handles background cache warming.
type WarmupJob struct {
	index         *Index
	worker        *Worker
	absClient     *abs.Client
	sessionStore  *store.SessionStore
	tokenDecrypt  TokenDecrypter
	config        WarmupConfig
	cancel        context.CancelFunc
}

// NewWarmupJob creates a new cache warmup job.
func NewWarmupJob(
	index *Index,
	worker *Worker,
	absClient *abs.Client,
	sessionStore *store.SessionStore,
	tokenDecrypt TokenDecrypter,
	config WarmupConfig,
) *WarmupJob {
	return &WarmupJob{
		index:        index,
		worker:       worker,
		absClient:    absClient,
		sessionStore: sessionStore,
		tokenDecrypt: tokenDecrypt,
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

		if queued >= j.config.BatchSize {
			break
		}
	}

	slog.Info("cache warmup run complete", "queued", queued)
}

// warmLibrary queues items from a library for transcoding.
func (j *WarmupJob) warmLibrary(ctx context.Context, client *abs.Client, libraryID string) int {
	// Get library items
	items, err := client.GetLibraryItems(ctx, libraryID, abs.ItemsOptions{
		Limit: j.config.BatchSize,
		Sort:  "addedAt",
		Desc:  true, // Newest first
	})
	if err != nil {
		slog.Error("failed to get library items", "library_id", libraryID, "error", err)
		return 0
	}

	queued := 0
	for _, item := range items.Results {
		// Check if already cached
		cached, err := j.index.IsCached(item.ID)
		if err != nil {
			continue
		}
		if cached {
			continue
		}

		// Check current status
		status, _ := j.index.GetStatus(item.ID)
		if status == store.CacheStatusInProgress {
			continue // Already being processed
		}

		// Get source path
		audioFile := item.GetPrimaryAudioFile()
		if audioFile == nil {
			continue
		}

		sourcePath := audioFile.Metadata.Path
		if sourcePath == "" {
			continue
		}

		// Create cache entry if needed
		entry, _ := j.index.GetEntry(item.ID)
		if entry == nil {
			if err := j.index.CreateEntry(item.ID, sourcePath, 0, time.Now()); err != nil {
				slog.Warn("failed to create cache entry", "item_id", item.ID, "error", err)
				continue
			}
		}

		// Queue for transcoding
		job := Job{
			ItemID:     item.ID,
			SourcePath: sourcePath,
		}

		if j.worker.Enqueue(job) {
			queued++
			slog.Debug("queued item for cache warmup", "item_id", item.ID)
		}

		if queued >= j.config.BatchSize {
			break
		}
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
