package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/cache"
	"audiobookshelf-sonos-bridge/internal/store"
)

func TestHandlePlay_MissingParams(t *testing.T) {
	// Create a request without required params
	form := url.Values{}
	req := httptest.NewRequest("POST", "/play", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create handler without dependencies (will fail at auth check)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePlay(w, req)

	// Should fail with unauthorized since no session
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandlePlay_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/play", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePlay(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandlePause_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/transport/pause", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePause(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleResume_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/transport/resume", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleResume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleSeek_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/transport/seek", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleSeek(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleStop_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/transport/stop", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleStop(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandlePause_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/transport/pause", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePause(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleResume_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/transport/resume", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleResume(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleSeek_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/transport/seek", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleSeek(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleStop_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/transport/stop", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleStop(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleStatus_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("GET", "/status", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleStatus(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleCacheStatus_MissingItemID(t *testing.T) {
	req := httptest.NewRequest("GET", "/cache/status/", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandleCacheStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "0:00"},
		{30, "0:30"},
		{60, "1:00"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{7325, "2:02:05"},
	}

	for _, tc := range tests {
		result := formatSeconds(tc.seconds)
		if result != tc.expected {
			t.Errorf("formatSeconds(%d) = %q, want %q", tc.seconds, result, tc.expected)
		}
	}
}

// Helper function matching the template function
func formatSeconds(sec int) string {
	hours := sec / 3600
	minutes := (sec % 3600) / 60
	seconds := sec % 60

	if hours > 0 {
		return sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return sprintf("%d:%02d", minutes, seconds)
}

func sprintf(format string, args ...interface{}) string {
	switch len(args) {
	case 2:
		return strings.Replace(strings.Replace(format, "%d", itoa(args[0].(int)), 1), "%02d", pad2(args[1].(int)), 1)
	case 3:
		s := strings.Replace(format, "%d", itoa(args[0].(int)), 1)
		s = strings.Replace(s, "%02d", pad2(args[1].(int)), 1)
		s = strings.Replace(s, "%02d", pad2(args[2].(int)), 1)
		return s
	}
	return format
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"<tag>", "&lt;tag&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{"<a href=\"url\">link</a>", "&lt;a href=&quot;url&quot;&gt;link&lt;/a&gt;"},
	}

	for _, tc := range tests {
		result := escapeXML(tc.input)
		if result != tc.expected {
			t.Errorf("escapeXML(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestBuildDIDLMetadata(t *testing.T) {
	// This is a basic test to ensure the function produces valid-looking XML
	// A full test would validate against the DIDL-Lite schema

	item := &abs.LibraryItem{
		Media: abs.BookMedia{
			Metadata: abs.BookMetadata{
				Title: "Test Book",
				Authors: []abs.Author{
					{Name: "Test Author"},
				},
			},
		},
	}

	metadata := buildDIDLMetadata(item, "http://example.com/stream", "audio/mp4")

	// Should contain DIDL-Lite namespace
	if !strings.Contains(metadata, "DIDL-Lite") {
		t.Error("expected metadata to contain DIDL-Lite")
	}

	// Should contain the stream URL
	if !strings.Contains(metadata, "http://example.com/stream") {
		t.Error("expected metadata to contain stream URL")
	}

	// Should have audio class
	if !strings.Contains(metadata, "object.item.audioItem") {
		t.Error("expected metadata to contain audioItem class")
	}

	// Should contain title
	if !strings.Contains(metadata, "Test Book") {
		t.Error("expected metadata to contain title")
	}

	// Should contain author
	if !strings.Contains(metadata, "Test Author") {
		t.Error("expected metadata to contain author")
	}
}

func TestHandlePrepare_MissingItemID(t *testing.T) {
	req := httptest.NewRequest("POST", "/cache/prepare/", nil)
	req = req.WithContext(context.WithValue(req.Context(), sessionContextKey, &store.Session{ID: "s", UserID: "u"}))
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePrepare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePrepare_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/cache/prepare/x", nil)
	h := &PlayerHandler{}

	w := httptest.NewRecorder()
	h.HandlePrepare(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleCacheStatus_JSON(t *testing.T) {
	db, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	idx := cache.NewIndex(store.NewCacheStore(db), t.TempDir())
	worker := cache.NewWorker(idx, cache.NewTranscoder(), 1)
	if err := idx.CreateEntry("item-1", "/media/a.m4a", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := idx.MarkFailed("item-1", "boom"); err != nil {
		t.Fatal(err)
	}

	h := &PlayerHandler{cacheIndex: idx, cacheWorker: worker}

	req := httptest.NewRequest("GET", "/cache/status/item-1?format=json", nil)
	req.SetPathValue("id", "item-1")
	w := httptest.NewRecorder()
	h.HandleCacheStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "failed" || resp["error"] != "boom" {
		t.Errorf("unexpected response: %v", resp)
	}
	for _, k := range []string{"percent", "phase", "elapsed_sec"} {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}
}

func cacheStatusJSON(t *testing.T, h *PlayerHandler, itemID string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/cache/status/"+itemID+"?format=json", nil)
	req.SetPathValue("id", itemID)
	w := httptest.NewRecorder()
	h.HandleCacheStatus(w, req)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return resp
}

func TestHandleCacheStatus_EffectiveStatus(t *testing.T) {
	db, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	idx := cache.NewIndex(store.NewCacheStore(db), t.TempDir())
	worker := cache.NewWorker(idx, cache.NewTranscoder(), 1)
	h := &PlayerHandler{cacheIndex: idx, cacheWorker: worker}

	// "ready" in the DB but no files on disk: reported as pending.
	if err := idx.CreateEntry("item-1", "/media/a.m4a", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := idx.MarkReady("item-1", 60); err != nil {
		t.Fatal(err)
	}
	resp := cacheStatusJSON(t, h, "item-1")
	if resp["status"] != "pending" || resp["percent"] != float64(0) {
		t.Errorf("ready without files: %v", resp)
	}

	// Same entry with an active (queued) job: in_progress.
	worker.Enqueue(cache.Job{ItemID: "item-1"})
	resp = cacheStatusJSON(t, h, "item-1")
	if resp["status"] != "in_progress" {
		t.Errorf("ready without files but queued: %v", resp)
	}

	// Pending entry with a queued job: in_progress; HTML badge agrees.
	if err := idx.CreateEntry("item-2", "/media/b.m4a", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	worker.Enqueue(cache.Job{ItemID: "item-2"})
	resp = cacheStatusJSON(t, h, "item-2")
	if resp["status"] != "in_progress" {
		t.Errorf("queued pending: %v", resp)
	}
}

func TestEffectiveCacheStatus(t *testing.T) {
	cases := []struct {
		status         store.CacheStatus
		cached, active bool
		want           store.CacheStatus
	}{
		{store.CacheStatusPending, true, false, store.CacheStatusPending},
		{store.CacheStatusPending, true, true, store.CacheStatusInProgress},
		{store.CacheStatusReady, true, false, store.CacheStatusReady},
		{store.CacheStatusReady, false, false, store.CacheStatusPending},
		{store.CacheStatusReady, false, true, store.CacheStatusInProgress},
		{store.CacheStatusFailed, true, false, store.CacheStatusFailed},
	}
	for _, c := range cases {
		if got := effectiveCacheStatus(c.status, c.cached, c.active); got != c.want {
			t.Errorf("effectiveCacheStatus(%s,%v,%v) = %s, want %s", c.status, c.cached, c.active, got, c.want)
		}
	}
}

func TestIPInDevices(t *testing.T) {
	devs := []*store.SonosDevice{{IPAddress: "192.168.1.10"}, {IPAddress: "192.168.1.11"}}
	if !ipInDevices("192.168.1.11", devs) {
		t.Error("known IP not found")
	}
	if ipInDevices("169.254.169.254", devs) || ipInDevices("", devs) {
		t.Error("unknown IP accepted")
	}
}

func TestPreferABSPosition(t *testing.T) {
	now := time.Now()
	playing := &store.PlaybackSession{IsPlaying: true, LastPositionUpdate: now.Add(-time.Minute)}
	if preferABSPosition(playing, now.UnixMilli()) {
		t.Error("ABS must not win while playing")
	}
	paused := &store.PlaybackSession{LastPositionUpdate: now.Add(-time.Minute)}
	if !preferABSPosition(paused, now.UnixMilli()) {
		t.Error("newer ABS progress should win when paused")
	}
	if preferABSPosition(paused, now.Add(-time.Hour).UnixMilli()) {
		t.Error("older ABS progress must not win")
	}
}
