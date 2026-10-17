package cache

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// jobState is the lifecycle state of an in-flight job.
type jobState int

const (
	stateQueued   jobState = iota // waiting in the regular queue
	statePromoted                 // waiting in the priority queue
	stateRunning                  // picked up by a worker
)

// isCanceled reports whether err stems from a canceled or expired context.
func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Job represents a transcoding job.
type Job struct {
	ItemID      string
	SourcePath  string   // Deprecated: use SourcePaths
	SourcePaths []string // Multiple source files to concatenate
}

// Job phases reported via JobProgress.
const (
	PhaseAnalyzing  = "analyzing"
	PhaseRemux      = "remux"
	PhaseTranscode  = "transcode"
	PhaseSegmenting = "segmenting"
)

// progressInterval is how often the progress estimate is recomputed.
var progressInterval = time.Second

// JobProgress describes the progress of a running (or queued) job.
type JobProgress struct {
	Phase     string
	Percent   int
	StartedAt time.Time
}

// Worker manages transcoding jobs with a configurable worker pool.
type Worker struct {
	index        *Index
	transcoder   *Transcoder
	jobs         chan Job
	priorityJobs chan Job
	workers      int
	wg           sync.WaitGroup
	cancel       context.CancelFunc

	onStart func(Job) // test hook, called when a job is picked up

	mu       sync.Mutex
	stopped  bool
	inflight map[string]jobState
	progress map[string]*JobProgress
}

// NewWorker creates a new worker pool.
func NewWorker(index *Index, transcoder *Transcoder, workers int) *Worker {
	return &Worker{
		index:        index,
		transcoder:   transcoder,
		jobs:         make(chan Job, 500), // Buffer for 500 jobs
		priorityJobs: make(chan Job, 50),
		workers:      workers,
		inflight:     make(map[string]jobState),
		progress:     make(map[string]*JobProgress),
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
	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	slog.Info("cache worker pool stopped")
}

// Enqueue adds a job to the queue. Returns false if the item is already
// queued or running, or if the queue is full.
func (w *Worker) Enqueue(job Job) bool {
	return w.enqueue(job, false)
}

// EnqueuePriority adds a job to the priority queue (user-initiated requests).
// Priority jobs are picked up before regular jobs. If the item is already
// waiting in the regular queue it is promoted to the priority queue.
func (w *Worker) EnqueuePriority(job Job) bool {
	return w.enqueue(job, true)
}

func (w *Worker) enqueue(job Job, priority bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		slog.Debug("worker stopped, rejecting job", "item_id", job.ItemID)
		return false
	}
	if w.inflight == nil {
		w.inflight = make(map[string]jobState)
	}

	if st, ok := w.inflight[job.ItemID]; ok {
		if priority && st == stateQueued {
			// Promote: push a copy to the priority queue. The copy in the
			// regular queue is skipped by the worker loop.
			select {
			case w.priorityJobs <- job:
				w.inflight[job.ItemID] = statePromoted
				slog.Debug("queued job promoted to priority", "item_id", job.ItemID)
				return true
			default:
				slog.Warn("priority queue full, cannot promote job", "item_id", job.ItemID)
				return false
			}
		}
		slog.Debug("job already queued or running, skipping", "item_id", job.ItemID)
		return false
	}

	ch, state := w.jobs, stateQueued
	if priority {
		ch, state = w.priorityJobs, statePromoted
	}
	select {
	case ch <- job:
		w.inflight[job.ItemID] = state
		return true
	default:
		slog.Warn("job queue full, dropping job", "item_id", job.ItemID)
		return false
	}
}

// claim transitions a job taken from a queue to running. It returns false if
// the job must be skipped (stale copy of a promoted or finished job).
func (w *Worker) claim(job Job, fromPriority bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	st, ok := w.inflight[job.ItemID]
	if !ok {
		return false
	}
	if !fromPriority && st != stateQueued {
		return false
	}
	w.inflight[job.ItemID] = stateRunning
	return true
}

// IsActive reports whether a job for the item is queued or running.
func (w *Worker) IsActive(itemID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.inflight[itemID]
	return ok
}

// Progress returns the progress of a running job, if any.
func (w *Worker) Progress(itemID string) (JobProgress, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.progress[itemID]
	if !ok {
		return JobProgress{}, false
	}
	return *p, true
}

func (w *Worker) setPhase(itemID, phase string) {
	w.mu.Lock()
	if p, ok := w.progress[itemID]; ok {
		p.Phase = phase
	}
	w.mu.Unlock()
}

