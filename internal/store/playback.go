package store

import (
	"database/sql"
	"errors"
	"time"
)

// PlaybackSession represents an active playback session in the database.
type PlaybackSession struct {
	ID                  string
	SessionID           string // Reference to sessions.id
	ItemID              string
	SonosUUID           string
	StreamToken         string
	PositionSec         int    // Global position in seconds (across all segments)
	DurationSec         int    // Total duration in seconds
	CurrentSegment      int    // Current segment index (0-based, for segmented playback)
	SegmentDurationSec  int    // Duration of each segment (e.g., 7200 for 2h)
	IsPlaying           bool
	StartedAt           time.Time
	LastPositionUpdate  time.Time
	ABSProgressSyncedAt time.Time
}

// PlaybackStore provides CRUD operations for playback sessions.
type PlaybackStore struct {
	db *sql.DB
}

// NewPlaybackStore creates a new playback store.
func NewPlaybackStore(db *DB) *PlaybackStore {
	return &PlaybackStore{db: db.Conn()}
}

// Create inserts a new playback session.
func (s *PlaybackStore) Create(ps *PlaybackSession) error {
	query := `
		INSERT INTO playback_sessions (id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	isPlaying := 0
	if ps.IsPlaying {
		isPlaying = 1
	}

	_, err := s.db.Exec(query,
		ps.ID,
		ps.SessionID,
		ps.ItemID,
		ps.SonosUUID,
		ps.StreamToken,
		ps.PositionSec,
		ps.DurationSec,
		ps.CurrentSegment,
		ps.SegmentDurationSec,
		isPlaying,
		ps.StartedAt.Unix(),
		ps.LastPositionUpdate.Unix(),
		ps.ABSProgressSyncedAt.Unix(),
	)
	return err
}

// Get retrieves a playback session by ID.
func (s *PlaybackStore) Get(id string) (*PlaybackSession, error) {
	query := `
		SELECT id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at
		FROM playback_sessions WHERE id = ?
	`
	row := s.db.QueryRow(query, id)
	return s.scanRow(row)
}

// GetBySessionID retrieves the active playback session for a web session.
func (s *PlaybackStore) GetBySessionID(sessionID string) (*PlaybackSession, error) {
	query := `
		SELECT id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at
		FROM playback_sessions WHERE session_id = ? ORDER BY started_at DESC LIMIT 1
	`
	row := s.db.QueryRow(query, sessionID)
	return s.scanRow(row)
}

// GetByToken retrieves a playback session by stream token.
func (s *PlaybackStore) GetByToken(token string) (*PlaybackSession, error) {
	query := `
		SELECT id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at
		FROM playback_sessions WHERE stream_token = ?
	`
	row := s.db.QueryRow(query, token)
	return s.scanRow(row)
}

// UpdatePosition updates the current playback position.
func (s *PlaybackStore) UpdatePosition(id string, positionSec int) error {
	query := `UPDATE playback_sessions SET position_sec = ?, last_position_update = ? WHERE id = ?`
	_, err := s.db.Exec(query, positionSec, time.Now().Unix(), id)
	return err
}

// UpdatePlaying updates the playing state.
func (s *PlaybackStore) UpdatePlaying(id string, isPlaying bool) error {
	query := `UPDATE playback_sessions SET is_playing = ?, last_position_update = ? WHERE id = ?`
	playing := 0
	if isPlaying {
		playing = 1
	}
	_, err := s.db.Exec(query, playing, time.Now().Unix(), id)
	return err
}

// UpdateABSSyncTime updates the last ABS progress sync timestamp.
func (s *PlaybackStore) UpdateABSSyncTime(id string) error {
	query := `UPDATE playback_sessions SET abs_progress_synced_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, time.Now().Unix(), id)
	return err
}

// UpdateSonosUUID updates the Sonos device UUID (for player switching).
func (s *PlaybackStore) UpdateSonosUUID(id string, sonosUUID string) error {
	query := `UPDATE playback_sessions SET sonos_uuid = ?, last_position_update = ? WHERE id = ?`
	_, err := s.db.Exec(query, sonosUUID, time.Now().Unix(), id)
	return err
}

// UpdateStreamToken updates the stream token (for player switching with fresh token).
func (s *PlaybackStore) UpdateStreamToken(id string, streamToken string) error {
	query := `UPDATE playback_sessions SET stream_token = ?, last_position_update = ? WHERE id = ?`
	_, err := s.db.Exec(query, streamToken, time.Now().Unix(), id)
	return err
}

