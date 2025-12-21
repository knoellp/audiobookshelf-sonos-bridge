package cache

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Job represents a transcoding job.
type Job struct {
	ItemID      string
	SourcePath  string   // Deprecated: use SourcePaths
	SourcePaths []string // Multiple source files to concatenate
}

// Worker manages transcoding jobs with a configurable worker pool.
type Worker struct {
	index      *Index
	transcoder *Transcoder
	jobs       chan Job
	workers    int
	wg         sync.WaitGroup
	cancel     context.CancelFunc
}

// NewWorker creates a new worker pool.
func NewWorker(index *Index, transcoder *Transcoder, workers int) *Worker {
	return &Worker{
		index:      index,
		transcoder: transcoder,
		jobs:       make(chan Job, 100), // Buffer for 100 jobs
		workers:    workers,
	}
}

// Start starts the worker pool.
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.worker(ctx, i)
	}

	slog.Info("cache worker pool started", "workers", w.workers)
}

// Stop stops the worker pool gracefully.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	close(w.jobs)
	w.wg.Wait()
	slog.Info("cache worker pool stopped")
}

// Enqueue adds a job to the queue.
func (w *Worker) Enqueue(job Job) bool {
	select {
	case w.jobs <- job:
		return true
	default:
		slog.Warn("job queue full, dropping job", "item_id", job.ItemID)
		return false
	}
}

// QueueLength returns the current number of jobs in the queue.
func (w *Worker) QueueLength() int {
	return len(w.jobs)
}

// worker is the worker goroutine.
func (w *Worker) worker(ctx context.Context, id int) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-w.jobs:
			if !ok {
				return
			}
			w.processJob(ctx, job)
		}
	}
}

// processJob processes a single transcoding job.
func (w *Worker) processJob(ctx context.Context, job Job) {
	startTime := time.Now()

	// Get source paths (support both old single path and new multiple paths)
	sourcePaths := job.SourcePaths
	if len(sourcePaths) == 0 && job.SourcePath != "" {
		sourcePaths = []string{job.SourcePath}
	}

	slog.Debug("starting transcoding job", "item_id", job.ItemID, "source_count", len(sourcePaths))

	// Mark as in progress
	if err := w.index.MarkInProgress(job.ItemID); err != nil {
		slog.Error("failed to mark job in progress", "item_id", job.ItemID, "error", err)
		return
	}

	// Create output directory
	if err := w.index.EnsureDirectory(job.ItemID); err != nil {
		w.index.MarkFailed(job.ItemID, err.Error())
		slog.Error("failed to create cache directory", "item_id", job.ItemID, "error", err)
		return
	}

	// Check if we need segmented processing (for files > 2 hours)
	// This is required for ZP90/Sonos Connect which has a ~128MB RAM limit
	if w.needsSegmentation(ctx, sourcePaths) {
		w.processJobSegmented(ctx, job, sourcePaths, startTime)
		return
	}

	// Standard processing for shorter files
	w.processJobStandard(ctx, job, sourcePaths, startTime)
}

// needsSegmentation checks if the source files require segmented processing.
// Returns true if total duration exceeds 2 hours (SegmentDuration).
func (w *Worker) needsSegmentation(ctx context.Context, sourcePaths []string) bool {
	if len(sourcePaths) == 0 {
		return false
	}

	var totalDuration int
	for _, path := range sourcePaths {
		duration, err := w.transcoder.GetDuration(ctx, path)
		if err != nil {
			slog.Debug("failed to get duration, assuming no segmentation needed", "path", path, "error", err)
			continue
		}
		totalDuration += duration
	}

	needsSegment := totalDuration > SegmentDuration
	if needsSegment {
		slog.Debug("file requires segmentation",
			"total_duration", totalDuration,
			"segment_threshold", SegmentDuration,
			"segment_count", (totalDuration+SegmentDuration-1)/SegmentDuration)
	}

	return needsSegment
}