// trackProgress starts a goroutine that estimates the progress of a job from
// the bytes written to its cache directory. The returned function stops it.
//
// If expectedOverride is > 0 it is used as expected output size (transcodes,
// where the output size is unrelated to the input size).
func (w *Worker) trackProgress(itemID string, sourcePaths []string, segmented bool, expectedOverride int64) func() {
	w.mu.Lock()
	w.progress[itemID] = &JobProgress{Phase: PhaseAnalyzing, StartedAt: time.Now()}
	w.mu.Unlock()

	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()

		var expected int64
		for _, sp := range sourcePaths {
			if fi, err := os.Stat(sp); err == nil {
				expected += fi.Size()
			}
		}
		if expectedOverride > 0 {
			expected = expectedOverride
		}
		// Multiple files are concatenated into a temp file first and then
		// segmented, so roughly twice the data is written.
		if segmented && len(sourcePaths) > 1 {
			expected *= 2
		}
		dir := w.index.GetCacheDir(itemID)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if expected <= 0 {
					continue
				}
				var written int64
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					if fi, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
						written += fi.Size()
					}
				}
				pct := int(written * 100 / expected)
				if pct > 99 {
					pct = 99
				}
				w.mu.Lock()
				if p, ok := w.progress[itemID]; ok && pct > p.Percent {
					p.Percent = pct
				}
				w.mu.Unlock()
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
		w.mu.Lock()
		delete(w.progress, itemID)
		w.mu.Unlock()
	}
}

// QueueLength returns the current number of jobs in the queue.
func (w *Worker) QueueLength() int {
	return len(w.jobs) + len(w.priorityJobs)
}

// worker is the worker goroutine.
func (w *Worker) worker(ctx context.Context, id int) {
	defer w.wg.Done()

	for {
		// Priority jobs first
		select {
		case <-ctx.Done():
			return
		case job := <-w.priorityJobs:
			if w.claim(job, true) {
				w.processJob(ctx, job)
			}
			continue
		default:
		}

		select {
		case <-ctx.Done():
			return
		case job := <-w.priorityJobs:
			if w.claim(job, true) {
				w.processJob(ctx, job)
			}
		case job := <-w.jobs:
			if w.claim(job, false) {
				w.processJob(ctx, job)
			}
		}
	}
}

// processJob processes a single transcoding job.
func (w *Worker) processJob(ctx context.Context, job Job) {
	startTime := time.Now()

	if w.onStart != nil {
		w.onStart(job)
	}

	defer func() {
		w.mu.Lock()
		delete(w.inflight, job.ItemID)
		w.mu.Unlock()
	}()

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

	// Remove stale outputs of earlier (failed or interrupted) runs
	w.cleanStaleOutputs(job.ItemID)

	// Check if we need segmented processing (for files > 2 hours)
	// This is required for ZP90/Sonos Connect which has a ~128MB RAM limit
	totalDuration, err := w.estimateTotalDuration(ctx, sourcePaths)
	if err != nil {
		if isCanceled(err) {
			w.index.MarkPending(job.ItemID)
			slog.Info("job canceled", "item_id", job.ItemID)
			return
		}
		w.index.MarkFailed(job.ItemID, "duration detection failed")
		slog.Error("duration detection failed", "item_id", job.ItemID, "error", err)
		return
	}
	segmented := totalDuration > SegmentDuration

	var expectedOverride int64
	if w.determineTargetFormat(ctx, sourcePaths) == "mp3" {
		expectedOverride = w.transcoder.EstimateOutputSize(totalDuration)
	}
	stop := w.trackProgress(job.ItemID, sourcePaths, segmented, expectedOverride)
	defer stop()

	if segmented {
		w.processJobSegmented(ctx, job, sourcePaths, startTime)
		return
	}

	// Standard processing for shorter files
	w.processJobStandard(ctx, job, sourcePaths, startTime)
}

// estimateTotalDuration returns the total duration of all source files in
// seconds. Files whose duration cannot be determined are estimated from their
// size using the ratio of the files with a known duration. If no duration is
// known at all, an error is returned.
func (w *Worker) estimateTotalDuration(ctx context.Context, sourcePaths []string) (int, error) {
	if len(sourcePaths) == 0 {
		return 0, nil
	}

	var knownDuration int
	var knownSize int64
	var unknownSize int64
	var unknownCount int
	for _, path := range sourcePaths {
		var size int64
		if fi, err := os.Stat(path); err == nil {
			size = fi.Size()
		}
		duration, err := w.transcoder.GetDuration(ctx, path)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			slog.Debug("failed to get duration, will estimate from size", "path", path, "error", err)
			unknownSize += size
			unknownCount++
			continue
		}
		knownDuration += duration
		knownSize += size
	}

	if unknownCount == 0 {
		return knownDuration, nil
	}
	if knownDuration <= 0 || knownSize <= 0 {
		return 0, errors.New("duration detection failed")
	}
	estimated := int(float64(unknownSize) * float64(knownDuration) / float64(knownSize))
	slog.Debug("estimated duration of files with unknown duration",
		"files", unknownCount, "estimated_sec", estimated)
	return knownDuration + estimated, nil
}

