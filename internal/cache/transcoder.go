package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// TranscodeProfile defines the output format settings.
type TranscodeProfile struct {
	AudioCodec   string
	Bitrate      string
	SampleRate   int
	Channels     int
	OutputFormat string
}

// DefaultProfile is the Sonos-compatible MP3 profile.
var DefaultProfile = TranscodeProfile{
	AudioCodec:   "libmp3lame",
	Bitrate:      "128k",
	SampleRate:   44100,
	Channels:     2,
	OutputFormat: "mp3",
}

// Common errors
var (
	ErrInsufficientDiskSpace = errors.New("insufficient disk space")
	ErrFFmpegNotFound        = errors.New("ffmpeg not found")
	ErrFFprobeNotFound       = errors.New("ffprobe not found")
	ErrInputFileNotFound     = errors.New("input file not found")
	ErrInvalidInputFile      = errors.New("invalid input file")
)

// TranscodeError provides detailed error information for transcoding failures.
type TranscodeError struct {
	ExitCode int
	Output   string
	Err      error
}

func (e *TranscodeError) Error() string {
	if e.ExitCode != 0 {
		return fmt.Sprintf("ffmpeg exited with code %d: %s", e.ExitCode, e.Output)
	}
	return e.Err.Error()
}

func (e *TranscodeError) Unwrap() error {
	return e.Err
}

// ParseFFmpegExitCode returns a human-readable error for common ffmpeg exit codes.
func ParseFFmpegExitCode(exitCode int, output string) error {
	switch exitCode {
	case 1:
		// Check output for specific errors
		outputLower := strings.ToLower(output)
		if strings.Contains(outputLower, "no such file") {
			return ErrInputFileNotFound
		}
		if strings.Contains(outputLower, "invalid data") || strings.Contains(outputLower, "invalid argument") {
			return ErrInvalidInputFile
		}
		return fmt.Errorf("ffmpeg general error")
	case 2:
		return fmt.Errorf("ffmpeg output error")
	case 69:
		return fmt.Errorf("ffmpeg resource unavailable")
	case 137:
		return fmt.Errorf("ffmpeg killed (possibly OOM)")
	default:
		return fmt.Errorf("ffmpeg unknown error (code %d)", exitCode)
	}
}

// Transcoder handles audio transcoding using ffmpeg.
type Transcoder struct {
	profile TranscodeProfile
}

// NewTranscoder creates a new transcoder with the default profile.
func NewTranscoder() *Transcoder {
	return &Transcoder{
		profile: DefaultProfile,
	}
}

// TranscodeResult contains the result of a transcoding operation.
type TranscodeResult struct {
	DurationSec int
	OutputSize  int64
}

