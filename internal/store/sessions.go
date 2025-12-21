package store

import (
	"database/sql"
	"errors"
	"time"
)

// Session represents a user session in the database.
type Session struct {
	ID          string
	ABSTokenEnc []byte // AES-256-GCM encrypted Audiobookshelf token
	ABSToken    string // Decrypted token (populated by auth handler)
	ABSUserID   string
	UserID      string // Alias for ABSUserID
	ABSUsername string
	CreatedAt   time.Time
	LastUsedAt  time.Time
}

// SessionStore provides CRUD operations for sessions.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a new session store.
func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{db: db.Conn()}
}

// Create inserts a new session.
func (s *SessionStore) Create(session *Session) error {
	query := `
		INSERT INTO sessions (id, abs_token_enc, abs_user_id, abs_username, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		session.ID,
		session.ABSTokenEnc,
		session.ABSUserID,
		session.ABSUsername,
		session.CreatedAt.Unix(),
		session.LastUsedAt.Unix(),
	)
	return err
}

// Get retrieves a session by ID.
func (s *SessionStore) Get(id string) (*Session, error) {
	query := `
		SELECT id, abs_token_enc, abs_user_id, abs_username, created_at, last_used_at
		FROM sessions WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	var session Session
	var createdAt, lastUsedAt int64

	err := row.Scan(
		&session.ID,
		&session.ABSTokenEnc,
		&session.ABSUserID,
		&session.ABSUsername,
		&createdAt,
		&lastUsedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	session.UserID = session.ABSUserID // Alias for consistency with List/ListActive
	session.CreatedAt = time.Unix(createdAt, 0)
	session.LastUsedAt = time.Unix(lastUsedAt, 0)

	return &session, nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (s *SessionStore) UpdateLastUsed(id string) error {
	query := `UPDATE sessions SET last_used_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, time.Now().Unix(), id)
	return err
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(id string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// DeleteOlderThan removes sessions not used since the given time.
func (s *SessionStore) DeleteOlderThan(since time.Time) (int64, error) {
	query := `DELETE FROM sessions WHERE last_used_at < ?`
	result, err := s.db.Exec(query, since.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// List returns all sessions.
func (s *SessionStore) List() ([]*Session, error) {
	query := `
		SELECT id, abs_token_enc, abs_user_id, abs_username, created_at, last_used_at
		FROM sessions ORDER BY last_used_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var createdAt, lastUsedAt int64

		err := rows.Scan(
			&session.ID,
			&session.ABSTokenEnc,
			&session.ABSUserID,
			&session.ABSUsername,
			&createdAt,
			&lastUsedAt,
		)
		if err != nil {
			return nil, err
		}

		session.UserID = session.ABSUserID
		session.CreatedAt = time.Unix(createdAt, 0)
		session.LastUsedAt = time.Unix(lastUsedAt, 0)
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// ListActive returns sessions that have been used recently (within 24 hours).
func (s *SessionStore) ListActive() ([]*Session, error) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	query := `
		SELECT id, abs_token_enc, abs_user_id, abs_username, created_at, last_used_at
		FROM sessions WHERE last_used_at > ? ORDER BY last_used_at DESC
	`
	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var createdAt, lastUsedAt int64

		err := rows.Scan(
			&session.ID,
			&session.ABSTokenEnc,
			&session.ABSUserID,
			&session.ABSUsername,
			&createdAt,
			&lastUsedAt,
		)
		if err != nil {
			return nil, err
		}

		session.UserID = session.ABSUserID
		session.CreatedAt = time.Unix(createdAt, 0)
		session.LastUsedAt = time.Unix(lastUsedAt, 0)
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}
