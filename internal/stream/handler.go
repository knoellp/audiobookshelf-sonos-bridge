package stream

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"audiobookshelf-sonos-bridge/internal/cache"
)

// segmentPattern matches segment file names like "segment_000.m4a"
var segmentPattern = regexp.MustCompile(`^segment_(\d{3})\.(m4a|mp3|flac)$`)

// Handler handles streaming requests.
type Handler struct {
	tokenGen   *TokenGenerator
	cacheIndex *cache.Index
	publicURL  string
}

// NewHandler creates a new stream handler.
func NewHandler(tokenGen *TokenGenerator, cacheIndex *cache.Index, publicURL string) *Handler {
	return &Handler{
		tokenGen:   tokenGen,
		cacheIndex: cacheIndex,
		publicURL:  publicURL,
	}
}

// GetStreamURL returns the full URL for streaming an item.
// format should be "mp3", "mp4", "flac", "ogg", or "asf".
func (h *Handler) GetStreamURL(token string, format string) string {
	fileName := cache.GetCacheFileName(format)
	return fmt.Sprintf("%s/stream/%s/%s", h.publicURL, token, fileName)
}

// GetSegmentStreamURL returns the full URL for streaming a specific segment.
func (h *Handler) GetSegmentStreamURL(token string, segmentIndex int, format string) string {
	ext := ".m4a"
	switch format {
	case "mp3":
		ext = ".mp3"
	case "flac":
		ext = ".flac"
	}
	return fmt.Sprintf("%s/stream/%s/segment_%03d%s", h.publicURL, token, segmentIndex, ext)
}

// HandleStream handles GET /stream/{token}/audio.* or /stream/{token}/segment_*.* requests.
func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	// Extract token and filename from path
	// Path format: /stream/{token}/audio.* or /stream/{token}/segment_000.*
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	tokenStr := pathParts[1]
	fileName := pathParts[2]

	// Validate token
	payload, err := h.tokenGen.Validate(tokenStr)
	if err != nil {
		slog.Warn("invalid stream token", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Look up cache entry to get the correct format and path
	entry, err := h.cacheIndex.GetEntry(payload.ItemID)
	if err != nil {
		slog.Error("failed to get cache entry", "item_id", payload.ItemID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if entry == nil {
		slog.Warn("cache entry not found", "item_id", payload.ItemID)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Determine the cache file path
	var cachePath string

	// Check if this is a segment request
	if matches := segmentPattern.FindStringSubmatch(fileName); matches != nil {
		// Segment request: /stream/{token}/segment_000.m4a
		segmentIndex, _ := strconv.Atoi(matches[1])

		if !entry.IsSegmented() {
			slog.Warn("segment requested but cache is not segmented",
				"item_id", payload.ItemID,
				"requested_segment", segmentIndex)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if segmentIndex >= entry.SegmentCount {
			slog.Warn("segment index out of range",
				"item_id", payload.ItemID,
				"requested_segment", segmentIndex,
				"segment_count", entry.SegmentCount)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Get segment file path
		cacheDir := h.cacheIndex.GetCacheDir(entry.ItemID)
		segmentFile := entry.GetSegmentFileName(segmentIndex)
		cachePath = filepath.Join(cacheDir, segmentFile)

		slog.Debug("streaming segment",
			"item_id", payload.ItemID,
			"segment_index", segmentIndex,
			"segment_count", entry.SegmentCount,
			"path", cachePath)
	} else {
		// Standard single-file request: /stream/{token}/audio.m4a
		if entry.IsSegmented() {
			// For segmented entries, requesting audio.* is invalid
			// Client should request specific segments
			slog.Warn("single file requested but cache is segmented",
				"item_id", payload.ItemID,
				"segment_count", entry.SegmentCount)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		cachePath = h.cacheIndex.GetCachePathFromEntry(entry)
	}

	// Check if file exists
	fileInfo, err := os.Stat(cachePath)
	if os.IsNotExist(err) {
		slog.Warn("cache file not found", "item_id", payload.ItemID, "path", cachePath, "format", entry.CacheFormat)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("failed to stat cache file", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Open file
	file, err := os.Open(cachePath)
	if err != nil {
		slog.Error("failed to open cache file", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileSize := fileInfo.Size()

	// Get MIME type from cache entry format
	mimeType := cache.GetContentType(entry.CacheFormat)
	slog.Debug("streaming cached file", "item_id", payload.ItemID, "format", entry.CacheFormat, "mime_type", mimeType, "size", fileSize)

	// Handle Range requests (RFC 7233)
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		h.handleRangeRequest(w, r, file, fileSize, rangeHeader, mimeType)
		return
	}

	// Full file response
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")

	if r.Method == http.MethodHead {
		return
	}

	if _, err := io.Copy(w, file); err != nil {
		slog.Debug("stream copy error", "error", err)
	}

	slog.Debug("streamed full file", "item_id", payload.ItemID, "size", fileSize)
}

// handleRangeRequest handles HTTP Range requests for partial content.
func (h *Handler) handleRangeRequest(w http.ResponseWriter, r *http.Request, file *os.File, fileSize int64, rangeHeader string, mimeType string) {
	// Parse Range header: "bytes=start-end" or "bytes=start-"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		http.Error(w, "invalid range header", http.StatusBadRequest)
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		http.Error(w, "invalid range format", http.StatusBadRequest)
		return
	}

	var start, end int64
	var err error

	// Parse start
	if parts[0] == "" {
		// Suffix range: "-500" means last 500 bytes
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.Error(w, "invalid range", http.StatusBadRequest)
			return
		}
		start = fileSize - suffixLen
		if start < 0 {
			start = 0
		}
		end = fileSize - 1
	} else {
		// Normal range
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid range", http.StatusBadRequest)
			return
		}

		if parts[1] == "" {
			end = fileSize - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				http.Error(w, "invalid range", http.StatusBadRequest)
				return
			}
		}
	}

	// Validate range
	if start < 0 || start >= fileSize || end < start || end >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Seek to start position
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "seek error", http.StatusInternalServerError)
		return
	}

	// Calculate content length
	contentLength := end - start + 1

	// Set headers for partial content
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusPartialContent)

	if r.Method == http.MethodHead {
		return
	}

	// Copy only the requested range
	if _, err := io.CopyN(w, file, contentLength); err != nil {
		slog.Debug("range copy error", "error", err)
	}

	slog.Debug("streamed range", "start", start, "end", end, "length", contentLength)
}