// TranscodeMultiple concatenates multiple audio files and transcodes to the target format.
func (t *Transcoder) TranscodeMultiple(ctx context.Context, inputPaths []string, outputPath string) (*TranscodeResult, error) {
	if len(inputPaths) == 0 {
		return nil, ErrInputFileNotFound
	}

	// If only one file, use regular transcode
	if len(inputPaths) == 1 {
		return t.Transcode(ctx, inputPaths[0], outputPath)
	}

	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify all input files exist and calculate total size
	var totalSize int64
	for _, path := range inputPaths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%w: %s", ErrInputFileNotFound, path)
			}
			return nil, fmt.Errorf("failed to stat input file %s: %w", path, err)
		}
		totalSize += info.Size()
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space
	if err := t.CheckDiskSpace(outputPath, totalSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Create concat list file for ffmpeg
	concatListPath := outputPath + ".concat.txt"
	concatFile, err := os.Create(concatListPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat list: %w", err)
	}

	for _, path := range inputPaths {
		// Escape single quotes in path for ffmpeg concat format
		escapedPath := strings.ReplaceAll(path, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()
	defer os.Remove(concatListPath)

	// Create temp file
	tempPath := outputPath + ".tmp"

	// Build ffmpeg command using concat demuxer
	// -map 0:a selects only audio streams, excluding data streams (like chapter markers)
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-map", "0:a",          // Select only audio streams (excludes data/subtitle streams)
		"-map_chapters", "-1",  // Remove chapter metadata (prevents bin_data stream that Sonos can't handle)
		"-vn",                  // No video
		"-ar", strconv.Itoa(t.profile.SampleRate),
		"-ac", strconv.Itoa(t.profile.Channels),
		"-b:a", t.profile.Bitrate,
		"-f", t.profile.OutputFormat,
		"-y", // Overwrite output
		tempPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Capture stderr for error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tempPath)

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			outputStr := string(output)
			parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

			return nil, &TranscodeError{
				ExitCode: exitCode,
				Output:   truncateOutput(outputStr, 500),
				Err:      parsedErr,
			}
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, &TranscodeError{
			Err: fmt.Errorf("ffmpeg failed: %w", err),
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	// Get duration
	duration, err := t.GetDuration(ctx, outputPath)
	if err != nil {
		duration = ExtractDurationFromFFmpegOutput(string(output))
	}

	return &TranscodeResult{
		DurationSec: duration,
		OutputSize:  info.Size(),
	}, nil
}

// Transcode converts an audio file to the target format.
func (t *Transcoder) Transcode(ctx context.Context, inputPath, outputPath string) (*TranscodeResult, error) {
	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify input file exists
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInputFileNotFound
		}
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space (estimate: input size is a reasonable upper bound for compressed output)
	estimatedSize := inputInfo.Size()
	if err := t.CheckDiskSpace(outputPath, estimatedSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Create temp file
	tempPath := outputPath + ".tmp"

	// Build ffmpeg command
	// -map 0:a selects only audio streams, excluding data streams (like chapter markers)
	args := []string{
		"-i", inputPath,
		"-map", "0:a",          // Select only audio streams (excludes data/subtitle streams)
		"-map_chapters", "-1",  // Remove chapter metadata (prevents bin_data stream that Sonos can't handle)
		"-vn",                  // No video
		"-ar", strconv.Itoa(t.profile.SampleRate),
		"-ac", strconv.Itoa(t.profile.Channels),
		"-b:a", t.profile.Bitrate,
		"-f", t.profile.OutputFormat,
		"-y", // Overwrite output
		tempPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Capture stderr for error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			outputStr := string(output)

			// Parse specific error
			parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

			return nil, &TranscodeError{
				ExitCode: exitCode,
				Output:   truncateOutput(outputStr, 500),
				Err:      parsedErr,
			}
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, &TranscodeError{
			Err: fmt.Errorf("ffmpeg failed: %w", err),
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	// Get duration
	duration, err := t.GetDuration(ctx, outputPath)
	if err != nil {
		// Try to extract from ffmpeg output
		duration = ExtractDurationFromFFmpegOutput(string(output))
	}

	return &TranscodeResult{
		DurationSec: duration,
		OutputSize:  info.Size(),
	}, nil
}

// truncateOutput truncates a string to maxLen characters.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// GetDuration returns the duration of an audio file in seconds.
func (t *Transcoder) GetDuration(ctx context.Context, path string) (int, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return int(duration), nil
}

// CheckDiskSpace checks if there's enough disk space for transcoding.
func (t *Transcoder) CheckDiskSpace(path string, requiredBytes int64) error {
	var stat syscall.Statfs_t

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := syscall.Statfs(dir, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	available := stat.Bavail * uint64(stat.Bsize)

	// Require at least the estimated size plus 10% buffer
	required := uint64(float64(requiredBytes) * 1.1)

	if available < required {
		return fmt.Errorf("insufficient disk space: %d bytes available, %d required", available, required)
	}

	return nil
}

// EstimateOutputSize estimates the output file size based on duration.
func (t *Transcoder) EstimateOutputSize(durationSec int) int64 {
	// MP3 at 128kbps = 16KB per second
	bytesPerSecond := int64(16 * 1024)
	return int64(durationSec) * bytesPerSecond
}

// ExtractDurationFromFFmpegOutput extracts duration from ffmpeg stderr.
func ExtractDurationFromFFmpegOutput(output string) int {
	// Look for "Duration: HH:MM:SS.ms"
	re := regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 4 {
		return 0
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])

	return hours*3600 + minutes*60 + seconds
}

// Remux changes the container format without re-encoding the audio.
// This is much faster than transcoding as it only copies the audio stream.
func (t *Transcoder) Remux(ctx context.Context, inputPath, outputPath string, outputFormat string) (*TranscodeResult, error) {
	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify input file exists
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInputFileNotFound
		}
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space (remux output is similar size to input)
	estimatedSize := inputInfo.Size()
	if err := t.CheckDiskSpace(outputPath, estimatedSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Create temp file
	tempPath := outputPath + ".tmp"

	// Build ffmpeg command for remuxing
	// -c:a copy means copy audio codec without re-encoding
	// -map 0:a selects only audio streams, excluding data streams (like chapter markers)
	// See: docs/references/ffmpeg.md#stream-copy-no-re-encoding
	args := []string{
		"-i", inputPath,
		"-map", "0:a",          // Select only audio streams (excludes data/subtitle streams)
		"-map_chapters", "-1",  // Remove chapter metadata (prevents bin_data stream that Sonos can't handle)
		"-c:a", "copy",         // Copy audio stream without re-encoding
		"-vn",                  // No video (removes cover art etc.)
	}

	// Add movflags for MP4/M4A files to enable HTTP streaming
	// This moves the moov atom to the beginning of the file
	// Use 'ipod' muxer with M4A brand for better compatibility with older Sonos devices (e.g., ZP90/Connect)
	// The default 'mp4' muxer uses 'isom' brand which is not recognized by some older Sonos firmware
	// The 'ipod' muxer respects the -brand flag and produces proper M4A files
	if outputFormat == "mp4" {
		args = append(args, "-movflags", "+faststart", "-brand", "M4A")
		args = append(args, "-f", "ipod") // Use ipod muxer for proper M4A brand
	} else {
		args = append(args, "-f", outputFormat)
	}

	args = append(args,
		"-y", // Overwrite output
		tempPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Capture stderr for error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			outputStr := string(output)

			// Parse specific error
			parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

			return nil, &TranscodeError{
				ExitCode: exitCode,
				Output:   truncateOutput(outputStr, 500),
				Err:      parsedErr,
			}
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, &TranscodeError{
			Err: fmt.Errorf("ffmpeg remux failed: %w", err),
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	// Get duration
	duration, err := t.GetDuration(ctx, outputPath)
	if err != nil {
		// Try to extract from ffmpeg output
		duration = ExtractDurationFromFFmpegOutput(string(output))
	}

	return &TranscodeResult{
		DurationSec: duration,
		OutputSize:  info.Size(),
	}, nil
}

// RemuxMultiple concatenates multiple audio files using remuxing (no re-encoding).
// All input files must have the same audio codec.
func (t *Transcoder) RemuxMultiple(ctx context.Context, inputPaths []string, outputPath string, outputFormat string) (*TranscodeResult, error) {
	if len(inputPaths) == 0 {
		return nil, ErrInputFileNotFound
	}

	// If only one file, use regular remux
	if len(inputPaths) == 1 {
		return t.Remux(ctx, inputPaths[0], outputPath, outputFormat)
	}

	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify all input files exist and calculate total size
	var totalSize int64
	for _, path := range inputPaths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%w: %s", ErrInputFileNotFound, path)
			}
			return nil, fmt.Errorf("failed to stat input file %s: %w", path, err)
		}
		totalSize += info.Size()
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space
	if err := t.CheckDiskSpace(outputPath, totalSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Create concat list file for ffmpeg
	concatListPath := outputPath + ".concat.txt"
	concatFile, err := os.Create(concatListPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat list: %w", err)
	}

	for _, path := range inputPaths {
		// Escape single quotes in path for ffmpeg concat format
		escapedPath := strings.ReplaceAll(path, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()
	defer os.Remove(concatListPath)

	// Create temp file
	tempPath := outputPath + ".tmp"

	// Build ffmpeg command using concat demuxer with stream copy
	// -map 0:a selects only audio streams, excluding data streams (like chapter markers)
	// See: docs/references/ffmpeg.md#concatenation
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-map", "0:a",          // Select only audio streams (excludes data/subtitle streams)
		"-map_chapters", "-1",  // Remove chapter metadata (prevents bin_data stream that Sonos can't handle)
		"-c:a", "copy",         // Copy audio stream (no re-encoding)
		"-vn",                  // No video
	}

	// Add movflags for MP4/M4A files to enable HTTP streaming
	// Use 'ipod' muxer with M4A brand for better compatibility with older Sonos devices (e.g., ZP90/Connect)
	// The default 'mp4' muxer uses 'isom' brand which is not recognized by some older Sonos firmware
	if outputFormat == "mp4" {
		args = append(args, "-movflags", "+faststart", "-brand", "M4A")
		args = append(args, "-f", "ipod") // Use ipod muxer for proper M4A brand
	} else {
		args = append(args, "-f", outputFormat)
	}

	args = append(args,
		"-y", // Overwrite output
		tempPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Capture stderr for error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tempPath)

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			outputStr := string(output)
			parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

			return nil, &TranscodeError{
				ExitCode: exitCode,
				Output:   truncateOutput(outputStr, 500),
				Err:      parsedErr,
			}
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, &TranscodeError{
			Err: fmt.Errorf("ffmpeg remux concat failed: %w", err),
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	// Get duration
	duration, err := t.GetDuration(ctx, outputPath)
	if err != nil {
		duration = ExtractDurationFromFFmpegOutput(string(output))
	}

	return &TranscodeResult{
		DurationSec: duration,
		OutputSize:  info.Size(),
	}, nil
}

// SmartTranscode intelligently chooses between copy, remux, or transcode based on format.
func (t *Transcoder) SmartTranscode(ctx context.Context, inputPath, outputPath string) (*TranscodeResult, error) {
	// Detect input format
	detector := NewFormatDetector()
	format, err := detector.Detect(ctx, inputPath)
	if err != nil {
		// If format detection fails, fall back to regular transcode
		slog.Debug("format detection failed, using transcode fallback", "path", inputPath, "error", err)
		return t.Transcode(ctx, inputPath, outputPath)
	}

	slog.Debug("audio format detected", "format", format.String())

	// Check compatibility
	checker := NewCompatibilityChecker()
	compatibility := checker.Check(format)

	switch compatibility {
	case Compatible:
		// File is already compatible, just copy it
		// Note: We still need to ensure it's in the right location
		// For now, we'll use remux with the same format to ensure consistency
		slog.Debug("using fast-path: remux (already compatible)",
			"container", format.Container,
			"codec", format.AudioCodec)
		targetFormat := checker.GetTargetFormat(format)
		return t.Remux(ctx, inputPath, outputPath, targetFormat)

	case NeedsRemux:
		// Codec is compatible but container needs changing
		slog.Debug("using fast-path: remux (container change)",
			"container", format.Container,
			"codec", format.AudioCodec,
			"target_format", checker.GetTargetFormat(format))
		targetFormat := checker.GetTargetFormat(format)
		return t.Remux(ctx, inputPath, outputPath, targetFormat)

	case NeedsTranscode:
		// Full transcoding required
		slog.Debug("using slow-path: transcode",
			"container", format.Container,
			"codec", format.AudioCodec,
			"reason", "codec not compatible")
		return t.Transcode(ctx, inputPath, outputPath)

	default:
		// Unknown compatibility, fall back to transcode
		slog.Debug("using slow-path: transcode (unknown compatibility)",
			"container", format.Container,
			"codec", format.AudioCodec)
		return t.Transcode(ctx, inputPath, outputPath)
	}
}

// SegmentDuration is the default segment duration for ZP90-compatible streaming.
// 2 hours = 7200 seconds, producing ~55MB segments at 63kbps AAC.
const SegmentDuration = 7200

// SegmentedResult contains the result of a segmented transcoding operation.
type SegmentedResult struct {
	DurationSec        int   // Total duration of the audio
	SegmentCount       int   // Number of segments created
	SegmentDurationSec int   // Duration of each segment (last may be shorter)
	TotalSize          int64 // Total size of all segments
}

// RemuxSegmented splits a single input file into 2-hour segments without re-encoding.
// This is required for ZP90/Sonos Connect devices which have a ~128MB RAM limit.
// Each segment is named segment_000.ext, segment_001.ext, etc.
func (t *Transcoder) RemuxSegmented(ctx context.Context, inputPath, outputDir string, outputFormat string) (*SegmentedResult, error) {
	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify input file exists
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInputFileNotFound
		}
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space (segments should be similar total size to input)
	estimatedSize := inputInfo.Size()
	if err := t.CheckDiskSpace(outputDir, estimatedSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Get total duration
	totalDuration, err := t.GetDuration(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get input duration: %w", err)
	}

	// Calculate number of segments
	segmentCount := (totalDuration + SegmentDuration - 1) / SegmentDuration // ceiling division
	if segmentCount < 1 {
		segmentCount = 1
	}

	slog.Debug("creating segmented remux",
		"input_path", inputPath,
		"total_duration", totalDuration,
		"segment_count", segmentCount,
		"segment_duration", SegmentDuration)

	// Determine file extension
	ext := ".m4a"
	switch outputFormat {
	case "mp3":
		ext = ".mp3"
	case "flac":
		ext = ".flac"
	}

	var totalSize int64

	// Create each segment
	for i := 0; i < segmentCount; i++ {
		startTime := i * SegmentDuration
		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%03d%s", i, ext))
		tempPath := segmentPath + ".tmp"

		// Calculate actual segment duration (last segment may be shorter)
		segmentDuration := SegmentDuration
		if startTime+segmentDuration > totalDuration {
			segmentDuration = totalDuration - startTime
		}

		slog.Debug("creating segment",
			"segment_index", i,
			"start_time", startTime,
			"segment_duration", segmentDuration,
			"output_path", segmentPath)

		// Build ffmpeg command for this segment
		// -ss before -i for fast seeking, -t for duration
		args := []string{
			"-ss", strconv.Itoa(startTime),
			"-i", inputPath,
			"-t", strconv.Itoa(segmentDuration),
			"-map", "0:a",         // Select only audio streams
			"-map_chapters", "-1", // Remove chapter metadata
			"-c:a", "copy",        // Copy audio stream without re-encoding
			"-vn",                 // No video
		}

		// Add format-specific options
		if outputFormat == "mp4" {
			args = append(args, "-movflags", "+faststart", "-brand", "M4A")
			args = append(args, "-f", "ipod")
		} else {
			args = append(args, "-f", outputFormat)
		}

		args = append(args, "-y", tempPath)

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			os.Remove(tempPath)

			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode := exitErr.ExitCode()
				outputStr := string(output)
				parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

				return nil, &TranscodeError{
					ExitCode: exitCode,
					Output:   truncateOutput(outputStr, 500),
					Err:      fmt.Errorf("segment %d: %w", i, parsedErr),
				}
			}

			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			return nil, &TranscodeError{
				Err: fmt.Errorf("ffmpeg segment %d failed: %w", i, err),
			}
		}

		// Atomic rename
		if err := os.Rename(tempPath, segmentPath); err != nil {
			os.Remove(tempPath)
			return nil, fmt.Errorf("failed to rename segment %d: %w", i, err)
		}

		// Get segment size
		info, err := os.Stat(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat segment %d: %w", i, err)
		}
		totalSize += info.Size()

		slog.Debug("segment created",
			"segment_index", i,
			"size_bytes", info.Size())
	}

	slog.Info("segmented remux complete",
		"input_path", inputPath,
		"segment_count", segmentCount,
		"total_size", totalSize)

	return &SegmentedResult{
		DurationSec:        totalDuration,
		SegmentCount:       segmentCount,
		SegmentDurationSec: SegmentDuration,
		TotalSize:          totalSize,
	}, nil
}

// TranscodeSegmented splits a single input file into 2-hour segments with full transcoding.
// Used when the audio codec is not Sonos-compatible.
func (t *Transcoder) TranscodeSegmented(ctx context.Context, inputPath, outputDir string) (*SegmentedResult, error) {
	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, ErrFFmpegNotFound
	}

	// Verify input file exists
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInputFileNotFound
		}
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check disk space
	estimatedSize := inputInfo.Size()
	if err := t.CheckDiskSpace(outputDir, estimatedSize); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsufficientDiskSpace, err)
	}

	// Get total duration
	totalDuration, err := t.GetDuration(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get input duration: %w", err)
	}

	// Calculate number of segments
	segmentCount := (totalDuration + SegmentDuration - 1) / SegmentDuration
	if segmentCount < 1 {
		segmentCount = 1
	}

	slog.Debug("creating segmented transcode",
		"input_path", inputPath,
		"total_duration", totalDuration,
		"segment_count", segmentCount,
		"segment_duration", SegmentDuration)

	var totalSize int64

	// Create each segment
	for i := 0; i < segmentCount; i++ {
		startTime := i * SegmentDuration
		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%03d.mp3", i))
		tempPath := segmentPath + ".tmp"

		// Calculate actual segment duration (last segment may be shorter)
		segmentDuration := SegmentDuration
		if startTime+segmentDuration > totalDuration {
			segmentDuration = totalDuration - startTime
		}

		slog.Debug("transcoding segment",
			"segment_index", i,
			"start_time", startTime,
			"segment_duration", segmentDuration,
			"output_path", segmentPath)

		// Build ffmpeg command for this segment (full transcode)
		args := []string{
			"-ss", strconv.Itoa(startTime),
			"-i", inputPath,
			"-t", strconv.Itoa(segmentDuration),
			"-map", "0:a",
			"-map_chapters", "-1",
			"-vn",
			"-ar", strconv.Itoa(t.profile.SampleRate),
			"-ac", strconv.Itoa(t.profile.Channels),
			"-b:a", t.profile.Bitrate,
			"-f", t.profile.OutputFormat,
			"-y", tempPath,
		}

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			os.Remove(tempPath)

			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode := exitErr.ExitCode()
				outputStr := string(output)
				parsedErr := ParseFFmpegExitCode(exitCode, outputStr)

				return nil, &TranscodeError{
					ExitCode: exitCode,
					Output:   truncateOutput(outputStr, 500),
					Err:      fmt.Errorf("segment %d: %w", i, parsedErr),
				}
			}

			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			return nil, &TranscodeError{
				Err: fmt.Errorf("ffmpeg segment %d failed: %w", i, err),
			}
		}

		// Atomic rename
		if err := os.Rename(tempPath, segmentPath); err != nil {
			os.Remove(tempPath)
			return nil, fmt.Errorf("failed to rename segment %d: %w", i, err)
		}

		// Get segment size
		info, err := os.Stat(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat segment %d: %w", i, err)
		}
		totalSize += info.Size()

		slog.Debug("segment transcoded",
			"segment_index", i,
			"size_bytes", info.Size())
	}

	slog.Info("segmented transcode complete",
		"input_path", inputPath,
		"segment_count", segmentCount,
		"total_size", totalSize)

	return &SegmentedResult{
		DurationSec:        totalDuration,
		SegmentCount:       segmentCount,
		SegmentDurationSec: SegmentDuration,
		TotalSize:          totalSize,
	}, nil
}

