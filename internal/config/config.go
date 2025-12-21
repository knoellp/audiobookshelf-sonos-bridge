package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration values for the bridge service.
type Config struct {
	// Required
	ABSURL          string // Audiobookshelf server URL
	PublicURL       string // URL that Sonos can reach (for streaming)
	SessionSecret   string // Secret for session encryption (min 32 chars)

	// Optional with defaults
	Port              string        // Server port (default: 8080)
	CacheDir          string        // Cache directory (default: /cache)
	ConfigDir         string        // Config directory (default: /config)
	MediaDir          string        // Media directory (default: /media)
	ABSMediaPrefix    string        // Path prefix ABS uses for media (default: /audiobooks)
	PathMappings      []PathMapping // Additional path mappings (format: abs_prefix:local_path,...)
	TranscodeWorkers  int           // Number of parallel transcoding workers (default: 2)
	StreamTokenTTL    time.Duration // Streaming token validity (default: 24h)
	AllowedNetworks   []string      // Allowed networks for streaming (default: all)
	LogLevel          string        // Log level: debug, info, warn, error (default: info)
}

// Load reads configuration from environment variables.
// Returns an error if required variables are missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{}
	var errs []string

	// Required fields
	cfg.ABSURL = os.Getenv("BRIDGE_ABS_URL")
	if cfg.ABSURL == "" {
		errs = append(errs, "BRIDGE_ABS_URL is required")
	}

	cfg.PublicURL = os.Getenv("BRIDGE_PUBLIC_URL")
	if cfg.PublicURL == "" {
		errs = append(errs, "BRIDGE_PUBLIC_URL is required")
	}

	cfg.SessionSecret = os.Getenv("BRIDGE_SESSION_SECRET")
	if cfg.SessionSecret == "" {
		errs = append(errs, "BRIDGE_SESSION_SECRET is required")
	} else if len(cfg.SessionSecret) < 32 {
		errs = append(errs, "BRIDGE_SESSION_SECRET must be at least 32 characters")
	}

	// Optional fields with defaults
	cfg.Port = getEnvOrDefault("BRIDGE_PORT", "8080")
	cfg.CacheDir = getEnvOrDefault("BRIDGE_CACHE_DIR", "/cache")
	cfg.ConfigDir = getEnvOrDefault("BRIDGE_CONFIG_DIR", "/config")
	cfg.MediaDir = getEnvOrDefault("BRIDGE_MEDIA_DIR", "/media")
	cfg.ABSMediaPrefix = getEnvOrDefault("BRIDGE_ABS_MEDIA_PREFIX", "/audiobooks")
	cfg.LogLevel = strings.ToLower(getEnvOrDefault("BRIDGE_LOG_LEVEL", "info"))

	// Parse additional path mappings (format: abs_prefix:local_path,abs_prefix2:local_path2,...)
	pathMappingsStr := os.Getenv("BRIDGE_PATH_MAPPINGS")
	if pathMappingsStr != "" {
		for _, mapping := range strings.Split(pathMappingsStr, ",") {
			parts := strings.SplitN(strings.TrimSpace(mapping), ":", 2)
			if len(parts) == 2 {
				cfg.PathMappings = append(cfg.PathMappings, PathMapping{
					ABSPrefix: parts[0],
					LocalPath: parts[1],
				})
			}
		}
	}

	// Validate log level
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.LogLevel] {
		errs = append(errs, fmt.Sprintf("BRIDGE_LOG_LEVEL must be one of: debug, info, warn, error (got: %s)", cfg.LogLevel))
	}

	// Transcode workers
	workersStr := getEnvOrDefault("BRIDGE_TRANSCODE_WORKERS", "2")
	workers, err := strconv.Atoi(workersStr)
	if err != nil || workers < 1 {
		errs = append(errs, "BRIDGE_TRANSCODE_WORKERS must be a positive integer")
	} else {
		cfg.TranscodeWorkers = workers
	}

	// Stream token TTL
	ttlStr := getEnvOrDefault("BRIDGE_STREAM_TOKEN_TTL", "24h")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		errs = append(errs, fmt.Sprintf("BRIDGE_STREAM_TOKEN_TTL must be a valid duration (got: %s)", ttlStr))
	} else {
		cfg.StreamTokenTTL = ttl
	}

	// Allowed networks (optional, comma-separated)
	networksStr := os.Getenv("BRIDGE_ALLOWED_NETWORKS")
	if networksStr != "" {
		cfg.AllowedNetworks = strings.Split(networksStr, ",")
		for i, n := range cfg.AllowedNetworks {
			cfg.AllowedNetworks[i] = strings.TrimSpace(n)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New("configuration errors: " + strings.Join(errs, "; "))
	}

	return cfg, nil
}

// DatabasePath returns the full path to the SQLite database file.
func (c *Config) DatabasePath() string {
	return c.ConfigDir + "/bridge.db"
}

// PathMapping represents a single ABS path to local path mapping.
type PathMapping struct {
	ABSPrefix string
	LocalPath string
}

// MapABSPathToLocal converts an ABS media path to a local filesystem path.
// Supports multiple path mappings via BRIDGE_PATH_MAPPINGS env var.
func (c *Config) MapABSPathToLocal(absPath string) string {
	// Check additional path mappings first
	for _, mapping := range c.PathMappings {
		if strings.HasPrefix(absPath, mapping.ABSPrefix) {
			return mapping.LocalPath + strings.TrimPrefix(absPath, mapping.ABSPrefix)
		}
	}
	// Fall back to default mapping
	if strings.HasPrefix(absPath, c.ABSMediaPrefix) {
		return c.MediaDir + strings.TrimPrefix(absPath, c.ABSMediaPrefix)
	}
	// If no prefix match, assume the path is already correct
	return absPath
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
