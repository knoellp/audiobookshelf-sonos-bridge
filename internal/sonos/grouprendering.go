package sonos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// GroupRenderingControl provides access to Sonos GroupRenderingControl service.
// This service is used to control volume for all members of a group proportionally.
type GroupRenderingControl struct {
	ip         string
	httpClient *http.Client
}

// GroupRenderingControl service constants.
const (
	GroupRenderingControlServicePath = "/MediaRenderer/GroupRenderingControl/Control"
	GroupRenderingControlNamespace   = "urn:schemas-upnp-org:service:GroupRenderingControl:1"
)

// NewGroupRenderingControl creates a new GroupRenderingControl client.
func NewGroupRenderingControl(ip string) *GroupRenderingControl {
	return &GroupRenderingControl{
		ip: ip,
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

// GetGroupVolume returns the current group volume (0-100).
// This should be called on the coordinator of the group.
func (g *GroupRenderingControl) GetGroupVolume(ctx context.Context) (int, error) {
	action := "GetGroupVolume"
	body := fmt.Sprintf(`
		<u:GetGroupVolume xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:GetGroupVolume>`,
		GroupRenderingControlNamespace,
	)

	resp, err := g.sendCommand(ctx, action, body)
	if err != nil {
		return 0, fmt.Errorf("GetGroupVolume failed: %w", err)
	}

	// Parse response to extract CurrentVolume
	re := regexp.MustCompile(`<CurrentVolume>(\d+)</CurrentVolume>`)
	matches := re.FindStringSubmatch(resp)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse volume from response")
	}

	volume, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid volume value: %w", err)
	}

	slog.Debug("got group volume", "ip", g.ip, "volume", volume)
	return volume, nil
}

// SetGroupVolume sets the group volume (0-100).
// This adjusts all group members proportionally.
// This should be called on the coordinator of the group.
func (g *GroupRenderingControl) SetGroupVolume(ctx context.Context, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}

	action := "SetGroupVolume"
	body := fmt.Sprintf(`
		<u:SetGroupVolume xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<DesiredVolume>%d</DesiredVolume>
		</u:SetGroupVolume>`,
		GroupRenderingControlNamespace,
		volume,
	)

	_, err := g.sendCommand(ctx, action, body)
	if err != nil {
		return fmt.Errorf("SetGroupVolume failed: %w", err)
	}

	slog.Debug("set group volume", "ip", g.ip, "volume", volume)
	return nil
}

// GetGroupMute returns the current group mute state.
// This should be called on the coordinator of the group.
func (g *GroupRenderingControl) GetGroupMute(ctx context.Context) (bool, error) {
	action := "GetGroupMute"
	body := fmt.Sprintf(`
		<u:GetGroupMute xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:GetGroupMute>`,
		GroupRenderingControlNamespace,
	)

	resp, err := g.sendCommand(ctx, action, body)
	if err != nil {
		return false, fmt.Errorf("GetGroupMute failed: %w", err)
	}

	// Parse response to extract CurrentMute (0 or 1)
	re := regexp.MustCompile(`<CurrentMute>(\d+)</CurrentMute>`)
	matches := re.FindStringSubmatch(resp)
	if len(matches) < 2 {
		return false, fmt.Errorf("could not parse mute from response")
	}

	return matches[1] == "1", nil
}

// SetGroupMute sets the group mute state.
// This mutes/unmutes all group members.
// This should be called on the coordinator of the group.
func (g *GroupRenderingControl) SetGroupMute(ctx context.Context, mute bool) error {
	muteValue := 0
	if mute {
		muteValue = 1
	}

	action := "SetGroupMute"
	body := fmt.Sprintf(`
		<u:SetGroupMute xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<DesiredMute>%d</DesiredMute>
		</u:SetGroupMute>`,
		GroupRenderingControlNamespace,
		muteValue,
	)

	_, err := g.sendCommand(ctx, action, body)
	if err != nil {
		return fmt.Errorf("SetGroupMute failed: %w", err)
	}

	slog.Debug("set group mute", "ip", g.ip, "mute", mute)
	return nil
}

// sendCommand sends a SOAP command to the GroupRenderingControl service.
func (g *GroupRenderingControl) sendCommand(ctx context.Context, action string, body string) (string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
			<s:Body>%s</s:Body>
		</s:Envelope>`, body)

	url := fmt.Sprintf("http://%s:1400%s", g.ip, GroupRenderingControlServicePath)
	slog.Debug("sending GroupRenderingControl command", "action", action, "ip", g.ip)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, GroupRenderingControlNamespace, action))

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("GroupRenderingControl SOAP error",
			"status", resp.StatusCode,
			"action", action,
			"response", string(responseBody))
		return "", fmt.Errorf("SOAP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	return string(responseBody), nil
}
