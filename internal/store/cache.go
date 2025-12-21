package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CacheStatus represents the status of a cache entry.
type CacheStatus string

const (
	CacheStatusPending    CacheStatus = "pending"
	CacheStatusInProgress CacheStatus = "in_progress"
	CacheStatusReady      CacheStatus = "ready"
	CacheStatusFailed     CacheStatus = "failed"
)

// CacheEntry represents an entry in the cache index.
type CacheEntry struct {
	ItemID             string
	SourcePath         string
	SourceSize         int64
	SourceMtime        time.Time
	ProfileVersion     string
	CachePath          string
	CacheFormat        string // "mp3", "mp4", "flac", "ogg" - the actual output format
	DurationSec        *int   // nil if not yet known
	SegmentCount       int    // Number of segments (1 for single-file, >1 for segmented)
	SegmentDurationSec int    // Duration of each segment in seconds (e.g., 7200 for 2h)
	Status             CacheStatus
	ErrorText          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsSegmented returns true if the cache entry uses multiple segments.
func (e *CacheEntry) IsSegmented() bool {
	return e.SegmentCount > 1
}

// GetSegmentFileName returns the filename for a specific segment index.
func (e *CacheEntry) GetSegmentFileName(segmentIndex int) string {
	if !e.IsSegmented() {
		// Single file - use standard naming
		switch e.CacheFormat {
		case "mp4":
			return "audio.m4a"
		case "mp3":
			return "audio.mp3"
		case "flac":
			return "audio.flac"
		default:
			return "audio.mp3"
		}
	}
	// Segmented - use segment naming
	ext := ".m4a"
	switch e.CacheFormat {
	case "mp3":
		ext = ".mp3"
	case "flac":
		ext = ".flac"
	}
	return fmt.Sprintf("segment_%03d%s", segmentIndex, ext)
}

// GlobalToSegment converts a global position to segment index and local position.
func GlobalToSegment(globalPosSec, segmentDurationSec int) (segmentIndex int, localPosSec int) {
	if segmentDurationSec <= 0 {
		return 0, globalPosSec
	}
	segmentIndex = globalPosSec / segmentDurationSec
	localPosSec = globalPosSec % segmentDurationSec
	return
}

// SegmentToGlobal converts segment index and local position to global position.
func SegmentToGlobal(segmentIndex, localPosSec, segmentDurationSec int) int {
	return segmentIndex*segmentDurationSec + localPosSec
}

// CacheStore provides CRUD operations for the cache index.
type CacheStore struct {
	db *sql.DB
}

// NewCacheStore creates a new cache store.
func NewCacheStore(db *DB) *CacheStore {
	return &CacheStore{db: db.Conn()}
}

// Create inserts a new cache entry.
func (s *CacheStore) Create(entry *CacheEntry) error {
	query := `
		INSERT INTO cache_index (item_id, source_path, source_size, source_mtime, profile_version, cache_path, cache_format, duration_sec, segment_count, segment_duration_sec, status, error_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Unix()
	cacheFormat := entry.CacheFormat
	if cacheFormat == "" {
		cacheFormat = "mp3" // default for backwards compatibility
	}
	segmentCount := entry.SegmentCount
	if segmentCount == 0 {
		segmentCount = 1 // default for single-file cache
	}
	_, err := s.db.Exec(query,
		entry.ItemID,
		entry.SourcePath,
		entry.SourceSize,
		entry.SourceMtime.Unix(),
		entry.ProfileVersion,
		entry.CachePath,
		cacheFormat,
		entry.DurationSec,
		segmentCount,
		entry.SegmentDurationSec,
		string(entry.Status),
		entry.ErrorText,
		now,
		now,
	)
	return err
}

// Get retrieves a cache entry by item ID.
func (s *CacheStore) Get(itemID string) (*CacheEntry, error) {
	query := `
		SELECT item_id, source_path, source_size, source_mtime, profile_version, cache_path, cache_format, duration_sec, segment_count, segment_duration_sec, status, error_text, created_at, updated_at
		FROM cache_index WHERE item_id = ?
	`
	row := s.db.QueryRow(query, itemID)

	var entry CacheEntry
	var sourceMtime, createdAt, updatedAt int64
	var durationSec sql.NullInt64
	var segmentCount sql.NullInt64
	var segmentDurationSec sql.NullInt64
	var cacheFormat sql.NullString
	var errorText sql.NullString
	var status string

	err := row.Scan(
		&entry.ItemID,
		&entry.SourcePath,
		&entry.SourceSize,
		&sourceMtime,
		&entry.ProfileVersion,
		&entry.CachePath,
		&cacheFormat,
		&durationSec,
		&segmentCount,
		&segmentDurationSec,
		&status,
		&errorText,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	entry.SourceMtime = time.Unix(sourceMtime, 0)
	entry.CreatedAt = time.Unix(createdAt, 0)
	entry.UpdatedAt = time.Unix(updatedAt, 0)
	entry.Status = CacheStatus(status)
	if cacheFormat.Valid {
		entry.CacheFormat = cacheFormat.String
	} else {
		entry.CacheFormat = "mp3" // default for old entries
	}
	if errorText.Valid {
		entry.ErrorText = errorText.String
	}
	if durationSec.Valid {
		dur := int(durationSec.Int64)
		entry.DurationSec = &dur
	}
	if segmentCount.Valid {
		entry.SegmentCount = int(segmentCount.Int64)
	} else {
		entry.SegmentCount = 1 // default for old entries
	}
	if segmentDurationSec.Valid {
		entry.SegmentDurationSec = int(segmentDurationSec.Int64)
	}

	return &entry, nil
}

// UpdateStatus updates the status of a cache entry.
func (s *CacheStore) UpdateStatus(itemID string, status CacheStatus, errorText string) error {
	query := `UPDATE cache_index SET status = ?, error_text = ?, updated_at = ? WHERE item_id = ?`
	_, err := s.db.Exec(query, string(status), errorText, time.Now().Unix(), itemID)
	return err
}

// MarkReady marks a cache entry as ready and sets the duration.
func (s *CacheStore) MarkReady(itemID string, durationSec int) error {
	query := `UPDATE cache_index SET status = ?, duration_sec = ?, error_text = '', updated_at = ? WHERE item_id = ?`
	_, err := s.db.Exec(query, string(CacheStatusReady), durationSec, time.Now().Unix(), itemID)
	return err
}

// MarkReadyWithFormat marks a cache entry as ready and sets the duration and format.
func (s *CacheStore) MarkReadyWithFormat(itemID string, durationSec int, cacheFormat string) error {
	query := `UPDATE cache_index SET status = ?, duration_sec = ?, cache_format = ?, error_text = '', updated_at = ? WHERE item_id = ?`
	_, err := s.db.Exec(query, string(CacheStatusReady), durationSec, cacheFormat, time.Now().Unix(), itemID)
	return err
}

// MarkReadyWithSegments marks a cache entry as ready with segment information.
func (s *CacheStore) MarkReadyWithSegments(itemID string, durationSec int, cacheFormat string, segmentCount int, segmentDurationSec int) error {
	query := `UPDATE cache_index SET status = ?, duration_sec = ?, cache_format = ?, segment_count = ?, segment_duration_sec = ?, error_text = '', updated_at = ? WHERE item_id = ?`
	_, err := s.db.Exec(query, string(CacheStatusReady), durationSec, cacheFormat, segmentCount, segmentDurationSec, time.Now().Unix(), itemID)
	return err
}

// UpdateCacheFormat updates the cache format of an entry.
func (s *CacheStore) UpdateCacheFormat(itemID string, cacheFormat string) error {
	query := `UPDATE cache_index SET cache_format = ?, updated_at = ? WHERE item_id = ?`
	_, err := s.db.Exec(query, cacheFormat, time.Now().Unix(), itemID)
	return err
}

// MarkFailed marks a cache entry as failed with an error message.
func (s *CacheStore) MarkFailed(itemID string, errorText string) error {
	return s.UpdateStatus(itemID, CacheStatusFailed, errorText)
}

// MarkInProgress marks a cache entry as in progress.
func (s *CacheStore) MarkInProgress(itemID string) error {
	return s.UpdateStatus(itemID, CacheStatusInProgress, "")
}

// Delete removes a cache entry by item ID.
func (s *CacheStore) Delete(itemID string) error {
	query := `DELETE FROM cache_index WHERE item_id = ?`
	_, err := s.db.Exec(query, itemID)
	return err
}

// ListByStatus returns all cache entries with the given status.
func (s *CacheStore) ListByStatus(status CacheStatus) ([]*CacheEntry, error) {
	query := `
		SELECT item_id, source_path, source_size, source_mtime, profile_version, cache_path, cache_format, duration_sec, segment_count, segment_duration_sec, status, error_text, created_at, updated_at
		FROM cache_index WHERE status = ? ORDER BY created_at
	`
	rows, err := s.db.Query(query, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// ListAll returns all cache entries.
func (s *CacheStore) ListAll() ([]*CacheEntry, error) {
	query := `
		SELECT item_id, source_path, source_size, source_mtime, profile_version, cache_path, cache_format, duration_sec, segment_count, segment_duration_sec, status, error_text, created_at, updated_at
		FROM cache_index ORDER BY created_at
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// ResetInProgressToPending resets all in_progress entries to pending (used on startup).
func (s *CacheStore) ResetInProgressToPending() (int64, error) {
	query := `UPDATE cache_index SET status = ? WHERE status = ?`
	result, err := s.db.Exec(query, string(CacheStatusPending), string(CacheStatusInProgress))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteByProfile deletes all cache entries with a specific profile version.
func (s *CacheStore) DeleteByProfile(profileVersion string) (int64, error) {
	query := `DELETE FROM cache_index WHERE profile_version = ?`
	result, err := s.db.Exec(query, profileVersion)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *CacheStore) scanEntries(rows *sql.Rows) ([]*CacheEntry, error) {
	var entries []*CacheEntry
	for rows.Next() {
		var entry CacheEntry
		var sourceMtime, createdAt, updatedAt int64
		var durationSec sql.NullInt64
		var segmentCount sql.NullInt64
		var segmentDurationSec sql.NullInt64
		var cacheFormat sql.NullString
		var errorText sql.NullString
		var status string

		err := rows.Scan(
			&entry.ItemID,
			&entry.SourcePath,
			&entry.SourceSize,
			&sourceMtime,
			&entry.ProfileVersion,
			&entry.CachePath,
			&cacheFormat,
			&durationSec,
			&segmentCount,
			&segmentDurationSec,
			&status,
			&errorText,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		entry.SourceMtime = time.Unix(sourceMtime, 0)
		entry.CreatedAt = time.Unix(createdAt, 0)
		entry.UpdatedAt = time.Unix(updatedAt, 0)
		entry.Status = CacheStatus(status)
		if cacheFormat.Valid {
			entry.CacheFormat = cacheFormat.String
		} else {
			entry.CacheFormat = "mp3" // default for old entries
		}
		if errorText.Valid {
			entry.ErrorText = errorText.String
		}
		if durationSec.Valid {
			dur := int(durationSec.Int64)
			entry.DurationSec = &dur
		}
		if segmentCount.Valid {
			entry.SegmentCount = int(segmentCount.Int64)
		} else {
			entry.SegmentCount = 1 // default for old entries
		}
		if segmentDurationSec.Valid {
			entry.SegmentDurationSec = int(segmentDurationSec.Int64)
		}
		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}
