package web

import (
	"testing"

	"audiobookshelf-sonos-bridge/internal/store"
)

func TestShouldStorePosition(t *testing.T) {
	tests := []struct {
		name   string
		newSec int
		stored int
		state  string
		want   bool
	}{
		{"normal advance", 120, 100, "PLAYING", true},
		{"zero with stored position", 0, 500, "PLAYING", false},
		{"stopped", 300, 290, "STOPPED", false},
		{"zero at genuine start", 0, 0, "PLAYING", true},
		{"zero with small stored", 0, 10, "PLAYING", true},
		{"unknown state advance", 50, 40, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStorePosition(tt.newSec, tt.stored, tt.state); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobalPosition(t *testing.T) {
	seg := &store.PlaybackSession{CurrentSegment: 2, SegmentDurationSec: 7200}
	if got := globalPosition(seg, 100); got != 14500 {
		t.Errorf("segmented: got %d, want 14500", got)
	}
	plain := &store.PlaybackSession{CurrentSegment: 0}
	if got := globalPosition(plain, 100); got != 100 {
		t.Errorf("plain: got %d, want 100", got)
	}
	if got := globalPosition(nil, 7); got != 7 {
		t.Errorf("nil: got %d, want 7", got)
	}
}

func TestIsBridgeStream(t *testing.T) {
	pub := "http://bridge.local:8080"
	tests := []struct {
		uri  string
		want bool
	}{
		{pub + "/stream/tok/audio.m4a", true},
		{pub + "/stream/tok/segment_001.m4a", true},
		{"x-sonosapi-stream:s12345?sid=254", false},
		{"http://other:8080/stream/tok/audio.m4a", false},
		{pub + "/cover/abc", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isBridgeStream(tt.uri, pub); got != tt.want {
			t.Errorf("%q: got %v, want %v", tt.uri, got, tt.want)
		}
	}
	if !isBridgeStream(pub+"/stream/t/a.mp3", pub+"/") {
		t.Error("trailing slash in publicURL should be tolerated")
	}
}
