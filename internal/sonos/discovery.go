package sonos

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"audiobookshelf-sonos-bridge/internal/store"
)

// Discovery handles Sonos device discovery via SSDP.
type Discovery struct {
	deviceStore *store.DeviceStore
	httpClient  *http.Client
}

// NewDiscovery creates a new Sonos discovery service.
func NewDiscovery(deviceStore *store.DeviceStore) *Discovery {
	return &Discovery{
		deviceStore: deviceStore,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Discover performs SSDP discovery and updates the device store.
func (d *Discovery) Discover(ctx context.Context, timeout time.Duration) ([]Device, error) {
	// Mark all devices as unreachable before discovery
	if err := d.deviceStore.MarkAllUnreachable(); err != nil {
		slog.Warn("failed to mark devices unreachable", "error", err)
	}

	// Perform SSDP M-SEARCH
	locations, err := d.ssdpSearch(ctx, timeout)
	if err != nil {
		return nil, fmt.Errorf("SSDP search failed: %w", err)
	}

	slog.Debug("SSDP search complete", "locations_found", len(locations))

	// Fetch device descriptions
	var allDevices []Device
	for _, location := range locations {
		device, err := d.fetchDeviceDescription(ctx, location)
		if err != nil {
			slog.Warn("failed to fetch device description", "location", location, "error", err)
			continue
		}
		allDevices = append(allDevices, *device)
	}

	// Get zone topology info (invisible UUIDs and group info) from ZoneGroupTopology
	invisibleUUIDs, groupInfo := d.getZoneInfo(ctx, allDevices)

	// Process all devices - mark some as hidden (stereo pair slaves, non-coordinator group members)
	var devices []Device
	var invisibleFiltered, groupMembersFiltered int

	for _, device := range allDevices {
		// Normalize UUID for comparison (remove "uuid:" prefix)
		normalizedUUID := NormalizeUUID(device.UUID)

		// Determine if this device should be hidden
		isHidden := false
		groupSize := 1

		// Check if invisible (stereo pair slave)
		if invisibleUUIDs[normalizedUUID] {
			slog.Debug("marking invisible device as hidden (stereo pair slave)",
				"name", device.Name,
				"uuid", device.UUID,
				"model", device.Model)
			isHidden = true
			invisibleFiltered++
		}

		// Check group membership
		info, hasInfo := groupInfo[normalizedUUID]
		if hasInfo {
			groupSize = info.GroupSize
			if info.GroupSize > 1 && !info.IsCoordinator {
				// This device is a non-coordinator member of a group - hide it
				slog.Debug("marking non-coordinator group member as hidden",
					"name", device.Name,
					"uuid", device.UUID,
					"group_size", info.GroupSize)
				isHidden = true
				groupMembersFiltered++
			}
		}

		// Upsert to database with hidden flag and group size
		storeDevice := &store.SonosDevice{
			UUID:         device.UUID,
			Name:         device.Name,
			IPAddress:    device.IPAddress,
			LocationURL:  device.LocationURL,
			Model:        device.Model,
			IsReachable:  true,
			IsHidden:     isHidden,
			GroupSize:    groupSize,
			DiscoveredAt: time.Now(),
			LastSeenAt:   time.Now(),
		}

		// Check if this is a new device or update
		existing, _ := d.deviceStore.Get(device.UUID)
		if existing != nil {
			storeDevice.DiscoveredAt = existing.DiscoveredAt
		}

		if err := d.deviceStore.Upsert(storeDevice); err != nil {
			slog.Warn("failed to save device", "uuid", device.UUID, "error", err)
			continue
		}

		// Only add visible devices to the returned list
		if !isHidden {
			device.GroupSize = groupSize
			devices = append(devices, device)
			if device.GroupSize > 1 {
				slog.Info("discovered Sonos device (group coordinator)",
					"name", device.Name, "model", device.Model, "ip", device.IPAddress, "group_size", device.GroupSize)
			} else {
				slog.Info("discovered Sonos device", "name", device.Name, "model", device.Model, "ip", device.IPAddress)
			}
		}
	}

	slog.Info("Sonos discovery complete",
		"total_found", len(allDevices),
		"visible_devices", len(devices),
		"invisible_filtered", invisibleFiltered,
		"group_members_filtered", groupMembersFiltered)

	return devices, nil
}

// getZoneInfo retrieves invisible UUIDs and group info from ZoneGroupTopology.
func (d *Discovery) getZoneInfo(ctx context.Context, devices []Device) (invisibleUUIDs map[string]bool, groupInfo map[string]GroupInfo) {
	invisibleUUIDs = make(map[string]bool)
	groupInfo = make(map[string]GroupInfo)

	if len(devices) == 0 {
		return
	}

	// Use the first discovered device to query ZoneGroupTopology
	// All Sonos devices share the same topology information
	device := devices[0]
	topology := NewZoneGroupTopology(device.IPAddress)

	state, err := topology.GetZoneGroupState(ctx)
	if err != nil {
		slog.Warn("failed to get zone group state, skipping topology filtering",
			"device", device.Name,
			"error", err)
		return
	}

	invisibleUUIDs = state.GetInvisibleUUIDs()
	groupInfo = state.GetGroupInfo()
	return
}

// ssdpSearch performs an SSDP M-SEARCH and returns discovered device locations.
func (d *Discovery) ssdpSearch(ctx context.Context, timeout time.Duration) ([]string, error) {
	// Create UDP socket
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(timeout))

	// Multicast address
	addr, err := net.ResolveUDPAddr("udp4", SSDPMulticastAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Build M-SEARCH request
	searchRequest := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"MX: %d\r\n"+
			"ST: %s\r\n"+
			"\r\n",
		SSDPMulticastAddr,
		int(timeout.Seconds()),
		SSDPSearchTarget,
	)

	// Send M-SEARCH
	_, err = conn.WriteToUDP([]byte(searchRequest), addr)
	if err != nil {
		return nil, fmt.Errorf("failed to send M-SEARCH: %w", err)
	}

	// Collect responses
	locationSet := make(map[string]bool)
	buf := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return d.mapToSlice(locationSet), ctx.Err()
		default:
		}

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			// Other error, continue
			continue
		}

		response := string(buf[:n])
		location := d.extractLocation(response)
		if location != "" && !locationSet[location] {
			locationSet[location] = true
		}
	}

	return d.mapToSlice(locationSet), nil
}

