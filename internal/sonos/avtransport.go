package sonos

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the default timeout for UPnP requests.
	DefaultTimeout = 10 * time.Second
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries = 3
	// RetryBaseDelay is the base delay between retries.
	RetryBaseDelay = 500 * time.Millisecond
)

// AVTransport provides control over Sonos playback via UPnP AVTransport.
type AVTransport struct {
	deviceIP   string
	httpClient *http.Client
	maxRetries int
}

// NewAVTransport creates a new AVTransport client for a Sonos device.
func NewAVTransport(deviceIP string) *AVTransport {
	return &AVTransport{
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
		maxRetries: MaxRetries,
	}
}

// SetAVTransportURI sets the URI to play.
func (t *AVTransport) SetAVTransportURI(ctx context.Context, uri string, metadata string) error {
	action := "SetAVTransportURI"
	body := fmt.Sprintf(`
		<u:SetAVTransportURI xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<CurrentURI>%s</CurrentURI>
			<CurrentURIMetaData>%s</CurrentURIMetaData>
		</u:SetAVTransportURI>`,
		AVTransportNamespace,
		escapeXML(uri),
		escapeXML(metadata),
	)

	_, err := t.sendCommand(ctx, action, body)
	return err
}

// Play starts playback.
func (t *AVTransport) Play(ctx context.Context) error {
	action := "Play"
	body := fmt.Sprintf(`
		<u:Play xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Speed>1</Speed>
		</u:Play>`,
		AVTransportNamespace,
	)

	_, err := t.sendCommand(ctx, action, body)
	return err
}

// Pause pauses playback.
func (t *AVTransport) Pause(ctx context.Context) error {
	action := "Pause"
	body := fmt.Sprintf(`
		<u:Pause xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:Pause>`,
		AVTransportNamespace,
	)

	_, err := t.sendCommand(ctx, action, body)
	return err
}

// Stop stops playback.
func (t *AVTransport) Stop(ctx context.Context) error {
	action := "Stop"
	body := fmt.Sprintf(`
		<u:Stop xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:Stop>`,
		AVTransportNamespace,
	)

	_, err := t.sendCommand(ctx, action, body)
	return err
}

// Seek seeks to a position.
func (t *AVTransport) Seek(ctx context.Context, position time.Duration) error {
	action := "Seek"
	timeStr := formatDuration(position)

	body := fmt.Sprintf(`
		<u:Seek xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Unit>REL_TIME</Unit>
			<Target>%s</Target>
		</u:Seek>`,
		AVTransportNamespace,
		timeStr,
	)

	_, err := t.sendCommand(ctx, action, body)
	return err
}

// GetPositionInfo returns the current playback position.
func (t *AVTransport) GetPositionInfo(ctx context.Context) (*PositionInfo, error) {
	action := "GetPositionInfo"
	body := fmt.Sprintf(`
		<u:GetPositionInfo xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:GetPositionInfo>`,
		AVTransportNamespace,
	)

	resp, err := t.sendCommand(ctx, action, body)
	if err != nil {
		return nil, err
	}

	return parsePositionInfo(resp)
}

// GetTransportInfo returns the current transport state.
func (t *AVTransport) GetTransportInfo(ctx context.Context) (*TransportInfo, error) {
	action := "GetTransportInfo"
	body := fmt.Sprintf(`
		<u:GetTransportInfo xmlns:u="%s">
			<InstanceID>0</InstanceID>
		</u:GetTransportInfo>`,
		AVTransportNamespace,
	)

	resp, err := t.sendCommand(ctx, action, body)
	if err != nil {
		return nil, err
	}

	return parseTransportInfo(resp)
}

// sendCommand sends a SOAP command to the Sonos device with retry logic.
func (t *AVTransport) sendCommand(ctx context.Context, action string, body string) (string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
			<s:Body>%s</s:Body>
		</s:Envelope>`, body)

	url := fmt.Sprintf("http://%s:1400%s", t.deviceIP, AVTransportServicePath)
	slog.Debug("sending Sonos command", "action", action, "device_ip", t.deviceIP, "url", url)

	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s
			delay := RetryBaseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := t.doRequest(ctx, url, action, soapBody)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry on context cancellation or non-retryable errors
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if !isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("SOAP request failed after %d attempts: %w", t.maxRetries+1, lastErr)
}

// doRequest performs a single SOAP request.
func (t *AVTransport) doRequest(ctx context.Context, url, action, soapBody string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, AVTransportNamespace, action))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Sonos SOAP error", "status", resp.StatusCode, "response", string(responseBody))
		return "", fmt.Errorf("SOAP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	slog.Debug("Sonos command successful", "action", action, "status", resp.StatusCode)
	return string(responseBody), nil
}

// isRetryableError checks if an error is transient and should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Connection refused, reset, etc. are retryable
	errStr := err.Error()
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"no route to host",
		"network is unreachable",
		"i/o timeout",
	}
	for _, msg := range retryableMessages {
		if strings.Contains(strings.ToLower(errStr), msg) {
			return true
		}
	}

	return false
}

// parsePositionInfo parses a GetPositionInfo response.
func parsePositionInfo(response string) (*PositionInfo, error) {
	info := &PositionInfo{}

	// Extract values using regex (simpler than full XML parsing)
	info.Track = extractInt(response, "Track")
	info.TrackDuration = extractString(response, "TrackDuration")
	info.TrackMetaData = extractString(response, "TrackMetaData")
	info.TrackURI = extractString(response, "TrackURI")
	info.RelTime = extractString(response, "RelTime")
	info.AbsTime = extractString(response, "AbsTime")
	info.RelCount = extractInt(response, "RelCount")
	info.AbsCount = extractInt(response, "AbsCount")

	return info, nil
}

// parseTransportInfo parses a GetTransportInfo response.
func parseTransportInfo(response string) (*TransportInfo, error) {
	info := &TransportInfo{}

	info.CurrentTransportState = TransportState(extractString(response, "CurrentTransportState"))
	info.CurrentTransportStatus = extractString(response, "CurrentTransportStatus")
	info.CurrentSpeed = extractString(response, "CurrentSpeed")

	return info, nil
}

// extractString extracts a string value from XML.
func extractString(xml string, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>([^<]*)</%s>`, tag, tag))
	matches := re.FindStringSubmatch(xml)
	if len(matches) > 1 {
		return unescapeXML(matches[1])
	}
	return ""
}

