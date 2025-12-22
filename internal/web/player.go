package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/cache"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
	"audiobookshelf-sonos-bridge/internal/stream"
)

// PathMapper maps ABS paths to local filesystem paths.
type PathMapper func(absPath string) string

// PlayerHandler handles playback-related requests.
type PlayerHandler struct {
	authHandler   *AuthHandler
	cacheIndex    *cache.Index
	cacheWorker   *cache.Worker
	tokenGen      *stream.TokenGenerator
	publicURL     string
	templates     *template.Template
	sonosStore    *store.DeviceStore
	playbackStore *store.PlaybackStore
	pathMapper    PathMapper
}

// NewPlayerHandler creates a new player handler.
func NewPlayerHandler(
	authHandler *AuthHandler,
	cacheIndex *cache.Index,
	cacheWorker *cache.Worker,
	tokenGen *stream.TokenGenerator,
	publicURL string,
	templates *template.Template,
	sonosStore *store.DeviceStore,
	playbackStore *store.PlaybackStore,
	pathMapper PathMapper,
) *PlayerHandler {
	return &PlayerHandler{
		authHandler:   authHandler,
		cacheIndex:    cacheIndex,
		cacheWorker:   cacheWorker,
		tokenGen:      tokenGen,
		publicURL:     publicURL,
		templates:     templates,
		sonosStore:    sonosStore,
		playbackStore: playbackStore,
		pathMapper:    pathMapper,
	}
}