// processJobStandard handles standard (non-segmented) transcoding.
func (w *Worker) processJobStandard(ctx context.Context, job Job, sourcePaths []string, startTime time.Time) {
	// Determine target format based on input files
	targetFormat := w.determineTargetFormat(ctx, sourcePaths)
	slog.Debug("determined target format", "item_id", job.ItemID, "format", targetFormat)

	// Get output path with correct format
	outputPath := w.index.GetCachePathWithFormat(job.ItemID, targetFormat)

	// SmartTranscode: intelligently chooses between remux and transcode
	result, err := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, outputPath)
	if err != nil {
		w.index.MarkFailed(job.ItemID, err.Error())
		slog.Error("transcoding failed", "item_id", job.ItemID, "error", err)
		return
	}

	// Mark as ready with the target format
	if err := w.index.MarkReadyWithFormat(job.ItemID, result.DurationSec, targetFormat); err != nil {
		slog.Error("failed to mark job ready", "item_id", job.ItemID, "error", err)
		return
	}

	duration := time.Since(startTime)
	slog.Info("transcoding complete",
		"item_id", job.ItemID,
		"duration_sec", result.DurationSec,
		"output_size", result.OutputSize,
		"output_format", targetFormat,
		"transcode_time", duration,
		"source_files", len(sourcePaths),
	)
}

// processJobSegmented handles segmented transcoding for long files.
func (w *Worker) processJobSegmented(ctx context.Context, job Job, sourcePaths []string, startTime time.Time) {
	slog.Info("using segmented processing for long file",
		"item_id", job.ItemID,
		"source_count", len(sourcePaths))

	// Get output directory
	outputDir := w.index.GetCacheDir(job.ItemID)

	// For multiple files, we need to first concatenate, then segment
	// For a single file, we can segment directly
	var result *SegmentedResult
	var outputFormat string
	var err error

	if len(sourcePaths) == 1 {
		// Single file - segment directly
		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, sourcePaths[0], outputDir)
	} else {
		// Multiple files - first concatenate to temp file, then segment
		// For now, concatenate and transcode (this handles multi-file to segments)
		tempPath := outputDir + "/concat_temp.tmp"
		concatResult, concatErr := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, tempPath)
		if concatErr != nil {
			w.index.MarkFailed(job.ItemID, concatErr.Error())
			slog.Error("concatenation failed", "item_id", job.ItemID, "error", concatErr)
			return
		}

		// Now segment the concatenated file
		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, tempPath, outputDir)

		// Clean up temp file
		os.Remove(tempPath)

		// Override duration from concat result (more accurate)
		if result != nil {
			result.DurationSec = concatResult.DurationSec
		}
	}

	if err != nil {
		w.index.MarkFailed(job.ItemID, err.Error())
		slog.Error("segmented transcoding failed", "item_id", job.ItemID, "error", err)
		return
	}

	// Mark as ready with segment information
	if err := w.index.MarkReadyWithSegments(job.ItemID, result.DurationSec, outputFormat, result.SegmentCount, result.SegmentDurationSec); err != nil {
		slog.Error("failed to mark segmented job ready", "item_id", job.ItemID, "error", err)
		return
	}

	duration := time.Since(startTime)
	slog.Info("segmented transcoding complete",
		"item_id", job.ItemID,
		"duration_sec", result.DurationSec,
		"segment_count", result.SegmentCount,
		"segment_duration", result.SegmentDurationSec,
		"total_size", result.TotalSize,
		"output_format", outputFormat,
		"transcode_time", duration,
		"source_files", len(sourcePaths),
	)
}

// TranscodeSync performs synchronous transcoding (for on-demand).
// Deprecated: Use TranscodeSyncMultiple for new code.
func (w *Worker) TranscodeSync(ctx context.Context, itemID, sourcePath string) error {
	return w.TranscodeSyncMultiple(ctx, itemID, []string{sourcePath})
}

