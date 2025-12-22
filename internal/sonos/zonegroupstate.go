package sonos

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// ZoneGroupTopology provides access to Sonos zone group topology information.
type ZoneGroupTopology struct {
	deviceIP   string
	httpClient *http.Client
}

// NewZoneGroupTopology creates a new ZoneGroupTopology client for a Sonos device.
func NewZoneGroupTopology(deviceIP string) *ZoneGroupTopology {
	return &ZoneGroupTopology{
		deviceIP: deviceIP,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 5 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
}

// GetZoneGroupState retrieves the current zone group topology from the device.
func (z *ZoneGroupTopology) GetZoneGroupState(ctx context.Context) (*ZoneGroupState, error) {
	action := "GetZoneGroupState"
	body := fmt.Sprintf(`
		<u:GetZoneGroupState xmlns:u="%s">
		</u:GetZoneGroupState>`,
		ZoneGroupTopologyNamespace,
	)

	resp, err := z.sendCommand(ctx, action, body)
	if err != nil {
		return nil, fmt.Errorf("GetZoneGroupState failed: %w", err)
	}

	return parseZoneGroupState(resp)
}

// sendCommand sends a SOAP command to the ZoneGroupTopology service.
func (z *ZoneGroupTopology) sendCommand(ctx context.Context, action string, body string) (string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
			<s:Body>%s</s:Body>
		</s:Envelope>`, body)

	url := fmt.Sprintf("http://%s:1400%s", z.deviceIP, ZoneGroupTopologyServicePath)
	slog.Debug("sending ZoneGroupTopology command", "action", action, "device_ip", z.deviceIP)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, ZoneGroupTopologyNamespace, action))

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("ZoneGroupTopology SOAP error", "status", resp.StatusCode, "response", string(responseBody))
		return "", fmt.Errorf("SOAP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	return string(responseBody), nil
}

// parseZoneGroupState parses the GetZoneGroupState response.
func parseZoneGroupState(response string) (*ZoneGroupState, error) {
	// Extract the ZoneGroupState content from the SOAP response
	// The content is XML-escaped inside the response
	zoneGroupStateContent := extractString(response, "ZoneGroupState")
	if zoneGroupStateContent == "" {
		return nil, fmt.Errorf("ZoneGroupState not found in response")
	}

	// The content is HTML-escaped, need to unescape it
	unescaped := unescapeXML(zoneGroupStateContent)

	// Parse the ZoneGroupState XML (structure: <ZoneGroupState><ZoneGroups>...</ZoneGroups></ZoneGroupState>)
	var wrapper ZoneGroupStateWrapperXML
	if err := xml.Unmarshal([]byte(unescaped), &wrapper); err != nil {
		slog.Debug("ZoneGroupState raw content", "content", unescaped[:min(500, len(unescaped))])
		return nil, fmt.Errorf("failed to parse ZoneGroupState: %w", err)
	}

	// Convert to our domain types
	state := &ZoneGroupState{
		ZoneGroups: make([]ZoneGroup, 0, len(wrapper.ZoneGroups)),
	}

	for _, zg := range wrapper.ZoneGroups {
		group := ZoneGroup{
			Coordinator: zg.Coordinator,
			Members:     make([]ZoneGroupMember, 0, len(zg.Members)),
		}

		for _, m := range zg.Members {
			member := ZoneGroupMember{
				UUID:      m.UUID,
				ZoneName:  m.ZoneName,
				Invisible: m.Invisible == "1",
			}
			group.Members = append(group.Members, member)
		}

		state.ZoneGroups = append(state.ZoneGroups, group)
	}

	slog.Debug("parsed zone group state",
		"groups", len(state.ZoneGroups),
		"total_members", countTotalMembers(state))

	// Log details about each group to identify grouped players
	for i, group := range state.ZoneGroups {
		visibleCount := 0
		var memberNames []string
		for _, m := range group.Members {
			if !m.Invisible {
				visibleCount++
				memberNames = append(memberNames, m.ZoneName)
			}
		}
		if visibleCount > 1 {
			slog.Info("found grouped players",
				"group_index", i,
				"coordinator", group.Coordinator,
				"visible_members", visibleCount,
				"member_names", memberNames)
		}
	}

	return state, nil
}

// GetInvisibleUUIDs returns a set of UUIDs for invisible players (stereo pair slaves).
func (s *ZoneGroupState) GetInvisibleUUIDs() map[string]bool {
	invisible := make(map[string]bool)
	for _, group := range s.ZoneGroups {
		for _, member := range group.Members {
			if member.Invisible {
				invisible[member.UUID] = true
				slog.Debug("found invisible player",
					"uuid", member.UUID,
					"zone_name", member.ZoneName)
			}
		}
	}
	return invisible
}

// GetVisibleMembers returns only the visible members from all groups.
func (s *ZoneGroupState) GetVisibleMembers() []ZoneGroupMember {
	var visible []ZoneGroupMember
	for _, group := range s.ZoneGroups {
		for _, member := range group.Members {
			if !member.Invisible {
				visible = append(visible, member)
			}
		}
	}
	return visible
}

// GetGroupInfo returns a map of device UUID to GroupInfo containing group size and coordinator status.
// Only coordinators of groups with multiple visible members have GroupSize > 1.
// Non-coordinator members of groups are marked as grouped (should be hidden from UI).
type GroupInfo struct {
	GroupSize     int  // Number of visible members in this group
	IsCoordinator bool // True if this device is the group coordinator
}

func (s *ZoneGroupState) GetGroupInfo() map[string]GroupInfo {
	info := make(map[string]GroupInfo)

	for _, group := range s.ZoneGroups {
		// Count visible members in this group
		var visibleMembers []ZoneGroupMember
		for _, member := range group.Members {
			if !member.Invisible {
				visibleMembers = append(visibleMembers, member)
			}
		}

		groupSize := len(visibleMembers)

		// Set info for each visible member
		for _, member := range visibleMembers {
			isCoordinator := member.UUID == group.Coordinator
			info[member.UUID] = GroupInfo{
				GroupSize:     groupSize,
				IsCoordinator: isCoordinator,
			}
		}
	}

	return info
}

// countTotalMembers counts all members in all groups.
func countTotalMembers(state *ZoneGroupState) int {
	count := 0
	for _, group := range state.ZoneGroups {
		count += len(group.Members)
	}
	return count
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NormalizeUUID normalizes a Sonos UUID by stripping the "uuid:" prefix if present.
func NormalizeUUID(uuid string) string {
	return strings.TrimPrefix(uuid, "uuid:")
}
