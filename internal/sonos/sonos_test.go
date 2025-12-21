package sonos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"audiobookshelf-sonos-bridge/internal/store"
)

func setupTestDB(t *testing.T) (*store.DB, func()) {
	t.Helper()

	f, err := os.CreateTemp("", "sonos_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := f.Name()
	f.Close()

	db, err := store.New(dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"0:00:00", 0},
		{"0:01:00", time.Minute},
		{"0:00:30", 30 * time.Second},
		{"1:00:00", time.Hour},
		{"1:30:45", time.Hour + 30*time.Minute + 45*time.Second},
		{"2:15:30", 2*time.Hour + 15*time.Minute + 30*time.Second},
	}

	for _, tt := range tests {
		result := ParseDuration(tt.input)
		if result != tt.expected {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0:00:00"},
		{time.Minute, "0:01:00"},
		{30 * time.Second, "0:00:30"},
		{time.Hour, "1:00:00"},
		{time.Hour + 30*time.Minute + 45*time.Second, "1:30:45"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateDIDLMetadata(t *testing.T) {
	metadata := GenerateDIDLMetadata("Test Title", "Test Artist", "http://example.com/cover.jpg")

	if !containsString(metadata, "Test Title") {
		t.Error("metadata should contain title")
	}
	if !containsString(metadata, "Test Artist") {
		t.Error("metadata should contain artist")
	}
	if !containsString(metadata, "http://example.com/cover.jpg") {
		t.Error("metadata should contain album art URL")
	}
	if !containsString(metadata, "DIDL-Lite") {
		t.Error("metadata should be DIDL-Lite format")
	}
}

func TestGenerateDIDLMetadata_EscapesXML(t *testing.T) {
	metadata := GenerateDIDLMetadata("Test & Title <Special>", "Artist's \"Name\"", "http://example.com/cover.jpg")

	if containsString(metadata, "&") && !containsString(metadata, "&amp;") {
		t.Error("metadata should escape ampersand")
	}
	if containsString(metadata, "<Special>") {
		t.Error("metadata should escape special characters")
	}
}

func TestExtractString(t *testing.T) {
	xml := `<Response><Track>1</Track><RelTime>0:05:30</RelTime></Response>`

	track := extractString(xml, "Track")
	if track != "1" {
		t.Errorf("expected Track=1, got %q", track)
	}

	relTime := extractString(xml, "RelTime")
	if relTime != "0:05:30" {
		t.Errorf("expected RelTime=0:05:30, got %q", relTime)
	}

	missing := extractString(xml, "Missing")
	if missing != "" {
		t.Errorf("expected empty string for missing tag, got %q", missing)
	}
}

func TestExtractInt(t *testing.T) {
	xml := `<Response><Track>5</Track><Count>100</Count></Response>`

	track := extractInt(xml, "Track")
	if track != 5 {
		t.Errorf("expected Track=5, got %d", track)
	}

	count := extractInt(xml, "Count")
	if count != 100 {
		t.Errorf("expected Count=100, got %d", count)
	}
}

func TestAVTransport_MockServer(t *testing.T) {
	// Create a mock Sonos SOAP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check content type
		if r.Header.Get("Content-Type") != "text/xml; charset=utf-8" {
			t.Error("expected XML content type")
		}

		// Check SOAPAction header
		soapAction := r.Header.Get("SOAPAction")
		if soapAction == "" {
			t.Error("expected SOAPAction header")
		}

		// Return appropriate response based on action
		if containsString(soapAction, "GetPositionInfo") {
			w.Header().Set("Content-Type", "text/xml")
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
					<s:Body>
						<u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
							<Track>1</Track>
							<TrackDuration>1:30:00</TrackDuration>
							<RelTime>0:15:30</RelTime>
							<AbsTime>NOT_IMPLEMENTED</AbsTime>
							<RelCount>2147483647</RelCount>
							<AbsCount>2147483647</AbsCount>
						</u:GetPositionInfoResponse>
					</s:Body>
				</s:Envelope>`))
			return
		}

		if containsString(soapAction, "GetTransportInfo") {
			w.Header().Set("Content-Type", "text/xml")
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
					<s:Body>
						<u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
							<CurrentTransportState>PLAYING</CurrentTransportState>
							<CurrentTransportStatus>OK</CurrentTransportStatus>
							<CurrentSpeed>1</CurrentSpeed>
						</u:GetTransportInfoResponse>
					</s:Body>
				</s:Envelope>`))
			return
		}

		// Default success response for control commands
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
			<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
				<s:Body>
					<u:Response xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/>
				</s:Body>
			</s:Envelope>`))
	}))
	defer server.Close()

	// Extract host from server URL (remove http://)
	host := server.URL[7:]

	// Create AVTransport with mock server
	// Note: We need to modify the port in the actual implementation
	// For testing, we'll use the full URL directly
	transport := &AVTransport{
		deviceIP:   host,
		httpClient: server.Client(),
	}

	// Override the URL building in tests by using a custom httpClient
	transport.httpClient = &http.Client{
		Transport: &mockTransport{
			server: server,
		},
	}

	ctx := context.Background()

	// Test GetPositionInfo
	posInfo, err := transport.GetPositionInfo(ctx)
	if err != nil {
		t.Fatalf("GetPositionInfo failed: %v", err)
	}

	if posInfo.Track != 1 {
		t.Errorf("expected Track=1, got %d", posInfo.Track)
	}
	if posInfo.TrackDuration != "1:30:00" {
		t.Errorf("expected TrackDuration=1:30:00, got %s", posInfo.TrackDuration)
	}
	if posInfo.RelTime != "0:15:30" {
		t.Errorf("expected RelTime=0:15:30, got %s", posInfo.RelTime)
	}

	// Test GetTransportInfo
	transInfo, err := transport.GetTransportInfo(ctx)
	if err != nil {
		t.Fatalf("GetTransportInfo failed: %v", err)
	}

	if transInfo.CurrentTransportState != TransportStatePlaying {
		t.Errorf("expected state PLAYING, got %s", transInfo.CurrentTransportState)
	}
}

// mockTransport redirects all requests to the test server
type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	req.URL.Scheme = "http"
	req.URL.Host = m.server.URL[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(req)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDiscovery_ExtractIP(t *testing.T) {
	d := &Discovery{}

	tests := []struct {
		url      string
		expected string
	}{
		{"http://192.168.1.100:1400/xml/device_description.xml", "192.168.1.100"},
		{"http://10.0.0.5:1400/xml/device_description.xml", "10.0.0.5"},
		{"https://example.com/path", "example.com"},
	}

	for _, tt := range tests {
		result := d.extractIP(tt.url)
		if result != tt.expected {
			t.Errorf("extractIP(%q) = %q, want %q", tt.url, result, tt.expected)
		}
	}
}

func TestDiscovery_ExtractLocation(t *testing.T) {
	d := &Discovery{}

	// SSDP uses HTTP/1.1 style responses with CRLF line endings
	response := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=1800\r\n" +
		"EXT:\r\n" +
		"LOCATION: http://192.168.1.100:1400/xml/device_description.xml\r\n" +
		"SERVER: Linux UPnP/1.0 Sonos/70.3-42010\r\n" +
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
		"USN: uuid:RINCON_123456789::urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
		"\r\n"

	location := d.extractLocation(response)
	expected := "http://192.168.1.100:1400/xml/device_description.xml"

	if location != expected {
		t.Errorf("extractLocation() = %q, want %q", location, expected)
	}
}

func TestDiscovery_ExtractLocation_Empty(t *testing.T) {
	d := &Discovery{}

	response := `HTTP/1.1 200 OK
CACHE-CONTROL: max-age=1800

`

	location := d.extractLocation(response)
	if location != "" {
		t.Errorf("extractLocation() = %q, want empty string", location)
	}
}