// TranscodeSyncMultiple performs synchronous transcoding of multiple files.
func (w *Worker) TranscodeSyncMultiple(ctx context.Context, itemID string, sourcePaths []string) error {
	slog.Debug("starting sync transcoding", "item_id", itemID, "source_count", len(sourcePaths))

	// Mark as in progress
	if err := w.index.MarkInProgress(itemID); err != nil {
		return err
	}

	// Create output directory
	if err := w.index.EnsureDirectory(itemID); err != nil {
		w.index.MarkFailed(itemID, err.Error())
		return err
	}

	// Check if we need segmented processing (for files > 2 hours)
	if w.needsSegmentation(ctx, sourcePaths) {
		return w.transcodeSyncSegmented(ctx, itemID, sourcePaths)
	}

	// Standard processing for shorter files
	targetFormat := w.determineTargetFormat(ctx, sourcePaths)
	slog.Debug("determined target format", "item_id", itemID, "format", targetFormat)

	// Get output path with correct format
	outputPath := w.index.GetCachePathWithFormat(itemID, targetFormat)

	// SmartTranscode: intelligently chooses between remux and transcode
	result, err := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, outputPath)
	if err != nil {
		w.index.MarkFailed(itemID, err.Error())
		return err
	}

	slog.Info("sync transcoding complete",
		"item_id", itemID,
		"duration_sec", result.DurationSec,
		"output_size", result.OutputSize,
		"output_format", targetFormat,
		"source_files", len(sourcePaths),
	)

	// Mark as ready with format
	return w.index.MarkReadyWithFormat(itemID, result.DurationSec, targetFormat)
}

// transcodeSyncSegmented performs synchronous segmented transcoding.
func (w *Worker) transcodeSyncSegmented(ctx context.Context, itemID string, sourcePaths []string) error {
	slog.Info("using segmented sync processing for long file",
		"item_id", itemID,
		"source_count", len(sourcePaths))

	outputDir := w.index.GetCacheDir(itemID)

	var result *SegmentedResult
	var outputFormat string
	var err error

	if len(sourcePaths) == 1 {
		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, sourcePaths[0], outputDir)
	} else {
		// Multiple files - first concatenate to temp file, then segment
		tempPath := outputDir + "/concat_temp.tmp"
		concatResult, concatErr := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, tempPath)
		if concatErr != nil {
			w.index.MarkFailed(itemID, concatErr.Error())
			return concatErr
		}

		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, tempPath, outputDir)
		os.Remove(tempPath)

		if result != nil {
			result.DurationSec = concatResult.DurationSec
		}
	}

	if err != nil {
		w.index.MarkFailed(itemID, err.Error())
		return err
	}

	slog.Info("segmented sync transcoding complete",
		"item_id", itemID,
		"duration_sec", result.DurationSec,
		"segment_count", result.SegmentCount,
		"total_size", result.TotalSize,
		"output_format", outputFormat,
	)

	return w.index.MarkReadyWithSegments(itemID, result.DurationSec, outputFormat, result.SegmentCount, result.SegmentDurationSec)
}

// determineTargetFormat analyzes input files and returns the optimal output format.
func (w *Worker) determineTargetFormat(ctx context.Context, sourcePaths []string) string {
	if len(sourcePaths) == 0 {
		return "mp3" // default
	}

	detector := NewFormatDetector()
	checker := NewCompatibilityChecker()

	// Check first file to determine format
	format, err := detector.Detect(ctx, sourcePaths[0])
	if err != nil {
		slog.Debug("format detection failed, using mp3 default", "error", err)
		return "mp3"
	}

	// If codec is compatible (can be remuxed), use the optimal target format
	if checker.Check(format) != NeedsTranscode {
		targetFormat := checker.GetTargetFormat(format)
		slog.Debug("using remux-compatible format",
			"input_codec", format.AudioCodec,
			"input_container", format.Container,
			"target_format", targetFormat,
		)
		return targetFormat
	}

	// Needs full transcode - output will be mp3
	slog.Debug("needs transcoding, using mp3",
		"input_codec", format.AudioCodec,
		"input_container", format.Container,
	)
	return "mp3"
}

