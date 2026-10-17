package web

import (
	"strings"

	"audiobookshelf-sonos-bridge/internal/store"
)

// shouldStorePosition decides whether a position reported by Sonos should
// overwrite the stored position. A stopped transport or a sudden reset to zero
// (with a meaningful stored position) must not clobber the saved progress.
func shouldStorePosition(newGlobalSec, storedSec int, transportState string) bool {
	if transportState == "STOPPED" {
		return false
	}
	if newGlobalSec == 0 && storedSec > 10 {
		return false
	}
	return true
}

// globalPosition converts a position reported by Sonos for the current
// stream (local to the current segment) into a position within the whole
// item. For non-segmented playback the local position is returned unchanged.
func globalPosition(p *store.PlaybackSession, localSec int) int {
	if p != nil && p.SegmentDurationSec > 0 {
		return store.SegmentToGlobal(p.CurrentSegment, localSec, p.SegmentDurationSec)
	}
	return localSec
}

// isBridgeStream reports whether a Sonos track URI points at one of this
// bridge's stream endpoints. Anything else (radio, Spotify, another app)
// must never be interpreted as an audiobook position.
func isBridgeStream(trackURI, publicURL string) bool {
	return strings.HasPrefix(trackURI, strings.TrimRight(publicURL, "/")+"/stream/")
}