// HandlePlay handles POST /play requests to start playback on Sonos.
func (h *PlayerHandler) HandlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Log request details for debugging
	slog.Debug("play request headers", "content-type", r.Header.Get("Content-Type"))

	// Parse form - try multipart first, then regular form
	contentType := r.Header.Get("Content-Type")
	var itemID, sonosUUID string

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 10); err != nil {
			slog.Error("failed to parse multipart form", "error", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		itemID = r.FormValue("item_id")
		sonosUUID = r.FormValue("sonos_uuid")
	} else {
		if err := r.ParseForm(); err != nil {
			slog.Error("failed to parse form", "error", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		itemID = r.FormValue("item_id")
		sonosUUID = r.FormValue("sonos_uuid")
	}

	slog.Debug("play request received", "item_id", itemID, "sonos_uuid", sonosUUID, "form_values", r.Form)

	if itemID == "" || sonosUUID == "" {
		slog.Warn("play request missing parameters", "item_id", itemID, "sonos_uuid", sonosUUID)
		http.Error(w, "item_id and sonos_uuid required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get authenticated ABS client for the session
	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		slog.Error("failed to get ABS client for session", "error", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	// Get item details from ABS
	item, err := absClient.GetItem(ctx, itemID)
	if err != nil {
		slog.Error("failed to get item from ABS", "item_id", itemID, "error", err)
		http.Error(w, "failed to get item", http.StatusInternalServerError)
		return
	}

	// Get all audio files and map their paths
	if len(item.Media.AudioFiles) == 0 {
		slog.Error("no audio files in item", "item_id", itemID)
		http.Error(w, "no audio files", http.StatusBadRequest)
		return
	}

	// Sort audio files by index to ensure correct order
	audioFiles := make([]abs.AudioFile, len(item.Media.AudioFiles))
	copy(audioFiles, item.Media.AudioFiles)
	sort.Slice(audioFiles, func(i, j int) bool {
		return audioFiles[i].Index < audioFiles[j].Index
	})
	sourcePaths := make([]string, 0, len(audioFiles))
	for _, af := range audioFiles {
		absPath := af.Metadata.Path
		localPath := h.pathMapper(absPath)
		sourcePaths = append(sourcePaths, localPath)
	}
	slog.Debug("audio files mapped", "item_id", itemID, "file_count", len(sourcePaths))

	// Check cache status
	slog.Debug("checking cache status", "item_id", itemID)
	cached, err := h.cacheIndex.IsCached(itemID)
	if err != nil {
		slog.Error("failed to check cache status", "item_id", itemID, "error", err)
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}
	slog.Debug("cache status checked", "item_id", itemID, "cached", cached)

	if !cached {
		// Start on-demand transcoding
		entry, _ := h.cacheIndex.GetEntry(itemID)

		if entry == nil {
			// Create new entry (use first path for backwards compatibility)
			if err := h.cacheIndex.CreateEntry(itemID, sourcePaths[0], 0, time.Now()); err != nil {
				slog.Error("failed to create cache entry", "item_id", itemID, "error", err)
				http.Error(w, "cache error", http.StatusInternalServerError)
				return
			}
		}

		// Do synchronous transcoding for immediate playback (with all files)
		if err := h.cacheWorker.TranscodeSyncMultiple(ctx, itemID, sourcePaths); err != nil {
			slog.Error("transcoding failed", "item_id", itemID, "error", err)
			http.Error(w, "transcoding failed", http.StatusInternalServerError)
			return
		}
	}

	// Get cache entry to determine the correct file format
	cacheEntry, err := h.cacheIndex.GetEntry(itemID)
	if err != nil || cacheEntry == nil {
		slog.Error("failed to get cache entry for format", "item_id", itemID, "error", err)
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}

	// Generate stream token
	slog.Debug("generating stream token", "item_id", itemID)
	token, err := h.tokenGen.Generate(itemID, session.UserID, session.ID)
	if err != nil {
		slog.Error("failed to generate stream token", "error", err)
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	// Get saved progress from ABS (need this early for segment calculation)
	progress, _ := absClient.GetProgress(ctx, itemID)
	startPositionSec := 0
	if progress != nil && progress.CurrentTime > 0 {
		startPositionSec = int(progress.CurrentTime)
		slog.Info("resuming from saved position", "item_id", itemID, "position_sec", startPositionSec)
	}

	// Build stream URL - handle segmented vs non-segmented
	var streamURL string
	var currentSegment int
	var segmentDurationSec int

	if cacheEntry.IsSegmented() {
		// Calculate which segment to start with based on position
		currentSegment, _ = store.GlobalToSegment(startPositionSec, cacheEntry.SegmentDurationSec)
		segmentDurationSec = cacheEntry.SegmentDurationSec

		// Build segment URL
		ext := ".m4a"
		switch cacheEntry.CacheFormat {
		case "mp3":
			ext = ".mp3"
		case "flac":
			ext = ".flac"
		}
		streamURL = fmt.Sprintf("%s/stream/%s/segment_%03d%s", h.publicURL, token, currentSegment, ext)
		slog.Debug("segmented stream URL generated",
			"url", streamURL,
			"segment", currentSegment,
			"segment_count", cacheEntry.SegmentCount,
			"format", cacheEntry.CacheFormat)
	} else {
		// Standard single-file URL
		cacheFileName := cache.GetCacheFileName(cacheEntry.CacheFormat)
		streamURL = fmt.Sprintf("%s/stream/%s/%s", h.publicURL, token, cacheFileName)
		slog.Debug("stream URL generated", "url", streamURL, "format", cacheEntry.CacheFormat)
	}

	// Get Sonos device
	slog.Debug("getting Sonos device", "uuid", sonosUUID)
	device, err := h.sonosStore.Get(sonosUUID)
	if err != nil || device == nil {
		slog.Error("failed to get Sonos device", "uuid", sonosUUID, "error", err)
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	// Create AVTransport client using device IP
	avt := sonos.NewAVTransport(device.IPAddress)

	// Build DIDL-Lite metadata with correct MIME type
	mimeType := cache.GetContentType(cacheEntry.CacheFormat)
	metadata := buildDIDLMetadata(item, streamURL, mimeType)
	slog.Debug("DIDL metadata built", "mime_type", mimeType)

	// Set AV Transport URI
	if err := avt.SetAVTransportURI(ctx, streamURL, metadata); err != nil {
		slog.Error("failed to set transport URI", "error", err)
		http.Error(w, "failed to set URI on Sonos", http.StatusInternalServerError)
		return
	}

	// Start playback
	if err := avt.Play(ctx); err != nil {
		slog.Error("failed to start playback", "error", err)
		http.Error(w, "failed to start playback", http.StatusInternalServerError)
		return
	}

	// Seek to saved position if needed
	if startPositionSec > 0 {
		// For segmented playback, seek to local position within the segment
		var seekPosition int
		if cacheEntry.IsSegmented() {
			_, seekPosition = store.GlobalToSegment(startPositionSec, cacheEntry.SegmentDurationSec)
			slog.Debug("seeking to local position in segment",
				"global_position", startPositionSec,
				"segment", currentSegment,
				"local_position", seekPosition)
		} else {
			seekPosition = startPositionSec
		}

		// Small delay to ensure playback has started
		time.Sleep(500 * time.Millisecond)
		if err := avt.Seek(ctx, time.Duration(seekPosition)*time.Second); err != nil {
			slog.Warn("failed to seek to saved position", "error", err)
			// Non-fatal, continue with playback
		}
	}

	// Calculate total duration (use Media.Duration, or sum audio files if 0)
	totalDuration := item.Media.Duration
	if totalDuration == 0 && len(item.Media.AudioFiles) > 0 {
		for _, af := range item.Media.AudioFiles {
			totalDuration += af.Duration
		}
	}

	// Create/update playback session
	playbackSession := &store.PlaybackSession{
		ID:                 generateID(),
		SessionID:          session.ID,
		ItemID:             itemID,
		SonosUUID:          sonosUUID,
		StreamToken:        token,
		IsPlaying:          true,
		PositionSec:        startPositionSec,
		DurationSec:        int(totalDuration),
		CurrentSegment:     currentSegment,
		SegmentDurationSec: segmentDurationSec,
		StartedAt:          time.Now(),
		LastPositionUpdate: time.Now(),
	}

	if err := h.playbackStore.Create(playbackSession); err != nil {
		slog.Warn("failed to save playback session", "error", err)
	}

	slog.Info("playback started",
		"item_id", itemID,
		"sonos_uuid", sonosUUID,
		"user_id", session.UserID,
		"duration_sec", playbackSession.DurationSec,
		"audio_files", len(item.Media.AudioFiles),
		"current_segment", currentSegment,
		"segmented", cacheEntry.IsSegmented(),
	)

	// Return JSON with redirect URL (HX-Redirect header not accessible from JS fetch due to CORS)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Redirect", "/player/"+itemID) // Keep for htmx compatibility
	json.NewEncoder(w).Encode(map[string]string{
		"redirect": "/player/" + itemID,
	})
}

// HandlePause handles POST /transport/pause requests.
func (h *PlayerHandler) HandlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		http.Error(w, "no active playback", http.StatusNotFound)
		return
	}

	// Get Sonos device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	avt := sonos.NewAVTransport(device.IPAddress)

	// Get current position BEFORE pausing (most accurate)
	posInfo, _ := avt.GetPositionInfo(ctx)
	if posInfo != nil {
		localPos := int(sonos.ParseDuration(posInfo.RelTime).Seconds())

		// Calculate global position for segmented playback
		var globalPos int
		if playback.SegmentDurationSec > 0 {
			globalPos = store.SegmentToGlobal(playback.CurrentSegment, localPos, playback.SegmentDurationSec)
		} else {
			globalPos = localPos
		}

		playback.PositionSec = globalPos
		h.playbackStore.UpdatePosition(playback.ID, globalPos)
	}

	// Pause on Sonos
	if err := avt.Pause(ctx); err != nil {
		// Error 701 = "Transition not available" - device is already paused/stopped
		if strings.Contains(err.Error(), "errorCode>701") {
			slog.Debug("pause not needed - device already paused/stopped")
		} else {
			slog.Error("failed to pause", "error", err)
			http.Error(w, "failed to pause", http.StatusInternalServerError)
			return
		}
	}

	// Update status
	h.playbackStore.UpdatePlaying(playback.ID, false)

	// Sync progress to ABS immediately (like HandleStop does)
	if playback.PositionSec > 0 && playback.DurationSec > 0 {
		absClient, err := h.authHandler.GetABSClientForSession(session)
		if err == nil {
			progress := abs.ProgressUpdate{
				CurrentTime: float64(playback.PositionSec),
				Duration:    float64(playback.DurationSec),
				Progress:    float64(playback.PositionSec) / float64(playback.DurationSec),
			}
			if err := absClient.UpdateProgress(ctx, playback.ItemID, progress); err != nil {
				slog.Warn("failed to sync progress to ABS on pause", "error", err)
			} else {
				slog.Debug("synced progress to ABS on pause",
					"item_id", playback.ItemID,
					"position_sec", playback.PositionSec,
					"progress_pct", int(progress.Progress*100),
				)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// HandleResume handles POST /transport/resume requests.
func (h *PlayerHandler) HandleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Get optional new sonos_uuid for player switching
	newSonosUUID := r.FormValue("sonos_uuid")

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		http.Error(w, "no active playback", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	// Check if player is being switched
	if newSonosUUID != "" && newSonosUUID != playback.SonosUUID {
		slog.Info("switching Sonos player",
			"old_uuid", playback.SonosUUID,
			"new_uuid", newSonosUUID,
			"item_id", playback.ItemID,
			"position_sec", playback.PositionSec,
		)

		// Stop on old device first
		oldDevice, err := h.sonosStore.Get(playback.SonosUUID)
		if err == nil && oldDevice != nil {
			oldAVT := sonos.NewAVTransport(oldDevice.IPAddress)
			if err := oldAVT.Stop(ctx); err != nil {
				slog.Debug("failed to stop old device (may already be stopped)", "error", err)
			} else {
				slog.Debug("stopped old device", "device", oldDevice.Name)
			}
		} else {
			slog.Warn("old device not found", "uuid", playback.SonosUUID, "error", err)
		}

		// Get new device
		newDevice, err := h.sonosStore.Get(newSonosUUID)
		if err != nil || newDevice == nil {
			slog.Error("new device not found", "uuid", newSonosUUID, "error", err)
			http.Error(w, "new device not found", http.StatusNotFound)
			return
		}
		slog.Debug("found new device", "name", newDevice.Name, "ip", newDevice.IPAddress)

		// Get cache entry for stream URL
		cacheEntry, err := h.cacheIndex.GetEntry(playback.ItemID)
		if err != nil || cacheEntry == nil {
			slog.Error("cache entry not found for player switch", "item_id", playback.ItemID, "error", err)
			http.Error(w, "cache entry not found", http.StatusNotFound)
			return
		}

		// Generate fresh stream token to avoid expiration issues
		newToken, err := h.tokenGen.Generate(playback.ItemID, session.UserID, session.ID)
		if err != nil {
			slog.Error("failed to generate new stream token", "error", err)
			http.Error(w, "token generation failed", http.StatusInternalServerError)
			return
		}
		slog.Debug("generated new stream token for player switch")

		// Build stream URL with fresh token
		// For segmented files, use the correct segment based on current position
		var streamURL string
		var localSeekPos int // Position to seek within the segment
		if cacheEntry.IsSegmented() {
			// Calculate which segment the current position falls into
			segmentDuration := cacheEntry.SegmentDurationSec
			if segmentDuration <= 0 {
				segmentDuration = 7200 // Default 2 hours
			}
			currentSegment := playback.CurrentSegment
			// Calculate local position within the segment
			localSeekPos = playback.PositionSec - (currentSegment * segmentDuration)
			if localSeekPos < 0 {
				localSeekPos = 0
			}
			// Build segment URL
			ext := ".m4a"
			switch cacheEntry.CacheFormat {
			case "mp3":
				ext = ".mp3"
			case "flac":
				ext = ".flac"
			}
			streamURL = fmt.Sprintf("%s/stream/%s/segment_%03d%s", h.publicURL, newToken, currentSegment, ext)
			slog.Debug("player switch using segmented stream",
				"segment", currentSegment,
				"global_position", playback.PositionSec,
				"local_seek_pos", localSeekPos,
				"stream_url", streamURL)
		} else {
			cacheFileName := cache.GetCacheFileName(cacheEntry.CacheFormat)
			streamURL = fmt.Sprintf("%s/stream/%s/%s", h.publicURL, newToken, cacheFileName)
			localSeekPos = playback.PositionSec
		}

		// Get item for metadata
		absClient, err := h.authHandler.GetABSClientForSession(session)
		if err != nil {
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		item, err := absClient.GetItem(ctx, playback.ItemID)
		if err != nil {
			http.Error(w, "failed to get item", http.StatusInternalServerError)
			return
		}

		// Build DIDL metadata
		mimeType := cache.GetContentType(cacheEntry.CacheFormat)
		metadata := buildDIDLMetadata(item, streamURL, mimeType)

		// Set URI on new device
		newAVT := sonos.NewAVTransport(newDevice.IPAddress)
		slog.Debug("setting transport URI on new device", "url", streamURL)
		if err := newAVT.SetAVTransportURI(ctx, streamURL, metadata); err != nil {
			slog.Error("failed to set transport URI on new device", "device", newDevice.Name, "error", err)
			http.Error(w, "failed to set URI on new Sonos", http.StatusInternalServerError)
			return
		}

		// Start playback on new device
		slog.Debug("starting playback on new device", "device", newDevice.Name)
		if err := newAVT.Play(ctx); err != nil {
			slog.Error("failed to start playback on new device", "device", newDevice.Name, "error", err)
			http.Error(w, "failed to start playback", http.StatusInternalServerError)
			return
		}

		// Seek to current position (use local position for segmented files)
		if localSeekPos > 0 {
			time.Sleep(500 * time.Millisecond)
			slog.Debug("seeking to position on new device", "local_seek_pos", localSeekPos, "global_pos", playback.PositionSec)
			if err := newAVT.Seek(ctx, time.Duration(localSeekPos)*time.Second); err != nil {
				slog.Warn("failed to seek on new device", "error", err)
			}
		}

		// Update playback session with new device and new token
		h.playbackStore.UpdateSonosUUID(playback.ID, newSonosUUID)
		h.playbackStore.UpdatePlaying(playback.ID, true)
		// Update stream token in database
		if err := h.playbackStore.UpdateStreamToken(playback.ID, newToken); err != nil {
			slog.Warn("failed to update stream token in database", "error", err)
		}

		slog.Info("player switch completed successfully",
			"new_device", newDevice.Name,
			"item_id", playback.ItemID,
			"position_sec", playback.PositionSec,
		)

		w.WriteHeader(http.StatusOK)
		return
	}

	// Normal resume on same device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	avt := sonos.NewAVTransport(device.IPAddress)

	// Fetch latest progress from ABS (single source of truth)
	var absPositionSec int
	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err == nil {
		progress, err := absClient.GetProgress(ctx, playback.ItemID)
		if err == nil && progress != nil {
			absPositionSec = int(progress.CurrentTime)
			slog.Debug("fetched ABS progress on resume",
				"item_id", playback.ItemID,
				"abs_position_sec", absPositionSec,
				"local_position_sec", playback.PositionSec,
			)
		}
	}

	// Check if ABS has a different position (user listened elsewhere)
	needsSeek := false
	var targetPosition int
	if absPositionSec > 0 {
		diff := absPositionSec - playback.PositionSec
		if diff < 0 {
			diff = -diff
		}
		// If difference is more than 5 seconds, use ABS position
		if diff > 5 {
			needsSeek = true
			targetPosition = absPositionSec
			slog.Info("position changed in ABS, will seek after resume",
				"item_id", playback.ItemID,
				"local_position", playback.PositionSec,
				"abs_position", absPositionSec,
				"diff_sec", diff,
			)
			// Update local position
			h.playbackStore.UpdatePosition(playback.ID, absPositionSec)
		}
	}

	if err := avt.Play(ctx); err != nil {
		// Error 701 = "Transition not available" - device may already be playing
		if strings.Contains(err.Error(), "errorCode>701") {
			slog.Debug("resume not needed - transition not available")
		} else {
			slog.Error("failed to resume", "error", err)
			http.Error(w, "failed to resume", http.StatusInternalServerError)
			return
		}
	}

	// Seek to ABS position if it changed
	if needsSeek {
		// For segmented playback, calculate local position within segment
		var seekPosition int
		if playback.SegmentDurationSec > 0 {
			_, seekPosition = store.GlobalToSegment(targetPosition, playback.SegmentDurationSec)
		} else {
			seekPosition = targetPosition
		}

		time.Sleep(300 * time.Millisecond) // Brief delay for playback to start
		if err := avt.Seek(ctx, time.Duration(seekPosition)*time.Second); err != nil {
			slog.Warn("failed to seek to ABS position on resume", "error", err)
		} else {
			slog.Debug("seeked to ABS position on resume", "position_sec", seekPosition)
		}
	}

	// Update status
	h.playbackStore.UpdatePlaying(playback.ID, true)

	w.WriteHeader(http.StatusOK)
}

// HandleSeek handles POST /transport/seek requests.
func (h *PlayerHandler) HandleSeek(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Get offset in seconds (can be negative for skip back)
	offsetStr := r.FormValue("offset")
	positionStr := r.FormValue("position")

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		http.Error(w, "no active playback", http.StatusNotFound)
		return
	}

	// Get Sonos device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	avt := sonos.NewAVTransport(device.IPAddress)
	ctx := r.Context()

	var targetGlobalPositionSec int

	if positionStr != "" {
		// Absolute position in seconds (this is a GLOBAL position)
		posSec, err := strconv.Atoi(positionStr)
		if err != nil {
			http.Error(w, "invalid position", http.StatusBadRequest)
			return
		}
		targetGlobalPositionSec = posSec
	} else if offsetStr != "" {
		// Relative offset
		offsetSec, err := strconv.Atoi(offsetStr)
		if err != nil {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}

		// Get current local position
		posInfo, err := avt.GetPositionInfo(ctx)
		if err != nil {
			slog.Error("failed to get position", "error", err)
			http.Error(w, "failed to get position", http.StatusInternalServerError)
			return
		}

		localPos := int(sonos.ParseDuration(posInfo.RelTime).Seconds())

		// Convert to global position and apply offset
		if playback.SegmentDurationSec > 0 {
			currentGlobalPos := store.SegmentToGlobal(playback.CurrentSegment, localPos, playback.SegmentDurationSec)
			targetGlobalPositionSec = currentGlobalPos + offsetSec
		} else {
			targetGlobalPositionSec = localPos + offsetSec
		}

		if targetGlobalPositionSec < 0 {
			targetGlobalPositionSec = 0
		}
		if targetGlobalPositionSec > playback.DurationSec {
			targetGlobalPositionSec = playback.DurationSec
		}
	} else {
		http.Error(w, "offset or position required", http.StatusBadRequest)
		return
	}

	// Handle segmented playback - check if we need to switch segments
	if playback.SegmentDurationSec > 0 {
		targetSegment, localPosition := store.GlobalToSegment(targetGlobalPositionSec, playback.SegmentDurationSec)

		if targetSegment != playback.CurrentSegment {
			// Need to switch to a different segment
			slog.Info("seek requires segment change",
				"from_segment", playback.CurrentSegment,
				"to_segment", targetSegment,
				"global_position", targetGlobalPositionSec,
				"local_position", localPosition)

			// Get cache entry for segment info
			cacheEntry, err := h.cacheIndex.GetEntry(playback.ItemID)
			if err != nil || cacheEntry == nil {
				slog.Error("failed to get cache entry for seek", "error", err)
				http.Error(w, "cache error", http.StatusInternalServerError)
				return
			}

			// Validate segment index
			if targetSegment >= cacheEntry.SegmentCount {
				targetSegment = cacheEntry.SegmentCount - 1
				// Recalculate local position for last segment
				localPosition = playback.DurationSec - (targetSegment * playback.SegmentDurationSec)
			}

			// Build URL for target segment
			ext := ".m4a"
			switch cacheEntry.CacheFormat {
			case "mp3":
				ext = ".mp3"
			case "flac":
				ext = ".flac"
			}
			segmentURL := fmt.Sprintf("%s/stream/%s/segment_%03d%s", h.publicURL, playback.StreamToken, targetSegment, ext)

			// Get item for metadata
			absClient, _ := h.authHandler.GetABSClientForSession(session)
			var metadata string
			if absClient != nil {
				item, err := absClient.GetItem(ctx, playback.ItemID)
				if err == nil && item != nil {
					mimeType := cache.GetContentType(cacheEntry.CacheFormat)
					metadata = buildDIDLMetadata(item, segmentURL, mimeType)
				}
			}

			// Set new segment URI
			if err := avt.SetAVTransportURI(ctx, segmentURL, metadata); err != nil {
				slog.Error("failed to set segment URI for seek", "error", err)
				http.Error(w, "failed to seek", http.StatusInternalServerError)
				return
			}

			// Start playback and seek
			if err := avt.Play(ctx); err != nil {
				slog.Error("failed to start after segment change", "error", err)
				http.Error(w, "failed to seek", http.StatusInternalServerError)
				return
			}

			// Wait briefly for playback to start
			time.Sleep(300 * time.Millisecond)

			// Seek within the new segment
			if localPosition > 0 {
				if err := avt.Seek(ctx, time.Duration(localPosition)*time.Second); err != nil {
					slog.Warn("failed to seek within segment", "error", err)
				}
			}

			// Update segment in playback session
			h.playbackStore.UpdatePositionAndSegment(playback.ID, targetGlobalPositionSec, targetSegment)
		} else {
			// Same segment - just seek locally
			if err := avt.Seek(ctx, time.Duration(localPosition)*time.Second); err != nil {
				slog.Error("failed to seek", "error", err)
				http.Error(w, "failed to seek", http.StatusInternalServerError)
				return
			}
			h.playbackStore.UpdatePosition(playback.ID, targetGlobalPositionSec)
		}
	} else {
		// Non-segmented playback - simple seek
		if err := avt.Seek(ctx, time.Duration(targetGlobalPositionSec)*time.Second); err != nil {
			slog.Error("failed to seek", "error", err)
			http.Error(w, "failed to seek", http.StatusInternalServerError)
			return
		}
		h.playbackStore.UpdatePosition(playback.ID, targetGlobalPositionSec)
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStatus handles GET /status requests.
func (h *PlayerHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active": false,
		})
		return
	}

	// Get Sonos device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	avt := sonos.NewAVTransport(device.IPAddress)

	// Get current transport state from Sonos
	transportInfo, err := avt.GetTransportInfo(r.Context())
	isPlaying := playback.IsPlaying // fallback to stored value
	if err == nil {
		isPlaying = transportInfo.CurrentTransportState == sonos.TransportStatePlaying
		// Update stored state if it changed
		if isPlaying != playback.IsPlaying {
			h.playbackStore.UpdatePlaying(playback.ID, isPlaying)
		}
	}

	// Get current position
	posInfo, err := avt.GetPositionInfo(r.Context())
	if err != nil {
		slog.Warn("failed to get position info", "error", err)
		// Return stored values
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active":       true,
			"item_id":      playback.ItemID,
			"is_playing":   isPlaying,
			"position_sec": playback.PositionSec,
			"duration_sec": playback.DurationSec,
		})
		return
	}

	// Parse position from string (this is the LOCAL position within current segment)
	relTime := sonos.ParseDuration(posInfo.RelTime)
	trackDuration := sonos.ParseDuration(posInfo.TrackDuration)
	localPositionSec := int(relTime.Seconds())

	// Calculate global position (across all segments)
	var globalPositionSec int
	if playback.SegmentDurationSec > 0 {
		// Segmented playback: global = segment * segment_duration + local
		globalPositionSec = store.SegmentToGlobal(playback.CurrentSegment, localPositionSec, playback.SegmentDurationSec)

		// Check if we're near segment end and need to switch to next segment
		segmentEndThreshold := playback.SegmentDurationSec - 5 // 5 seconds before end
		if isPlaying && localPositionSec >= segmentEndThreshold {
			h.handleSegmentTransition(r.Context(), playback, device)
		}
	} else {
		// Non-segmented playback: global = local
		globalPositionSec = localPositionSec
	}

	// Use stored total duration (always correct for global duration)
	durationSec := playback.DurationSec
	if durationSec == 0 {
		// Fallback to track duration if stored duration is 0
		durationSec = int(trackDuration.Seconds())
	}

	slog.Debug("status position check",
		"local_position", localPositionSec,
		"global_position", globalPositionSec,
		"current_segment", playback.CurrentSegment,
		"segment_duration", playback.SegmentDurationSec,
		"total_duration", durationSec,
	)

	// Update stored global position
	h.playbackStore.UpdatePosition(playback.ID, globalPositionSec)

	// Get volume level
	volume, _ := avt.GetVolume(r.Context())
	muted, _ := avt.GetMute(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":       true,
		"item_id":      playback.ItemID,
		"is_playing":   isPlaying,
		"position_sec": globalPositionSec,
		"duration_sec": durationSec,
		"position_str": formatDuration(time.Duration(globalPositionSec) * time.Second),
		"duration_str": formatDurationSec(durationSec),
		"volume":       volume,
		"muted":        muted,
	})
}

// handleSegmentTransition handles the transition to the next segment.
func (h *PlayerHandler) handleSegmentTransition(ctx context.Context, playback *store.PlaybackSession, device *store.SonosDevice) {
	// Get cache entry to check segment count
	cacheEntry, err := h.cacheIndex.GetEntry(playback.ItemID)
	if err != nil || cacheEntry == nil {
		slog.Warn("failed to get cache entry for segment transition", "item_id", playback.ItemID, "error", err)
		return
	}

	nextSegment := playback.CurrentSegment + 1
	if nextSegment >= cacheEntry.SegmentCount {
		slog.Debug("reached last segment, no transition needed",
			"current_segment", playback.CurrentSegment,
			"segment_count", cacheEntry.SegmentCount)
		return
	}

	slog.Info("transitioning to next segment",
		"item_id", playback.ItemID,
		"from_segment", playback.CurrentSegment,
		"to_segment", nextSegment,
		"segment_count", cacheEntry.SegmentCount)

	// Build URL for next segment
	ext := ".m4a"
	switch cacheEntry.CacheFormat {
	case "mp3":
		ext = ".mp3"
	case "flac":
		ext = ".flac"
	}
	nextStreamURL := fmt.Sprintf("%s/stream/%s/segment_%03d%s", h.publicURL, playback.StreamToken, nextSegment, ext)

	// Get item for metadata
	session := &store.Session{ID: playback.SessionID}
	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		slog.Warn("failed to get ABS client for segment transition", "error", err)
		// Continue without metadata
	}

	var metadata string
	if absClient != nil {
		item, err := absClient.GetItem(ctx, playback.ItemID)
		if err == nil && item != nil {
			mimeType := cache.GetContentType(cacheEntry.CacheFormat)
			metadata = buildDIDLMetadata(item, nextStreamURL, mimeType)
		}
	}

	// Set next segment on Sonos
	avt := sonos.NewAVTransport(device.IPAddress)
	if err := avt.SetAVTransportURI(ctx, nextStreamURL, metadata); err != nil {
		slog.Error("failed to set next segment URI", "error", err)
		return
	}

	// Start playback of next segment
	if err := avt.Play(ctx); err != nil {
		slog.Error("failed to start next segment", "error", err)
		return
	}

	// Update playback session with new segment
	h.playbackStore.UpdateCurrentSegment(playback.ID, nextSegment)

	slog.Info("segment transition complete",
		"item_id", playback.ItemID,
		"new_segment", nextSegment)
}

// HandlePlayer handles GET /player/{item_id} requests.
func (h *PlayerHandler) HandlePlayer(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Extract item ID from path (Go 1.22+ path parameter)
	itemID := r.PathValue("id")
	if itemID == "" {
		// Fallback to manual extraction for compatibility
		itemID = r.URL.Path[len("/player/"):]
	}
	if itemID == "" {
		http.Error(w, "item_id required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get authenticated ABS client for the session
	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		slog.Error("failed to get ABS client for session", "error", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	// Get item details
	item, err := absClient.GetItem(ctx, itemID)
	if err != nil {
		slog.Error("failed to get item", "item_id", itemID, "error", err)
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	// Get playback session
	playback, _ := h.playbackStore.GetBySessionID(session.ID)

	// Build template data with layout requirements
	data := map[string]interface{}{
		"Title":      item.Media.Metadata.Title,
		"ShowHeader": true,
		"Username":   session.ABSUsername,
		"Item":       item,
		"Playback":   playback,
		"LibraryID":  item.LibraryID,
	}

	h.renderPlayerPage(w, "player.html", data)
}

// renderPlayerPage renders a full page with layout
func (h *PlayerHandler) renderPlayerPage(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	funcMap := template.FuncMap{
		"formatDuration": func(seconds int) string {
			hours := seconds / 3600
			minutes := (seconds % 3600) / 60
			if hours > 0 {
				if minutes > 0 {
					return fmt.Sprintf("%d hr %d min", hours, minutes)
				}
				return fmt.Sprintf("%d hr", hours)
			}
			if minutes > 0 {
				return fmt.Sprintf("%d min", minutes)
			}
			return "< 1 min"
		},
		"mult": func(a, b float64) float64 { return a * b },
		"progressPercent": func(position, duration int) float64 {
			if duration == 0 {
				return 0
			}
			return float64(position) / float64(duration) * 100
		},
		"plus1": func(i int) int { return i + 1 },
		"minus": func(a, b int) int { return a - b },
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("[]")
			}
			return template.JS(b)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("web/templates/layout.html")
	if err != nil {
		slog.Error("template parse error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	tmpl, err = tmpl.ParseGlob("web/templates/partials/*.html")
	if err != nil {
		slog.Error("template parse partials error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	tmpl, err = tmpl.ParseFiles("web/templates/" + name)
	if err != nil {
		slog.Error("template parse page error", "file", name, "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template execute error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleCacheStatus handles GET /cache/status/{item_id} requests.
// Returns HTML for htmx swap into the cache-status div.
func (h *PlayerHandler) HandleCacheStatus(w http.ResponseWriter, r *http.Request) {
	// Extract item ID from path (Go 1.22+ path parameter)
	itemID := r.PathValue("id")
	if itemID == "" {
		// Fallback to manual extraction for compatibility
		itemID = r.URL.Path[len("/cache/status/"):]
	}
	if itemID == "" {
		http.Error(w, "item_id required", http.StatusBadRequest)
		return
	}

	status, err := h.cacheIndex.GetStatus(itemID)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	// Render HTML template for htmx
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Status": string(status),
	}
	if err := h.templates.ExecuteTemplate(w, "cache-status-badge", data); err != nil {
		slog.Error("template execute error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleStop handles POST /transport/stop requests.
func (h *PlayerHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form to get optional current_sonos_uuid (the device user currently has selected)
	r.ParseForm()
	currentSelectedUUID := r.FormValue("current_sonos_uuid")

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		http.Error(w, "no active playback", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	// Get Sonos device from playback session
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		slog.Warn("playback device not found", "uuid", playback.SonosUUID)
		// Continue anyway to clean up session
	}

	var avt *sonos.AVTransport
	if device != nil {
		avt = sonos.NewAVTransport(device.IPAddress)

		// Get current position before stopping
		posInfo, _ := avt.GetPositionInfo(ctx)
		if posInfo != nil {
			relTime := sonos.ParseDuration(posInfo.RelTime)
			playback.PositionSec = int(relTime.Seconds())
			h.playbackStore.UpdatePosition(playback.ID, playback.PositionSec)
		}

		// Stop playback on the playback session's device
		if err := avt.Stop(ctx); err != nil {
			// Error 701 = "Transition not available" - device may already be stopped
			if strings.Contains(err.Error(), "errorCode>701") {
				slog.Debug("stop not needed on primary device - already stopped", "device", device.Name)
			} else {
				slog.Warn("failed to stop on primary device", "device", device.Name, "error", err)
			}
		} else {
			slog.Debug("stopped playback on primary device", "device", device.Name)
		}
	}

	// Also try to stop on the currently selected device if different (handles failed player switches)
	if currentSelectedUUID != "" && currentSelectedUUID != playback.SonosUUID {
		selectedDevice, err := h.sonosStore.Get(currentSelectedUUID)
		if err == nil && selectedDevice != nil {
			selectedAVT := sonos.NewAVTransport(selectedDevice.IPAddress)
			if err := selectedAVT.Stop(ctx); err != nil {
				if strings.Contains(err.Error(), "errorCode>701") {
					slog.Debug("stop not needed on selected device - already stopped", "device", selectedDevice.Name)
				} else {
					slog.Debug("failed to stop on selected device", "device", selectedDevice.Name, "error", err)
				}
			} else {
				slog.Debug("stopped playback on selected device", "device", selectedDevice.Name)
			}
		}
	}

	// Save progress to ABS
	if playback.PositionSec > 0 {
		absClient, err := h.authHandler.GetABSClientForSession(session)
		if err == nil {
			progress := abs.ProgressUpdate{
				CurrentTime: float64(playback.PositionSec),
				Duration:    float64(playback.DurationSec),
				Progress:    float64(playback.PositionSec) / float64(playback.DurationSec),
			}
			if err := absClient.UpdateProgress(ctx, playback.ItemID, progress); err != nil {
				slog.Warn("failed to sync progress to ABS", "error", err)
			}
		}
	}

	// Delete playback session (user explicitly stopped)
	if err := h.playbackStore.Delete(playback.ID); err != nil {
		slog.Warn("failed to delete playback session", "error", err)
	}

	w.WriteHeader(http.StatusOK)
}

// buildDIDLMetadata creates DIDL-Lite XML for Sonos.
func buildDIDLMetadata(item *abs.LibraryItem, streamURL string, mimeType string) string {
	title := item.Media.Metadata.Title
	if title == "" {
		title = "Audiobook"
	}

	author := ""
	if len(item.Media.Metadata.Authors) > 0 {
		author = item.Media.Metadata.Authors[0].Name
	}

	// Minimal DIDL-Lite that Sonos accepts
	// protocolInfo format: <protocol>:<network>:<contentFormat>:<additionalInfo>
	return fmt.Sprintf(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">
<item id="1" parentID="0" restricted="1">
<dc:title>%s</dc:title>
<dc:creator>%s</dc:creator>
<upnp:class>object.item.audioItem.musicTrack</upnp:class>
<res protocolInfo="http-get:*:%s:*">%s</res>
</item>
</DIDL-Lite>`, escapeXML(title), escapeXML(author), mimeType, escapeXML(streamURL))
}

// escapeXML escapes special XML characters.
func escapeXML(s string) string {
	// Must escape & first, before other entities that contain &
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// formatDuration formats a duration as HH:MM:SS or MM:SS.
func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// formatDurationSec formats seconds as HH:MM:SS or MM:SS.
func formatDurationSec(totalSec int) string {
	hours := totalSec / 3600
	minutes := (totalSec % 3600) / 60
	seconds := totalSec % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// generateID generates a unique ID.
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// HandleSetVolume handles POST /transport/volume requests.
func (h *PlayerHandler) HandleSetVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	volumeStr := r.FormValue("volume")
	if volumeStr == "" {
		// No volume value - ignore silently
		w.WriteHeader(http.StatusOK)
		return
	}
	volume, err := strconv.Atoi(volumeStr)
	if err != nil {
		slog.Debug("invalid volume value", "value", volumeStr, "error", err)
		http.Error(w, "invalid volume", http.StatusBadRequest)
		return
	}

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		// No active playback - ignore volume change silently
		slog.Debug("volume change: no playback session found", "session_id", session.ID, "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Debug("volume change: found playback", "session_id", session.ID, "playback_id", playback.ID, "volume", volume)

	// Get Sonos device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	avt := sonos.NewAVTransport(device.IPAddress)

	if err := avt.SetVolume(r.Context(), volume); err != nil {
		slog.Error("failed to set volume", "error", err)
		http.Error(w, "failed to set volume", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleToggleMute handles POST /transport/mute requests.
func (h *PlayerHandler) HandleToggleMute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := GetSession(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get current playback session
	playback, err := h.playbackStore.GetBySessionID(session.ID)
	if err != nil || playback == nil {
		http.Error(w, "no active playback", http.StatusNotFound)
		return
	}

	// Get Sonos device
	device, err := h.sonosStore.Get(playback.SonosUUID)
	if err != nil || device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	avt := sonos.NewAVTransport(device.IPAddress)

	// Get current mute state and toggle
	currentMute, err := avt.GetMute(r.Context())
	if err != nil {
		slog.Error("failed to get mute state", "error", err)
		http.Error(w, "failed to get mute state", http.StatusInternalServerError)
		return
	}

	if err := avt.SetMute(r.Context(), !currentMute); err != nil {
		slog.Error("failed to toggle mute", "error", err)
		http.Error(w, "failed to toggle mute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"muted": !currentMute,
	})
}

// GetSession extracts the session from context.
func GetSession(ctx context.Context) *store.Session {
	session, ok := ctx.Value(sessionContextKey).(*store.Session)
	if !ok {
		return nil
	}
	return session
}
