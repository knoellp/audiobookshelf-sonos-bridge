package web

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
)

// TokenDecrypter decrypts ABS tokens from session storage.
type TokenDecrypter interface {
	DecryptToken(encrypted []byte) (string, error)
}

// sessionClientProvider is implemented by decrypters that can build an ABS
// client with automatic re-login (AuthHandler).
type sessionClientProvider interface {
	ClientForSession(session *store.Session) (*abs.Client, error)
}

// clientForSession builds an ABS client for a session, preferring the
// re-login-capable provider and falling back to plain token decryption.
func clientForSession(dec TokenDecrypter, base *abs.Client, session *store.Session) (*abs.Client, error) {
	if p, ok := dec.(sessionClientProvider); ok {
		return p.ClientForSession(session)
	}
	token, err := dec.DecryptToken(session.ABSTokenEnc)
	if err != nil {
		return nil, err
	}
	return base.WithToken(token), nil
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
	publicURL     string
	segments      *SegmentTransitioner

	// Backwards guard (see syncSession): last position PATCHed to ABS per
	// playback row, and rows for which a user seek/resume allows the next
	// sync to move backwards (value = time the permission expires).
	mu             sync.Mutex
	lastSynced     map[string]int
	allowBackwards map[string]time.Time
}

// backwardsGrace is how long after a seek/resume a backwards sync is allowed.
const backwardsGrace = 2 * time.Minute

// NewProgressSyncer creates a new progress syncer.
func NewProgressSyncer(
	absClient *abs.Client,
	playbackStore *store.PlaybackStore,
	sessionStore *store.SessionStore,
	deviceStore *store.DeviceStore,
	tokenDecrypt TokenDecrypter,
	publicURL string,
	segments *SegmentTransitioner,
) *ProgressSyncer {
	return &ProgressSyncer{
		absClient:     absClient,
		playbackStore: playbackStore,
		sessionStore:  sessionStore,
		deviceStore:   deviceStore,
		tokenDecrypt:  tokenDecrypt,
		pollInterval:  5 * time.Second,
		syncInterval:  30 * time.Second,
		publicURL:     publicURL,
		segments:      segments,

		lastSynced:     make(map[string]int),
		allowBackwards: make(map[string]time.Time),
	}
}

// AllowBackwards records that the user explicitly moved the position (seek,
// resume), so the next sync may write a position lower than the last one.
func (s *ProgressSyncer) AllowBackwards(playbackID string) {
	s.mu.Lock()
	s.allowBackwards[playbackID] = time.Now().Add(backwardsGrace)
	s.mu.Unlock()
}

// Forget drops all in-memory sync state of a deleted playback row.
func (s *ProgressSyncer) Forget(playbackID string) {
	s.mu.Lock()
	delete(s.lastSynced, playbackID)
	delete(s.allowBackwards, playbackID)
	s.mu.Unlock()
}

// permitSync decides whether position may be PATCHed to ABS. A position below
// the last synced one is only allowed shortly after an explicit seek/resume;
// otherwise it is most likely a stale or reset value and would destroy
// progress in ABS.
func (s *ProgressSyncer) permitSync(playbackID string, position int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last, ok := s.lastSynced[playbackID]
	if !ok || position >= last {
		return true
	}
	until, allowed := s.allowBackwards[playbackID]
	if allowed && time.Now().Before(until) {
		delete(s.allowBackwards, playbackID)
		return true
	}
	return false
}

func (s *ProgressSyncer) recordSynced(playbackID string, position int) {
	s.mu.Lock()
	s.lastSynced[playbackID] = position
	s.mu.Unlock()
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
	// Final sync so that the position of the last 30 s is not lost.
	if s.cancel != nil && s.playbackStore != nil {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.syncAllActive(syncCtx)
		syncCancel()
	}

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
	// Group members do not report the coordinator's transport state
	avt := sonos.NewAVTransport(coordinatorIP(ctx, device.IPAddress))
	posInfo, err := avt.GetPositionInfo(ctx)
	if err != nil {
		slog.Debug("failed to get position info",
			"session_id", playback.SessionID,
			"error", err,
		)
		return
	}

	// Sonos may be playing something else (radio, another app); that must
	// never be written to ABS as audiobook progress.
	if !isBridgeStream(posInfo.TrackURI, s.publicURL) {
		slog.Info("sonos is not playing a bridge stream, marking playback stopped",
			"session_id", playback.SessionID,
			"track_uri", posInfo.TrackURI,
		)
		s.playbackStore.UpdatePlaying(playback.ID, false)
		return
	}

	// Parse position
	relTime := sonos.ParseDuration(posInfo.RelTime)
	positionSec := int(relTime.Seconds())
	trackDuration := sonos.ParseDuration(posInfo.TrackDuration)

	// Near the end of a segment: switch to the next one, independent of any
	// open browser tab.
	if playback.SegmentDurationSec > 0 && s.segments != nil && trackDuration > 0 &&
		relTime >= trackDuration-10*time.Second && s.segments.HasNextSegment(playback) {
		if _, err := s.segments.Transition(ctx, playback, device); err != nil {
			slog.Warn("segment transition failed",
				"session_id", playback.SessionID,
				"item_id", playback.ItemID,
				"error", err,
			)
		}
		return
	}

	// Track (or last segment) has ended
	if trackDuration > 0 && relTime >= trackDuration-time.Second {
		slog.Info("playback ended",
			"session_id", playback.SessionID,
			"item_id", playback.ItemID,
		)
		// Final position update and sync before marking as stopped
		endPos := globalPosition(playback, int(trackDuration.Seconds()))
		s.playbackStore.UpdatePosition(playback.ID, endPos)
		playback.PositionSec = endPos
		s.syncSession(ctx, playback)
		s.playbackStore.UpdatePlaying(playback.ID, false)
		return
	}

	// Skip if Sonos is in TRANSITIONING state (both position and duration are zero)
	if positionSec == 0 && trackDuration == 0 {
		return
	}
	// Don't overwrite a valid stored position with zero (device stopped/resetting)
	if positionSec == 0 && playback.PositionSec > 10 {
		return
	}

	// For segmented playback, convert local segment position to global position
	globalPosSec := globalPosition(playback, positionSec)

	// Update stored position
	if globalPosSec != playback.PositionSec {
		s.playbackStore.UpdatePosition(playback.ID, globalPosSec)
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
	if !s.permitSync(playback.ID, playback.PositionSec) {
		slog.Warn("skipping ABS sync: position is behind last synced position and no seek happened",
			"session_id", playback.SessionID,
			"item_id", playback.ItemID,
			"position_sec", playback.PositionSec,
		)
		return
	}

	// Get user session for token
	session, err := s.sessionStore.Get(playback.SessionID)
	if err != nil || session == nil {
		slog.Warn("session not found for playback", "session_id", playback.SessionID)
		return
	}

	client, err := clientForSession(s.tokenDecrypt, s.absClient, session)
	if err != nil {
		slog.Warn("failed to create ABS client", "session_id", playback.SessionID, "error", err)
		return
	}

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

	s.recordSynced(playback.ID, playback.PositionSec)

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
