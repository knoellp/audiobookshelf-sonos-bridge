package cache

// CompatibilityLevel indicates how compatible an audio format is with Sonos.
type CompatibilityLevel int

const (
	// Compatible means the file can be used directly without any conversion.
	Compatible CompatibilityLevel = iota
	// NeedsRemux means the audio codec is compatible but the container needs to be changed.
	NeedsRemux
	// NeedsTranscode means the audio codec is not compatible and needs to be re-encoded.
	NeedsTranscode
)

// String returns a human-readable representation of the compatibility level.
func (cl CompatibilityLevel) String() string {
	switch cl {
	case Compatible:
		return "compatible"
	case NeedsRemux:
		return "needs_remux"
	case NeedsTranscode:
		return "needs_transcode"
	default:
		return "unknown"
	}
}

// CompatibilityChecker determines if an audio format is Sonos-compatible.
type CompatibilityChecker struct{}

// NewCompatibilityChecker creates a new compatibility checker.
func NewCompatibilityChecker() *CompatibilityChecker {
	return &CompatibilityChecker{}
}

// Sonos-compatible codecs (per Sonos documentation)
// See: docs/references/sonos-integration.md#unterstützte-audioformate
var sonosCompatibleCodecs = map[string]bool{
	"mp3":    true, // MP3: 16-48 kHz
	"aac":    true, // AAC-LC, HE-AAC, HEv2-AAC: 8-48 kHz
	"flac":   true, // FLAC: 8-48 kHz
	"wmav2":  true, // WMA (NOT WMA Voice): 8-48 kHz
	"vorbis": true, // Ogg Vorbis: 8-48 kHz
	// NOTE: ALAC and PCM/WAV are NOT listed in official Sonos documentation
	// NOTE: Opus is NOT supported by Sonos
}

// Container formats that are directly compatible with Sonos for streaming
// Per Sonos docs: .m4a/.mp4/.aac, .flac, .mp3, .ogg, .asf/.wma
var sonosCompatibleContainers = map[string]map[string]bool{
	// MP3 container with mp3 codec
	"mp3": {
		"mp3": true,
	},
	// MP4/M4A container with AAC codec
	"mp4": {
		"aac": true,
	},
	"m4a": {
		"aac": true,
	},
	// FLAC container with flac codec
	"flac": {
		"flac": true,
	},
	// OGG container with vorbis codec
	"ogg": {
		"vorbis": true,
	},
	// WMA/ASF container with wmav2 codec
	"asf": {
		"wmav2": true,
	},
}

// Containers that need remuxing even if codec is compatible
var remuxContainers = map[string]string{
	"m4b":      "mp4",      // Audiobook → M4A
	"mov":      "mp4",      // QuickTime → M4A
	"3gp":      "mp4",      // 3GPP → M4A
	"matroska": "mp4",      // MKA → depends on codec
	"mka":      "mp4",      // Matroska Audio → depends on codec
	"webm":     "ogg",      // WebM → OGG
}

// Check determines the compatibility level of an audio format.
func (cc *CompatibilityChecker) Check(format *AudioFormat) CompatibilityLevel {
	// First check if the codec is supported at all
	if !sonosCompatibleCodecs[format.AudioCodec] {
		// Codec not supported, needs full transcoding
		return NeedsTranscode
	}

	// Check if the container + codec combination is directly compatible
	if compatibleCodecs, ok := sonosCompatibleContainers[format.Container]; ok {
		if compatibleCodecs[format.AudioCodec] {
			// Perfect match: container and codec are both compatible
			return Compatible
		}
	}

	// Check if this container is known to need remuxing
	if _, needsRemux := remuxContainers[format.Container]; needsRemux {
		// Container needs remuxing but codec is compatible
		return NeedsRemux
	}

	// Special case: MP4 container with AAC is compatible (even if not in exact list)
	if format.Container == "mp4" && format.AudioCodec == "aac" {
		return Compatible
	}

	// Special case: any container with mp3 codec can be remuxed to mp3
	if format.AudioCodec == "mp3" {
		if format.Container == "mp3" {
			return Compatible
		}
		return NeedsRemux
	}

	// Codec is supported but container is not ideal
	// We can remux to a better container
	return NeedsRemux
}

// GetTargetFormat returns the recommended output format for a given audio format.
// Returns the ffmpeg format name for the target container.
func (cc *CompatibilityChecker) GetTargetFormat(format *AudioFormat) string {
	switch format.AudioCodec {
	case "aac":
		return "mp4" // M4A files use mp4 container
	case "mp3":
		return "mp3"
	case "flac":
		return "flac"
	case "vorbis":
		return "ogg"
	case "wmav2":
		return "asf" // WMA files use ASF container
	default:
		// Default to MP3 for transcoding (non-compatible codecs)
		return "mp3"
	}
}

// GetTargetExtension returns the file extension for a given format.
func (cc *CompatibilityChecker) GetTargetExtension(ffmpegFormat string) string {
	switch ffmpegFormat {
	case "mp4":
		return ".m4a"
	case "mp3":
		return ".mp3"
	case "flac":
		return ".flac"
	case "ogg":
		return ".ogg"
	case "asf":
		return ".wma"
	default:
		return ".mp3"
	}
}
