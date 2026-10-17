package web

import (
	"context"
	"fmt"
	"log/slog"

	"audiobookshelf-sonos-bridge/internal/cache"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
	"audiobookshelf-sonos-bridge/internal/stream"
)

// SegmentTransitioner switches Sonos to the next segment of a segmented
// item. It is shared by the status endpoint and the background progress
// syncer so that segment changes also happen when no browser tab is polling.
type SegmentTransitioner struct {
	cacheIndex    *cache.Index
	tokenGen      *stream.TokenGenerator
	publicURL     string
	playbackStore *store.PlaybackStore
	sessionStore  *store.SessionStore
	deviceStore   *store.DeviceStore
	auth          *AuthHandler
}

// NewSegmentTransitioner creates a SegmentTransitioner.
func NewSegmentTransitioner(
	cacheIndex *cache.Index,
	tokenGen *stream.TokenGenerator,
	publicURL string,
	playbackStore *store.PlaybackStore,
	sessionStore *store.SessionStore,
	deviceStore *store.DeviceStore,
	auth *AuthHandler,
) *SegmentTransitioner {
	return &SegmentTransitioner{
		cacheIndex:    cacheIndex,
		tokenGen:      tokenGen,
		publicURL:     publicURL,
		playbackStore: playbackStore,
		sessionStore:  sessionStore,
		deviceStore:   deviceStore,
		auth:          auth,
	}
}

// HasNextSegment reports whether the playback has a segment after the
// current one.
func (t *SegmentTransitioner) HasNextSegment(playback *store.PlaybackSession) bool {
	entry, err := t.cacheIndex.GetEntry(playback.ItemID)
	if err != nil || entry == nil {
		return false
	}
	return playback.CurrentSegment+1 < entry.SegmentCount
}

// Transition starts the next segment on the given device. It returns
// (false, nil) if there is no next segment or another caller already
// advanced the playback.
func (t *SegmentTransitioner) Transition(ctx context.Context, playback *store.PlaybackSession, device *store.SonosDevice) (bool, error) {
	// Re-read the row so that concurrent callers (status poll and background
	// syncer) do not both advance the same segment.
	fresh, err := t.playbackStore.Get(playback.ID)
	if err != nil {
		return false, fmt.Errorf("reload playback: %w", err)
	}
	if fresh == nil || fresh.CurrentSegment != playback.CurrentSegment {
		return false, nil
	}
	playback = fresh

	cacheEntry, err := t.cacheIndex.GetEntry(playback.ItemID)
	if err != nil || cacheEntry == nil {
		return false, fmt.Errorf("cache entry for segment transition: %v", err)
	}

	nextSegment := playback.CurrentSegment + 1
	if nextSegment >= cacheEntry.SegmentCount {
		return false, nil
	}

	session, err := t.sessionStore.Get(playback.SessionID)
	if err != nil || session == nil {
		return false, fmt.Errorf("web session %s not found: %v", playback.SessionID, err)
	}

	slog.Info("transitioning to next segment",
		"item_id", playback.ItemID,
		"from_segment", playback.CurrentSegment,
		"to_segment", nextSegment,
		"segment_count", cacheEntry.SegmentCount)

	ext := store.SegmentExt(cacheEntry.CacheFormat)
	streamToken := playback.StreamToken
	if newToken, genErr := t.tokenGen.Generate(playback.ItemID, session.UserID, session.ID); genErr != nil {
		slog.Warn("failed to refresh stream token for segment transition, using stored token", "error", genErr)
	} else {
		streamToken = newToken
		if err := t.playbackStore.UpdateStreamToken(playback.ID, newToken); err != nil {
			slog.Warn("failed to persist refreshed stream token", "error", err)
		}
	}
	nextStreamURL := fmt.Sprintf("%s/stream/%s/segment_%03d%s", t.publicURL, streamToken, nextSegment, ext)

	// DIDL metadata (optional; playback continues without it)
	var metadata string
	if absClient, cErr := t.auth.GetABSClientForSession(session); cErr != nil {
		slog.Warn("failed to get ABS client for segment transition", "error", cErr)
	} else if item, iErr := absClient.GetItem(ctx, playback.ItemID); iErr == nil && item != nil {
		metadata = buildDIDLMetadata(item, nextStreamURL, cache.GetContentType(cacheEntry.CacheFormat))
	}

	avt := sonos.NewAVTransport(coordinatorIP(ctx, device.IPAddress))
	if err := avt.SetAVTransportURI(ctx, nextStreamURL, metadata); err != nil {
		return false, fmt.Errorf("set next segment URI: %w", err)
	}
	if err := avt.Play(ctx); err != nil {
		return false, fmt.Errorf("start next segment: %w", err)
	}

	if err := t.playbackStore.UpdateCurrentSegment(playback.ID, nextSegment); err != nil {
		return true, fmt.Errorf("persist segment: %w", err)
	}

	slog.Info("segment transition complete", "item_id", playback.ItemID, "new_segment", nextSegment)
	return true, nil
}
