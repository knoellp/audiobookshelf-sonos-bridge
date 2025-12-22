package store

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection and provides store access.
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and runs migrations.
func New(dbPath string) (*DB, error) {
	// Open database with WAL mode for better concurrency
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := &DB{conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database initialized", "path", dbPath)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying database connection for direct queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// migrate runs all database migrations.
func (db *DB) migrate() error {
	migrations := []string{
		migrationSessions,
		migrationSonosDevices,
		migrationCacheIndex,
		migrationPlaybackSessions,
	}

	for i, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	// Run incremental migrations
	if err := db.runIncrementalMigrations(); err != nil {
		return err
	}

	return nil
}

// runIncrementalMigrations runs ALTER TABLE migrations that modify existing tables.
func (db *DB) runIncrementalMigrations() error {
	// Add cache_format column to cache_index if not exists
	// SQLite doesn't support IF NOT EXISTS for columns, so we check first
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cache_index') WHERE name = 'cache_format'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check cache_format column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating cache_index: adding cache_format column")
		_, err := db.conn.Exec(`ALTER TABLE cache_index ADD COLUMN cache_format TEXT DEFAULT 'mp3'`)
		if err != nil {
			return fmt.Errorf("failed to add cache_format column: %w", err)
		}
	}

	// Add segment_count column to cache_index if not exists
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cache_index') WHERE name = 'segment_count'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check segment_count column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating cache_index: adding segment_count column")
		_, err := db.conn.Exec(`ALTER TABLE cache_index ADD COLUMN segment_count INTEGER DEFAULT 1`)
		if err != nil {
			return fmt.Errorf("failed to add segment_count column: %w", err)
		}
	}

	// Add segment_duration_sec column to cache_index if not exists
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cache_index') WHERE name = 'segment_duration_sec'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check segment_duration_sec column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating cache_index: adding segment_duration_sec column")
		_, err := db.conn.Exec(`ALTER TABLE cache_index ADD COLUMN segment_duration_sec INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("failed to add segment_duration_sec column: %w", err)
		}
	}

	// Add current_segment column to playback_sessions if not exists
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('playback_sessions') WHERE name = 'current_segment'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check current_segment column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating playback_sessions: adding current_segment column")
		_, err := db.conn.Exec(`ALTER TABLE playback_sessions ADD COLUMN current_segment INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("failed to add current_segment column: %w", err)
		}
	}

	// Add segment_duration_sec column to playback_sessions if not exists
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('playback_sessions') WHERE name = 'segment_duration_sec'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check playback segment_duration_sec column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating playback_sessions: adding segment_duration_sec column")
		_, err := db.conn.Exec(`ALTER TABLE playback_sessions ADD COLUMN segment_duration_sec INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("failed to add playback segment_duration_sec column: %w", err)
		}
	}

	// Add is_hidden column to sonos_devices if not exists
	// Used to hide stereo pair slaves and non-coordinator group members without deleting them
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('sonos_devices') WHERE name = 'is_hidden'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check is_hidden column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating sonos_devices: adding is_hidden column")
		_, err := db.conn.Exec(`ALTER TABLE sonos_devices ADD COLUMN is_hidden INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("failed to add is_hidden column: %w", err)
		}
	}

	// Add group_size column to sonos_devices if not exists
	// Stores the number of players in this device's group (1 = standalone, >1 = group coordinator)
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('sonos_devices') WHERE name = 'group_size'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check group_size column: %w", err)
	}

	if count == 0 {
		slog.Info("migrating sonos_devices: adding group_size column")
		_, err := db.conn.Exec(`ALTER TABLE sonos_devices ADD COLUMN group_size INTEGER DEFAULT 1`)
		if err != nil {
			return fmt.Errorf("failed to add group_size column: %w", err)
		}
	}

	return nil
}

// Sessions table schema
const migrationSessions = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    abs_token_enc BLOB NOT NULL,
    abs_user_id TEXT NOT NULL,
    abs_username TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_last_used ON sessions(last_used_at);
`

// Sonos devices table schema
const migrationSonosDevices = `
CREATE TABLE IF NOT EXISTS sonos_devices (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    location_url TEXT NOT NULL,
    model TEXT,
    is_reachable INTEGER NOT NULL DEFAULT 1,
    discovered_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sonos_devices_reachable ON sonos_devices(is_reachable);
`

// Cache index table schema
const migrationCacheIndex = `
CREATE TABLE IF NOT EXISTS cache_index (
    item_id TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    source_size INTEGER NOT NULL,
    source_mtime INTEGER NOT NULL,
    profile_version TEXT NOT NULL,
    cache_path TEXT NOT NULL,
    duration_sec INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    error_text TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cache_status ON cache_index(status);
CREATE INDEX IF NOT EXISTS idx_cache_profile ON cache_index(profile_version);
`

// Playback sessions table schema
const migrationPlaybackSessions = `
CREATE TABLE IF NOT EXISTS playback_sessions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    item_id TEXT NOT NULL,
    sonos_uuid TEXT NOT NULL REFERENCES sonos_devices(uuid),
    stream_token TEXT NOT NULL,
    position_sec INTEGER NOT NULL DEFAULT 0,
    duration_sec INTEGER NOT NULL,
    is_playing INTEGER NOT NULL DEFAULT 0,
    started_at INTEGER NOT NULL,
    last_position_update INTEGER NOT NULL,
    abs_progress_synced_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_playback_session ON playback_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_playback_playing ON playback_sessions(is_playing);
`