// extractLocation extracts the LOCATION header from an SSDP response.
func (d *Discovery) extractLocation(response string) string {
	re := regexp.MustCompile(`(?i)LOCATION:\s*(.+)\r\n`)
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// fetchDeviceDescription fetches and parses the device description XML.
func (d *Discovery) fetchDeviceDescription(ctx context.Context, location string) (*Device, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var desc DeviceDescription
	if err := xml.Unmarshal(body, &desc); err != nil {
		return nil, fmt.Errorf("failed to parse device description: %w", err)
	}

	// Verify it's a Sonos device
	if !strings.Contains(desc.Device.Manufacturer, "Sonos") {
		return nil, fmt.Errorf("not a Sonos device")
	}

	// Extract IP from location URL
	ip := d.extractIP(location)

	// Use RoomName if available, otherwise fall back to FriendlyName
	name := desc.Device.RoomName
	if name == "" {
		name = desc.Device.FriendlyName
	}

	return &Device{
		UUID:        desc.Device.UDN,
		Name:        name,
		IPAddress:   ip,
		LocationURL: location,
		Model:       desc.Device.ModelName,
		IsReachable: true,
	}, nil
}

// extractIP extracts the IP address from a URL.
func (d *Discovery) extractIP(url string) string {
	re := regexp.MustCompile(`https?://([^:/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (d *Discovery) mapToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// GetDevices returns all known devices from the store.
func (d *Discovery) GetDevices() ([]*store.SonosDevice, error) {
	return d.deviceStore.List()
}

// GetReachableDevices returns only reachable devices.
func (d *Discovery) GetReachableDevices() ([]*store.SonosDevice, error) {
	return d.deviceStore.ListReachable()
}

// GetDevice returns a specific device by UUID.
func (d *Discovery) GetDevice(uuid string) (*store.SonosDevice, error) {
	return d.deviceStore.Get(uuid)
}

// RefreshGroupInfo queries ZoneGroupTopology and updates group sizes in the store.
// This is much faster than a full SSDP discovery (typically <100ms vs 5 seconds).
func (d *Discovery) RefreshGroupInfo(ctx context.Context) error {
	slog.Debug("RefreshGroupInfo: starting group info refresh")

	devices, err := d.deviceStore.List()
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	if len(devices) == 0 {
		slog.Debug("RefreshGroupInfo: no devices in store")
		return nil
	}

	// Query ZoneGroupTopology from any reachable device
	var zoneState *ZoneGroupState
	for _, device := range devices {
		if !device.IsReachable {
			continue
		}
		topology := NewZoneGroupTopology(device.IPAddress)
		state, err := topology.GetZoneGroupState(ctx)
		if err == nil {
			zoneState = state
			slog.Debug("RefreshGroupInfo: got zone state from device", "device_ip", device.IPAddress)
			break
		}
	}

	if zoneState == nil {
		return fmt.Errorf("could not query zone group state from any device")
	}

	// Get updated group info
	groupInfo := zoneState.GetGroupInfo()
	invisibleUUIDs := zoneState.GetInvisibleUUIDs()

	slog.Debug("RefreshGroupInfo: group info retrieved",
		"group_info_count", len(groupInfo),
		"invisible_count", len(invisibleUUIDs))

	// Update each device in the store with current group info
	for _, device := range devices {
		normalizedUUID := NormalizeUUID(device.UUID)

		// Check if invisible (stereo pair slave)
		isHidden := invisibleUUIDs[normalizedUUID]
		groupSize := 1

		if info, hasInfo := groupInfo[normalizedUUID]; hasInfo {
			groupSize = info.GroupSize
			// Hide non-coordinator group members
			if info.GroupSize > 1 && !info.IsCoordinator {
				isHidden = true
			}
			slog.Debug("RefreshGroupInfo: found group info for device",
				"device_name", device.Name,
				"normalized_uuid", normalizedUUID,
				"new_group_size", groupSize,
				"old_group_size", device.GroupSize,
				"is_coordinator", info.IsCoordinator)
		} else {
			slog.Debug("RefreshGroupInfo: no group info for device",
				"device_name", device.Name,
				"normalized_uuid", normalizedUUID)
		}

		// Only update if something changed
		if device.GroupSize != groupSize || device.IsHidden != isHidden {
			slog.Info("RefreshGroupInfo: updating device",
				"device_name", device.Name,
				"old_group_size", device.GroupSize,
				"new_group_size", groupSize,
				"old_hidden", device.IsHidden,
				"new_hidden", isHidden)
			device.GroupSize = groupSize
			device.IsHidden = isHidden
			if err := d.deviceStore.Upsert(device); err != nil {
				slog.Warn("failed to update device group info", "uuid", device.UUID, "error", err)
			}
		}
	}

	slog.Debug("RefreshGroupInfo: completed")
	return nil
}