// needsSegmentation checks if the source files require segmented processing.
// Returns true if total duration exceeds 2 hours (SegmentDuration).
func (w *Worker) needsSegmentation(ctx context.Context, sourcePaths []string) (bool, error) {
	total, err := w.estimateTotalDuration(ctx, sourcePaths)
	if err != nil {
		return false, err
	}
	return total > SegmentDuration, nil
}

// handleJobError records a job error. A canceled context (shutdown) resets
// the entry to pending so that it is picked up again; anything else fails it.
func (w *Worker) handleJobError(itemID, msg string, err error) {
	if isCanceled(err) {
		if mErr := w.index.MarkPending(itemID); mErr != nil {
			slog.Error("failed to reset canceled job to pending", "item_id", itemID, "error", mErr)
		}
		slog.Info("job canceled, reset to pending", "item_id", itemID)
		return
	}
	w.index.MarkFailed(itemID, err.Error())
	slog.Error(msg, "item_id", itemID, "error", err)
}

// processJobStandard handles standard (non-segmented) transcoding.
func (w *Worker) processJobStandard(ctx context.Context, job Job, sourcePaths []string, startTime time.Time) {
	// Determine target format based on input files
	targetFormat := w.determineTargetFormat(ctx, sourcePaths)
	slog.Debug("determined target format", "item_id", job.ItemID, "format", targetFormat)

	// Get output path with correct format
	outputPath := w.index.GetCachePathWithFormat(job.ItemID, targetFormat)

	// SmartTranscode: intelligently chooses between remux and transcode
	if targetFormat == "mp3" {
		w.setPhase(job.ItemID, PhaseTranscode)
	} else {
		w.setPhase(job.ItemID, PhaseRemux)
	}
	result, actualFormat, err := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, outputPath)
	if err != nil {
		w.handleJobError(job.ItemID, "transcoding failed", err)
		return
	}

	// The actual output format can differ from the guess based on the first
	// file (mixed codecs fall back to MP3). Rename the file so that name,
	// content and cache metadata agree.
	if actualFormat != "" && actualFormat != targetFormat {
		correctPath := w.index.GetCachePathWithFormat(job.ItemID, actualFormat)
		if err := os.Rename(outputPath, correctPath); err != nil {
			w.index.MarkFailed(job.ItemID, err.Error())
			slog.Error("failed to rename output to actual format", "item_id", job.ItemID,
				"from", outputPath, "to", correctPath, "error", err)
			return
		}
		slog.Info("output format differs from initial guess",
			"item_id", job.ItemID, "expected", targetFormat, "actual", actualFormat)
		targetFormat = actualFormat
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

	w.setPhase(job.ItemID, PhaseSegmenting)
	if len(sourcePaths) == 1 {
		// Single file - segment directly
		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, sourcePaths[0], outputDir)
	} else {
		// Multiple files - first concatenate to temp file, then segment
		// For now, concatenate and transcode (this handles multi-file to segments)
		tempPath := outputDir + "/concat_temp.tmp"
		w.setPhase(job.ItemID, PhaseRemux)
		concatResult, _, concatErr := w.transcoder.SmartTranscodeMultiple(ctx, sourcePaths, tempPath)
		if concatErr != nil {
			w.handleJobError(job.ItemID, "concatenation failed", concatErr)
			return
		}

		// Now segment the concatenated file
		w.setPhase(job.ItemID, PhaseSegmenting)
		result, outputFormat, err = w.transcoder.SmartTranscodeSegmented(ctx, tempPath, outputDir)

		// Clean up temp file
		os.Remove(tempPath)

		// Override duration from concat result (more accurate)
		if result != nil {
			result.DurationSec = concatResult.DurationSec
		}
	}

	if err != nil {
		w.handleJobError(job.ItemID, "segmented transcoding failed", err)
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

// cleanStaleOutputs removes leftover output files of earlier runs from the
// item's cache directory (the directory itself is kept).
func (w *Worker) cleanStaleOutputs(itemID string) {
	dir := w.index.GetCacheDir(itemID)
	for _, pattern := range []string{"audio.*", "segment_*.*", "*.tmp", "*.concat.txt"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			if err := os.Remove(m); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to remove stale output", "path", m, "error", err)
			}
		}
	}
}
