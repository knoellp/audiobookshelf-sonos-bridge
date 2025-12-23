package web

import (
	"context"
	"log/slog"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
)

// SleepTimerWorker handles background sleep timer checking and triggering.
type SleepTimerWorker struct {
	playbackStore *store.PlaybackStore
	sessionStore  *store.SessionStore
	deviceStore   *store.DeviceStore
	absClient     *abs.Client
	tokenDecrypt  TokenDecrypter
	checkInterval time.Duration
	cancel        context.CancelFunc
}

// NewSleepTimerWorker creates a new sleep timer worker.
func NewSleepTimerWorker(
	playbackStore *store.PlaybackStore,
	sessionStore *store.SessionStore,
	deviceStore *store.DeviceStore,
	absClient *abs.Client,
	tokenDecrypt TokenDecrypter,
) *SleepTimerWorker {
	return &SleepTimerWorker{
		playbackStore: playbackStore,
		sessionStore:  sessionStore,
		deviceStore:   deviceStore,
		absClient:     absClient,
		tokenDecrypt:  tokenDecrypt,
		checkInterval: 10 * time.Second,
	}
}

// Start begins the background timer checking process.
func (w *SleepTimerWorker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	go w.checkLoop(ctx)

	slog.Info("sleep timer worker started", "check_interval", w.checkInterval)
}

// Stop stops the background timer checking process.
func (w *SleepTimerWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	slog.Info("sleep timer worker stopped")
}

// checkLoop periodically checks for expired sleep timers.
func (w *SleepTimerWorker) checkLoop(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkExpiredTimers(ctx)
		}
	}
}

// checkExpiredTimers checks all sessions with active timers and triggers expired ones.
func (w *SleepTimerWorker) checkExpiredTimers(ctx context.Context) {
	sessions, err := w.playbackStore.GetSessionsWithActiveTimer()
	if err != nil {
		slog.Error("failed to get sessions with active timer", "error", err)
		return
	}

	now := time.Now()
	for _, session := range sessions {
		if session.SleepAt != nil && now.After(*session.SleepAt) {
			w.triggerSleepTimer(ctx, session)
		}
	}
}

// triggerSleepTimer pauses playback and syncs progress when a sleep timer expires.
func (w *SleepTimerWorker) triggerSleepTimer(ctx context.Context, session *store.PlaybackSession) {
	slog.Info("sleep timer triggered",
		"session_id", session.SessionID,
		"item_id", session.ItemID,
		"sonos_uuid", session.SonosUUID,
	)

	// Get device IP for Sonos control
	device, err := w.deviceStore.Get(session.SonosUUID)
	if err != nil || device == nil {
		slog.Error("failed to get device for sleep timer",
			"session_id", session.SessionID,
			"sonos_uuid", session.SonosUUID,
			"error", err,
		)
		// Clear the timer anyway to avoid repeated attempts
		w.playbackStore.ClearSleepTimer(session.ID)
		return
	}

	// Get current position from Sonos before pausing
	avt := sonos.NewAVTransport(device.IPAddress)
	posInfo, err := avt.GetPositionInfo(ctx)
	if err != nil {
		slog.Warn("failed to get position before sleep pause",
			"session_id", session.SessionID,
			"error", err,
		)
	} else {
		// Update position in database
		relTime := sonos.ParseDuration(posInfo.RelTime)
		positionSec := int(relTime.Seconds())
		if err := w.playbackStore.UpdatePosition(session.ID, positionSec); err != nil {
			slog.Warn("failed to update position before sleep pause",
				"session_id", session.SessionID,
				"error", err,
			)
		}
		session.PositionSec = positionSec
	}

	// Pause playback on Sonos
	if err := avt.Pause(ctx); err != nil {
		slog.Error("failed to pause for sleep timer",
			"session_id", session.SessionID,
			"error", err,
		)
	} else {
		slog.Info("sleep timer paused playback",
			"session_id", session.SessionID,
			"device", device.Name,
		)
	}

	// Mark session as not playing
	if err := w.playbackStore.UpdatePlaying(session.ID, false); err != nil {
		slog.Warn("failed to update playing state after sleep timer",
			"session_id", session.SessionID,
			"error", err,
		)
	}

	// Sync progress to Audiobookshelf
	w.syncProgressToABS(ctx, session)

	// Clear the sleep timer
	if err := w.playbackStore.ClearSleepTimer(session.ID); err != nil {
		slog.Error("failed to clear sleep timer",
			"session_id", session.SessionID,
			"error", err,
		)
	}
}

// syncProgressToABS syncs the current playback progress to Audiobookshelf.
func (w *SleepTimerWorker) syncProgressToABS(ctx context.Context, session *store.PlaybackSession) {
	// Get user session for token
	userSession, err := w.sessionStore.Get(session.SessionID)
	if err != nil || userSession == nil {
		slog.Warn("failed to get user session for ABS sync",
			"session_id", session.SessionID,
			"error", err,
		)
		return
	}

	// Decrypt the token
	token, err := w.tokenDecrypt.DecryptToken(userSession.ABSTokenEnc)
	if err != nil {
		slog.Warn("failed to decrypt token for ABS sync",
			"session_id", session.SessionID,
			"error", err,
		)
		return
	}

	// Create client with user's token
	client := w.absClient.WithToken(token)

	// Build progress update
	progress := float64(0)
	if session.DurationSec > 0 {
		progress = float64(session.PositionSec) / float64(session.DurationSec)
	}

	update := abs.ProgressUpdate{
		CurrentTime: float64(session.PositionSec),
		Duration:    float64(session.DurationSec),
		Progress:    progress,
	}

	// Sync to ABS
	if err := client.UpdateProgress(ctx, session.ItemID, update); err != nil {
		slog.Warn("failed to sync progress to ABS after sleep timer",
			"session_id", session.SessionID,
			"item_id", session.ItemID,
			"error", err,
		)
		return
	}

	// Update sync timestamp
	w.playbackStore.UpdateABSSyncTime(session.ID)

	slog.Debug("synced progress to ABS after sleep timer",
		"item_id", session.ItemID,
		"position_sec", session.PositionSec,
		"progress", progress,
	)
}