// SmartTranscodeSegmented intelligently chooses between remux or transcode for segmented output.
// Returns the output format used ("mp4" for M4A, "mp3" for MP3).
func (t *Transcoder) SmartTranscodeSegmented(ctx context.Context, inputPath, outputDir string) (*SegmentedResult, string, error) {
	// Detect input format
	detector := NewFormatDetector()
	format, err := detector.Detect(ctx, inputPath)
	if err != nil {
		// If format detection fails, fall back to transcode
		slog.Debug("format detection failed, using transcode fallback", "path", inputPath, "error", err)
		result, err := t.TranscodeSegmented(ctx, inputPath, outputDir)
		return result, "mp3", err
	}

	slog.Debug("audio format detected for segmented processing", "format", format.String())

	// Check compatibility
	checker := NewCompatibilityChecker()
	compatibility := checker.Check(format)

	switch compatibility {
	case Compatible, NeedsRemux:
		// Can use fast remux path
		slog.Debug("using fast-path: segmented remux",
			"container", format.Container,
			"codec", format.AudioCodec)
		targetFormat := checker.GetTargetFormat(format)
		result, err := t.RemuxSegmented(ctx, inputPath, outputDir, targetFormat)
		return result, targetFormat, err

	case NeedsTranscode:
		// Full transcoding required
		slog.Debug("using slow-path: segmented transcode",
			"container", format.Container,
			"codec", format.AudioCodec,
			"reason", "codec not compatible")
		result, err := t.TranscodeSegmented(ctx, inputPath, outputDir)
		return result, "mp3", err

	default:
		// Unknown compatibility, fall back to transcode
		slog.Debug("using slow-path: segmented transcode (unknown compatibility)",
			"container", format.Container,
			"codec", format.AudioCodec)
		result, err := t.TranscodeSegmented(ctx, inputPath, outputDir)
		return result, "mp3", err
	}
}