// extractInt extracts an integer value from XML.
func extractInt(xml string, tag string) int {
	str := extractString(xml, tag)
	val, _ := strconv.Atoi(str)
	return val
}

// formatDuration formats a duration as H:MM:SS.
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

// ParseDuration parses a duration string in H:MM:SS format.
func ParseDuration(s string) time.Duration {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}

	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])

	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

// escapeXML escapes special characters for XML.
func escapeXML(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// unescapeXML unescapes XML entities.
func unescapeXML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	return s
}

// GenerateDIDLMetadata generates DIDL-Lite metadata for an audio item.
func GenerateDIDLMetadata(title, artist, albumArt string) string {
	return fmt.Sprintf(`<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
		<item id="1" parentID="0" restricted="1">
			<dc:title>%s</dc:title>
			<dc:creator>%s</dc:creator>
			<upnp:class>object.item.audioItem.audioBroadcast</upnp:class>
			<upnp:albumArtURI>%s</upnp:albumArtURI>
		</item>
	</DIDL-Lite>`,
		escapeXML(title),
		escapeXML(artist),
		escapeXML(albumArt),
	)
}

// GetVolume returns the current volume level (0-100).
func (t *AVTransport) GetVolume(ctx context.Context) (int, error) {
	action := "GetVolume"
	body := fmt.Sprintf(`
		<u:GetVolume xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Channel>Master</Channel>
		</u:GetVolume>`,
		RenderingControlNamespace,
	)

	resp, err := t.sendRenderingControlCommand(ctx, action, body)
	if err != nil {
		return 0, err
	}

	return extractInt(resp, "CurrentVolume"), nil
}

// SetVolume sets the volume level (0-100).
func (t *AVTransport) SetVolume(ctx context.Context, level int) error {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}

	action := "SetVolume"
	body := fmt.Sprintf(`
		<u:SetVolume xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Channel>Master</Channel>
			<DesiredVolume>%d</DesiredVolume>
		</u:SetVolume>`,
		RenderingControlNamespace,
		level,
	)

	_, err := t.sendRenderingControlCommand(ctx, action, body)
	return err
}

// GetMute returns the current mute state.
func (t *AVTransport) GetMute(ctx context.Context) (bool, error) {
	action := "GetMute"
	body := fmt.Sprintf(`
		<u:GetMute xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Channel>Master</Channel>
		</u:GetMute>`,
		RenderingControlNamespace,
	)

	resp, err := t.sendRenderingControlCommand(ctx, action, body)
	if err != nil {
		return false, err
	}

	return extractInt(resp, "CurrentMute") == 1, nil
}

// SetMute sets the mute state.
func (t *AVTransport) SetMute(ctx context.Context, mute bool) error {
	action := "SetMute"
	muteVal := 0
	if mute {
		muteVal = 1
	}

	body := fmt.Sprintf(`
		<u:SetMute xmlns:u="%s">
			<InstanceID>0</InstanceID>
			<Channel>Master</Channel>
			<DesiredMute>%d</DesiredMute>
		</u:SetMute>`,
		RenderingControlNamespace,
		muteVal,
	)

	_, err := t.sendRenderingControlCommand(ctx, action, body)
	return err
}

// sendRenderingControlCommand sends a SOAP command to the RenderingControl service.
func (t *AVTransport) sendRenderingControlCommand(ctx context.Context, action string, body string) (string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
			<s:Body>%s</s:Body>
		</s:Envelope>`, body)

	url := fmt.Sprintf("http://%s:1400%s", t.deviceIP, RenderingControlServicePath)
	slog.Debug("sending Sonos RenderingControl command", "action", action, "device_ip", t.deviceIP, "url", url)

	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			delay := RetryBaseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := t.doRenderingControlRequest(ctx, url, action, soapBody)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if !isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("SOAP request failed after %d attempts: %w", t.maxRetries+1, lastErr)
}

// doRenderingControlRequest performs a single SOAP request to the RenderingControl service.
func (t *AVTransport) doRenderingControlRequest(ctx context.Context, url, action, soapBody string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, RenderingControlNamespace, action))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Sonos SOAP error", "status", resp.StatusCode, "response", string(responseBody))
		return "", fmt.Errorf("SOAP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	slog.Debug("Sonos RenderingControl command successful", "action", action, "status", resp.StatusCode)
	return string(responseBody), nil
}
