package web

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// LoggingMiddleware creates middleware that logs HTTP requests.
// Sensitive information (tokens, passwords, session IDs) is redacted.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := wrapResponseWriter(w)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Sanitize path for logging (redact tokens)
			path := sanitizePath(r.URL.Path)

			// Log the request
			logger.Info("request",
				"method", r.Method,
				"path", path,
				"status", wrapped.status,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", sanitizeRemoteAddr(r.RemoteAddr),
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// sanitizePath redacts sensitive information from URL paths.
func sanitizePath(path string) string {
	// Redact streaming tokens in path like /stream/{token}/file
	if strings.HasPrefix(path, "/stream/") && strings.HasSuffix(path, "/file") {
		return "/stream/[REDACTED]/file"
	}
	return path
}

// sanitizeRemoteAddr removes port from remote address for privacy.
func sanitizeRemoteAddr(addr string) string {
	// Just return the IP part, not the port
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// SanitizeForLog replaces sensitive values with [REDACTED].
// Use this when logging any potentially sensitive data.
func SanitizeForLog(key, value string) string {
	sensitiveKeys := []string{
		"password", "token", "secret", "authorization",
		"cookie", "session", "key", "credential",
	}

	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return "[REDACTED]"
		}
	}
	return value
}