// SmartTranscodeMultiple intelligently handles multiple files.
func (t *Transcoder) SmartTranscodeMultiple(ctx context.Context, inputPaths []string, outputPath string) (*TranscodeResult, error) {
	if len(inputPaths) == 0 {
		return nil, ErrInputFileNotFound
	}

	// If only one file, use regular smart transcode
	if len(inputPaths) == 1 {
		return t.SmartTranscode(ctx, inputPaths[0], outputPath)
	}

	slog.Debug("analyzing multiple files for smart transcode", "file_count", len(inputPaths))

	// Detect formats for all input files
	detector := NewFormatDetector()
	checker := NewCompatibilityChecker()

	var formats []*AudioFormat
	var firstCodec string
	allSameCodec := true
	canRemux := true

	for i, path := range inputPaths {
		format, err := detector.Detect(ctx, path)
		if err != nil {
			// If any file fails detection, fall back to transcode
			slog.Debug("format detection failed for file, using transcode fallback",
				"file_index", i,
				"path", path,
				"error", err)
			return t.TranscodeMultiple(ctx, inputPaths, outputPath)
		}
		formats = append(formats, format)

		if i == 0 {
			firstCodec = format.AudioCodec
			slog.Debug("first file format", "format", format.String())
		} else if format.AudioCodec != firstCodec {
			allSameCodec = false
			slog.Debug("codec mismatch detected",
				"file_index", i,
				"expected_codec", firstCodec,
				"actual_codec", format.AudioCodec)
		}

		// Check if this file needs transcoding
		if checker.Check(format) == NeedsTranscode {
			canRemux = false
			slog.Debug("file requires transcoding",
				"file_index", i,
				"container", format.Container,
				"codec", format.AudioCodec)
		}
	}

	// If all files have the same codec and can be remuxed, use RemuxMultiple
	if allSameCodec && canRemux {
		slog.Debug("using fast-path: remux multiple",
			"file_count", len(inputPaths),
			"codec", firstCodec,
			"target_format", checker.GetTargetFormat(formats[0]))
		targetFormat := checker.GetTargetFormat(formats[0])
		return t.RemuxMultiple(ctx, inputPaths, outputPath, targetFormat)
	}

	// Otherwise, fall back to transcode (which re-encodes everything to a common format)
	slog.Debug("using slow-path: transcode multiple",
		"file_count", len(inputPaths),
		"same_codec", allSameCodec,
		"can_remux", canRemux)
	return t.TranscodeMultiple(ctx, inputPaths, outputPath)
}
