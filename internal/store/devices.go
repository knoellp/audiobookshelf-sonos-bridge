package store

import (
	"database/sql"
	"errors"
	"time"
)

// SonosDevice represents a Sonos device in the database.
type SonosDevice struct {
	UUID         string
	Name         string
	IPAddress    string
	LocationURL  string
	Model        string
	IsReachable  bool
	DiscoveredAt time.Time
	LastSeenAt   time.Time
}

// DeviceStore provides CRUD operations for Sonos devices.
type DeviceStore struct {
	db *sql.DB
}

// NewDeviceStore creates a new device store.
func NewDeviceStore(db *DB) *DeviceStore {
	return &DeviceStore{db: db.Conn()}
}

// Upsert inserts or updates a Sonos device.
func (s *DeviceStore) Upsert(device *SonosDevice) error {
	query := `
		INSERT INTO sonos_devices (uuid, name, ip_address, location_url, model, is_reachable, discovered_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(uuid) DO UPDATE SET
			name = excluded.name,
			ip_address = excluded.ip_address,
			location_url = excluded.location_url,
			model = excluded.model,
			is_reachable = excluded.is_reachable,
			last_seen_at = excluded.last_seen_at
	`
	isReachable := 0
	if device.IsReachable {
		isReachable = 1
	}

	_, err := s.db.Exec(query,
		device.UUID,
		device.Name,
		device.IPAddress,
		device.LocationURL,
		device.Model,
		isReachable,
		device.DiscoveredAt.Unix(),
		device.LastSeenAt.Unix(),
	)
	return err
}

// Get retrieves a device by UUID.
func (s *DeviceStore) Get(uuid string) (*SonosDevice, error) {
	query := `
		SELECT uuid, name, ip_address, location_url, model, is_reachable, discovered_at, last_seen_at
		FROM sonos_devices WHERE uuid = ?
	`
	row := s.db.QueryRow(query, uuid)

	var device SonosDevice
	var isReachable int
	var discoveredAt, lastSeenAt int64

	err := row.Scan(
		&device.UUID,
		&device.Name,
		&device.IPAddress,
		&device.LocationURL,
		&device.Model,
		&isReachable,
		&discoveredAt,
		&lastSeenAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	device.IsReachable = isReachable == 1
	device.DiscoveredAt = time.Unix(discoveredAt, 0)
	device.LastSeenAt = time.Unix(lastSeenAt, 0)

	return &device, nil
}

// List returns all Sonos devices.
func (s *DeviceStore) List() ([]*SonosDevice, error) {
	query := `
		SELECT uuid, name, ip_address, location_url, model, is_reachable, discovered_at, last_seen_at
		FROM sonos_devices ORDER BY name
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*SonosDevice
	for rows.Next() {
		var device SonosDevice
		var isReachable int
		var discoveredAt, lastSeenAt int64

		err := rows.Scan(
			&device.UUID,
			&device.Name,
			&device.IPAddress,
			&device.LocationURL,
			&device.Model,
			&isReachable,
			&discoveredAt,
			&lastSeenAt,
		)
		if err != nil {
			return nil, err
		}

		device.IsReachable = isReachable == 1
		device.DiscoveredAt = time.Unix(discoveredAt, 0)
		device.LastSeenAt = time.Unix(lastSeenAt, 0)
		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

// ListReachable returns only reachable Sonos devices.
func (s *DeviceStore) ListReachable() ([]*SonosDevice, error) {
	query := `
		SELECT uuid, name, ip_address, location_url, model, is_reachable, discovered_at, last_seen_at
		FROM sonos_devices WHERE is_reachable = 1 ORDER BY name
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*SonosDevice
	for rows.Next() {
		var device SonosDevice
		var isReachable int
		var discoveredAt, lastSeenAt int64

		err := rows.Scan(
			&device.UUID,
			&device.Name,
			&device.IPAddress,
			&device.LocationURL,
			&device.Model,
			&isReachable,
			&discoveredAt,
			&lastSeenAt,
		)
		if err != nil {
			return nil, err
		}

		device.IsReachable = isReachable == 1
		device.DiscoveredAt = time.Unix(discoveredAt, 0)
		device.LastSeenAt = time.Unix(lastSeenAt, 0)
		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

// SetReachable updates the reachability status of a device.
func (s *DeviceStore) SetReachable(uuid string, reachable bool) error {
	query := `UPDATE sonos_devices SET is_reachable = ?, last_seen_at = ? WHERE uuid = ?`
	isReachable := 0
	if reachable {
		isReachable = 1
	}
	_, err := s.db.Exec(query, isReachable, time.Now().Unix(), uuid)
	return err
}

// Delete removes a device by UUID.
func (s *DeviceStore) Delete(uuid string) error {
	query := `DELETE FROM sonos_devices WHERE uuid = ?`
	_, err := s.db.Exec(query, uuid)
	return err
}

// MarkAllUnreachable marks all devices as unreachable (used before discovery scan).
func (s *DeviceStore) MarkAllUnreachable() error {
	query := `UPDATE sonos_devices SET is_reachable = 0`
	_, err := s.db.Exec(query)
	return err
}
