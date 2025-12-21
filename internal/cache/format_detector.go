package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// AudioFormat represents the detected format of an audio file.
type AudioFormat struct {
	Container  string // "mp4", "matroska", "mp3", etc.
	AudioCodec string // "aac", "mp3", "flac", etc.
	Bitrate    int    // in kbps
	SampleRate int    // in Hz
	Channels   int
	Duration   int // in seconds
}

// FormatDetector detects audio format using ffprobe.
type FormatDetector struct{}

// NewFormatDetector creates a new format detector.
func NewFormatDetector() *FormatDetector {
	return &FormatDetector{}
}

// ffprobeOutput represents the JSON output structure from ffprobe.
type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
		BitRate    string `json:"bit_rate"`
	} `json:"streams"`
}

// Detect probes an audio file and returns its format information.
func (fd *FormatDetector) Detect(ctx context.Context, path string) (*AudioFormat, error) {
	// Check if ffprobe is available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return nil, fmt.Errorf("ffprobe not found: %w", err)
	}

	// Build ffprobe command
	args := []string{
		"-v", "error",
		"-show_entries", "format=format_name,bit_rate,duration",
		"-show_entries", "stream=codec_name,codec_type,sample_rate,channels,bit_rate",
		"-of", "json",
		path,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Extract format information
	format := &AudioFormat{}

	// Parse container format (use first format name from comma-separated list)
	formatNames := strings.Split(probe.Format.FormatName, ",")
	if len(formatNames) > 0 {
		format.Container = formatNames[0]
	}

	// Parse duration
	if probe.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			format.Duration = int(duration)
		}
	}

	// Parse format-level bitrate
	formatBitrate := 0
	if probe.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(probe.Format.BitRate); err == nil {
			formatBitrate = bitrate / 1000 // Convert to kbps
		}
	}

	// Find the first audio stream and extract codec information
	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			format.AudioCodec = stream.CodecName

			// Parse sample rate
			if stream.SampleRate != "" {
				if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
					format.SampleRate = sampleRate
				}
			}

			// Parse channels
			format.Channels = stream.Channels

			// Parse stream-level bitrate
			if stream.BitRate != "" {
				if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
					format.Bitrate = bitrate / 1000 // Convert to kbps
				}
			}

			// If stream bitrate is not available, use format bitrate
			if format.Bitrate == 0 && formatBitrate > 0 {
				format.Bitrate = formatBitrate
			}

			break // Use first audio stream
		}
	}

	// Validate that we found an audio stream
	if format.AudioCodec == "" {
		return nil, fmt.Errorf("no audio stream found in file")
	}

	return format, nil
}

// String returns a human-readable representation of the audio format.
func (af *AudioFormat) String() string {
	return fmt.Sprintf("container=%s codec=%s bitrate=%dkbps samplerate=%dHz channels=%d duration=%ds",
		af.Container, af.AudioCodec, af.Bitrate, af.SampleRate, af.Channels, af.Duration)
}