// Delete removes a playback session by ID.
func (s *PlaybackStore) Delete(id string) error {
	query := `DELETE FROM playback_sessions WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// DeleteBySessionID removes all playback sessions for a web session.
func (s *PlaybackStore) DeleteBySessionID(sessionID string) error {
	query := `DELETE FROM playback_sessions WHERE session_id = ?`
	_, err := s.db.Exec(query, sessionID)
	return err
}

// UpdateCurrentSegment updates the current segment index.
func (s *PlaybackStore) UpdateCurrentSegment(id string, segment int) error {
	query := `UPDATE playback_sessions SET current_segment = ?, last_position_update = ? WHERE id = ?`
	_, err := s.db.Exec(query, segment, time.Now().Unix(), id)
	return err
}

// UpdatePositionAndSegment updates both position and segment atomically.
func (s *PlaybackStore) UpdatePositionAndSegment(id string, positionSec int, segment int) error {
	query := `UPDATE playback_sessions SET position_sec = ?, current_segment = ?, last_position_update = ? WHERE id = ?`
	_, err := s.db.Exec(query, positionSec, segment, time.Now().Unix(), id)
	return err
}

// ListActive returns all currently playing sessions.
func (s *PlaybackStore) ListActive() ([]*PlaybackSession, error) {
	query := `
		SELECT id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at
		FROM playback_sessions WHERE is_playing = 1 ORDER BY started_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// ListAll returns all playback sessions.
func (s *PlaybackStore) ListAll() ([]*PlaybackSession, error) {
	query := `
		SELECT id, session_id, item_id, sonos_uuid, stream_token, position_sec, duration_sec, current_segment, segment_duration_sec, is_playing, started_at, last_position_update, abs_progress_synced_at
		FROM playback_sessions ORDER BY started_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// DeleteStale removes playback sessions older than the given duration.
func (s *PlaybackStore) DeleteStale(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge).Unix()
	query := `DELETE FROM playback_sessions WHERE last_position_update < ?`
	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// StopAllPlaying marks all playing sessions as stopped.
// Used on startup to reset stale playback state from previous runs.
func (s *PlaybackStore) StopAllPlaying() (int64, error) {
	query := `UPDATE playback_sessions SET is_playing = 0 WHERE is_playing = 1`
	result, err := s.db.Exec(query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *PlaybackStore) scanRow(row *sql.Row) (*PlaybackSession, error) {
	var ps PlaybackSession
	var isPlaying int
	var currentSegment, segmentDurationSec sql.NullInt64
	var startedAt, lastPositionUpdate, absSyncedAt int64

	err := row.Scan(
		&ps.ID,
		&ps.SessionID,
		&ps.ItemID,
		&ps.SonosUUID,
		&ps.StreamToken,
		&ps.PositionSec,
		&ps.DurationSec,
		&currentSegment,
		&segmentDurationSec,
		&isPlaying,
		&startedAt,
		&lastPositionUpdate,
		&absSyncedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	ps.IsPlaying = isPlaying == 1
	ps.StartedAt = time.Unix(startedAt, 0)
	ps.LastPositionUpdate = time.Unix(lastPositionUpdate, 0)
	ps.ABSProgressSyncedAt = time.Unix(absSyncedAt, 0)
	if currentSegment.Valid {
		ps.CurrentSegment = int(currentSegment.Int64)
	}
	if segmentDurationSec.Valid {
		ps.SegmentDurationSec = int(segmentDurationSec.Int64)
	}

	return &ps, nil
}

func (s *PlaybackStore) scanRows(rows *sql.Rows) ([]*PlaybackSession, error) {
	var sessions []*PlaybackSession
	for rows.Next() {
		var ps PlaybackSession
		var isPlaying int
		var currentSegment, segmentDurationSec sql.NullInt64
		var startedAt, lastPositionUpdate, absSyncedAt int64

		err := rows.Scan(
			&ps.ID,
			&ps.SessionID,
			&ps.ItemID,
			&ps.SonosUUID,
			&ps.StreamToken,
			&ps.PositionSec,
			&ps.DurationSec,
			&currentSegment,
			&segmentDurationSec,
			&isPlaying,
			&startedAt,
			&lastPositionUpdate,
			&absSyncedAt,
		)
		if err != nil {
			return nil, err
		}

		ps.IsPlaying = isPlaying == 1
		ps.StartedAt = time.Unix(startedAt, 0)
		ps.LastPositionUpdate = time.Unix(lastPositionUpdate, 0)
		ps.ABSProgressSyncedAt = time.Unix(absSyncedAt, 0)
		if currentSegment.Valid {
			ps.CurrentSegment = int(currentSegment.Int64)
		}
		if segmentDurationSec.Valid {
			ps.SegmentDurationSec = int(segmentDurationSec.Int64)
		}
		sessions = append(sessions, &ps)
	}

	return sessions, rows.Err()
}
