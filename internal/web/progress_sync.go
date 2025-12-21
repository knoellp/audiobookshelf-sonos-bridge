package web

import (
	"context"
	"log/slog"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
)

// TokenDecrypter decrypts ABS tokens from session storage.
type TokenDecrypter interface {
	DecryptToken(encrypted []byte) (string, error)
}

// ProgressSyncer handles background progress synchronization.
type ProgressSyncer struct {
	absClient     *abs.Client
	playbackStore *store.PlaybackStore
	sessionStore  *store.SessionStore
	deviceStore   *store.DeviceStore
	tokenDecrypt  TokenDecrypter
	pollInterval  time.Duration
	syncInterval  time.Duration
	cancel        context.CancelFunc
}

// NewProgressSyncer creates a new progress syncer.
func NewProgressSyncer(
	absClient *abs.Client,
	playbackStore *store.PlaybackStore,
	sessionStore *store.SessionStore,
	deviceStore *store.DeviceStore,
	tokenDecrypt TokenDecrypter,
) *ProgressSyncer {
	return &ProgressSyncer{
		absClient:     absClient,
		playbackStore: playbackStore,
		sessionStore:  sessionStore,
		deviceStore:   deviceStore,
		tokenDecrypt:  tokenDecrypt,
		pollInterval:  5 * time.Second,
		syncInterval:  30 * time.Second,
	}
}

// Start begins the background sync process.
func (s *ProgressSyncer) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	go s.pollLoop(ctx)
	go s.syncLoop(ctx)

	slog.Info("progress syncer started",
		"poll_interval", s.pollInterval,
		"sync_interval", s.syncInterval,
	)
}

// Stop stops the background sync process.
func (s *ProgressSyncer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	slog.Info("progress syncer stopped")
}

// pollLoop polls Sonos devices for position updates.
func (s *ProgressSyncer) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollAllActive(ctx)
		}
	}
}

// syncLoop periodically syncs progress to Audiobookshelf.
func (s *ProgressSyncer) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAllActive(ctx)
		}
	}
}

// pollAllActive polls all active playback sessions.
func (s *ProgressSyncer) pollAllActive(ctx context.Context) {
	sessions, err := s.playbackStore.ListActive()
	if err != nil {
		slog.Error("failed to list active sessions", "error", err)
		return
	}

	for _, playback := range sessions {
		s.pollSession(ctx, playback)
	}
}

// pollSession polls a single playback session for position.
func (s *ProgressSyncer) pollSession(ctx context.Context, playback *store.PlaybackSession) {
	// Get device
	device, err := s.deviceStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		slog.Warn("device not found for playback",
			"session_id", playback.SessionID,
			"sonos_uuid", playback.SonosUUID,
		)
		return
	}

	// Get position from Sonos
	avt := sonos.NewAVTransport(device.IPAddress)
	posInfo, err := avt.GetPositionInfo(ctx)
	if err != nil {
		slog.Debug("failed to get position info",
			"session_id", playback.SessionID,
			"error", err,
		)
		return
	}

	// Parse position
	relTime := sonos.ParseDuration(posInfo.RelTime)
	positionSec := int(relTime.Seconds())

	// Check if playback has ended
	trackDuration := sonos.ParseDuration(posInfo.TrackDuration)
	if trackDuration > 0 && relTime >= trackDuration-time.Second {
		// Track has ended
		slog.Info("playback ended",
			"session_id", playback.SessionID,
			"item_id", playback.ItemID,
		)
		s.playbackStore.UpdatePlaying(playback.ID, false)
		return
	}

	// Update stored position
	if positionSec != playback.PositionSec {
		s.playbackStore.UpdatePosition(playback.ID, positionSec)
	}
}

// syncAllActive syncs progress for all active sessions to Audiobookshelf.
func (s *ProgressSyncer) syncAllActive(ctx context.Context) {
	sessions, err := s.playbackStore.ListActive()
	if err != nil {
		slog.Error("failed to list active sessions", "error", err)
		return
	}

	for _, playback := range sessions {
		s.syncSession(ctx, playback)
	}
}

// syncSession syncs a single playback session to Audiobookshelf.
func (s *ProgressSyncer) syncSession(ctx context.Context, playback *store.PlaybackSession) {
	// Get user session for token
	session, err := s.sessionStore.Get(playback.SessionID)
	if err != nil || session == nil {
		slog.Warn("session not found for playback", "session_id", playback.SessionID)
		return
	}

	// Decrypt the token
	token, err := s.tokenDecrypt.DecryptToken(session.ABSTokenEnc)
	if err != nil {
		slog.Warn("failed to decrypt token", "session_id", playback.SessionID, "error", err)
		return
	}

	// Create client with user's token
	client := s.absClient.WithToken(token)

	// Build progress update
	progress := float64(0)
	if playback.DurationSec > 0 {
		progress = float64(playback.PositionSec) / float64(playback.DurationSec)
	}

	update := abs.ProgressUpdate{
		CurrentTime: float64(playback.PositionSec),
		Duration:    float64(playback.DurationSec),
		Progress:    progress,
	}

	// Sync to ABS
	if err := client.UpdateProgress(ctx, playback.ItemID, update); err != nil {
		slog.Warn("failed to sync progress",
			"session_id", playback.SessionID,
			"item_id", playback.ItemID,
			"error", err,
		)
		return
	}

	// Update sync timestamp
	s.playbackStore.UpdateABSSyncTime(playback.ID)

	slog.Debug("synced progress to ABS",
		"item_id", playback.ItemID,
		"position_sec", playback.PositionSec,
		"progress", progress,
	)
}

// SyncNow forces an immediate sync for a specific session.
func (s *ProgressSyncer) SyncNow(ctx context.Context, sessionID string) error {
	playback, err := s.playbackStore.GetBySessionID(sessionID)
	if err != nil {
		return err
	}
	if playback == nil {
		return nil
	}

	s.syncSession(ctx, playback)
	return nil
}
