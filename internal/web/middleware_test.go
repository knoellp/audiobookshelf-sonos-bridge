package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddleware(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a simple handler that returns 200 OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with logging middleware
	logged := LoggingMiddleware(logger)(handler)

	// Make a test request
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	logged.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check log output contains expected fields
	logOutput := buf.String()
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Error("log should contain method")
	}
	if !strings.Contains(logOutput, `"path":"/health"`) {
		t.Error("log should contain path")
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Error("log should contain status")
	}
}

func TestLoggingMiddleware_RedactsStreamTokens(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logged := LoggingMiddleware(logger)(handler)

	// Make request with token in path
	req := httptest.NewRequest("GET", "/stream/abc123secret456/file", nil)
	rec := httptest.NewRecorder()

	logged.ServeHTTP(rec, req)

	// Check that token is redacted
	logOutput := buf.String()
	if strings.Contains(logOutput, "abc123secret456") {
		t.Error("log should not contain actual token")
	}
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Error("log should contain [REDACTED]")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/health", "/health"},
		{"/libraries", "/libraries"},
		{"/stream/secrettoken123/file", "/stream/[REDACTED]/file"},
		{"/stream/abc/file", "/stream/[REDACTED]/file"},
		{"/stream/", "/stream/"},
		{"/api/items", "/api/items"},
	}

	for _, tt := range tests {
		result := sanitizePath(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"username", "john", "john"},
		{"password", "secret123", "[REDACTED]"},
		{"Password", "secret123", "[REDACTED]"},
		{"auth_token", "abc123", "[REDACTED]"},
		{"Authorization", "Bearer xyz", "[REDACTED]"},
		{"session_id", "sess_123", "[REDACTED]"},
		{"api_key", "key_456", "[REDACTED]"},
		{"user_credential", "cred", "[REDACTED]"},
		{"title", "My Book", "My Book"},
		{"author", "John Doe", "John Doe"},
	}

	for _, tt := range tests {
		result := SanitizeForLog(tt.key, tt.value)
		if result != tt.expected {
			t.Errorf("SanitizeForLog(%q, %q) = %q, want %q", tt.key, tt.value, result, tt.expected)
		}
	}
}

func TestResponseWriter_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := wrapResponseWriter(rec)

	wrapped.WriteHeader(http.StatusNotFound)

	if wrapped.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", wrapped.status)
	}
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := wrapResponseWriter(rec)

	// Write without explicit WriteHeader
	wrapped.Write([]byte("test"))

	if wrapped.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", wrapped.status)
	}
}

func TestResponseWriter_OnlyFirstWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := wrapResponseWriter(rec)

	wrapped.WriteHeader(http.StatusCreated)
	wrapped.WriteHeader(http.StatusNotFound) // Should be ignored

	if wrapped.status != http.StatusCreated {
		t.Errorf("expected status 201 (first call), got %d", wrapped.status)
	}
}
