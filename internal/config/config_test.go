package config

import (
	"os"
	"strings"
	"testing"
)

func clearEnv() {
	os.Unsetenv("BRIDGE_ABS_URL")
	os.Unsetenv("BRIDGE_PUBLIC_URL")
	os.Unsetenv("BRIDGE_SESSION_SECRET")
	os.Unsetenv("BRIDGE_PORT")
	os.Unsetenv("BRIDGE_CACHE_DIR")
	os.Unsetenv("BRIDGE_CONFIG_DIR")
	os.Unsetenv("BRIDGE_MEDIA_DIR")
	os.Unsetenv("BRIDGE_TRANSCODE_WORKERS")
	os.Unsetenv("BRIDGE_STREAM_TOKEN_TTL")
	os.Unsetenv("BRIDGE_ALLOWED_NETWORKS")
	os.Unsetenv("BRIDGE_LOG_LEVEL")
}

func setRequiredEnv() {
	os.Setenv("BRIDGE_ABS_URL", "http://audiobookshelf:13378")
	os.Setenv("BRIDGE_PUBLIC_URL", "http://192.168.1.100:8080")
	os.Setenv("BRIDGE_SESSION_SECRET", "this-is-a-secret-key-at-least-32chars")
}

func TestLoad_RequiredFieldsMissing(t *testing.T) {
	clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "BRIDGE_ABS_URL") {
		t.Error("expected error to mention BRIDGE_ABS_URL")
	}
	if !strings.Contains(errStr, "BRIDGE_PUBLIC_URL") {
		t.Error("expected error to mention BRIDGE_PUBLIC_URL")
	}
	if !strings.Contains(errStr, "BRIDGE_SESSION_SECRET") {
		t.Error("expected error to mention BRIDGE_SESSION_SECRET")
	}
}

func TestLoad_SessionSecretTooShort(t *testing.T) {
	clearEnv()
	os.Setenv("BRIDGE_ABS_URL", "http://audiobookshelf:13378")
	os.Setenv("BRIDGE_PUBLIC_URL", "http://192.168.1.100:8080")
	os.Setenv("BRIDGE_SESSION_SECRET", "short")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short session secret")
	}

	if !strings.Contains(err.Error(), "at least 32 characters") {
		t.Errorf("expected error about 32 characters, got: %v", err)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	clearEnv()
	setRequiredEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ABSURL != "http://audiobookshelf:13378" {
		t.Errorf("unexpected ABSURL: %s", cfg.ABSURL)
	}
	if cfg.PublicURL != "http://192.168.1.100:8080" {
		t.Errorf("unexpected PublicURL: %s", cfg.PublicURL)
	}
	if cfg.SessionSecret != "this-is-a-secret-key-at-least-32chars" {
		t.Errorf("unexpected SessionSecret: %s", cfg.SessionSecret)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	setRequiredEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got: %s", cfg.Port)
	}
	if cfg.CacheDir != "/cache" {
		t.Errorf("expected default cache dir /cache, got: %s", cfg.CacheDir)
	}
	if cfg.ConfigDir != "/config" {
		t.Errorf("expected default config dir /config, got: %s", cfg.ConfigDir)
	}
	if cfg.MediaDir != "/media" {
		t.Errorf("expected default media dir /media, got: %s", cfg.MediaDir)
	}
	if cfg.TranscodeWorkers != 2 {
		t.Errorf("expected default transcode workers 2, got: %d", cfg.TranscodeWorkers)
	}
	if cfg.StreamTokenTTL.Hours() != 24 {
		t.Errorf("expected default token TTL 24h, got: %v", cfg.StreamTokenTTL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got: %s", cfg.LogLevel)
	}
	if len(cfg.AllowedNetworks) != 0 {
		t.Errorf("expected no allowed networks by default, got: %v", cfg.AllowedNetworks)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("BRIDGE_PORT", "9090")
	os.Setenv("BRIDGE_CACHE_DIR", "/custom/cache")
	os.Setenv("BRIDGE_CONFIG_DIR", "/custom/config")
	os.Setenv("BRIDGE_MEDIA_DIR", "/custom/media")
	os.Setenv("BRIDGE_TRANSCODE_WORKERS", "4")
	os.Setenv("BRIDGE_STREAM_TOKEN_TTL", "12h")
	os.Setenv("BRIDGE_ALLOWED_NETWORKS", "192.168.0.0/16, 10.0.0.0/8")
	os.Setenv("BRIDGE_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got: %s", cfg.Port)
	}
	if cfg.CacheDir != "/custom/cache" {
		t.Errorf("expected cache dir /custom/cache, got: %s", cfg.CacheDir)
	}
	if cfg.ConfigDir != "/custom/config" {
		t.Errorf("expected config dir /custom/config, got: %s", cfg.ConfigDir)
	}
	if cfg.MediaDir != "/custom/media" {
		t.Errorf("expected media dir /custom/media, got: %s", cfg.MediaDir)
	}
	if cfg.TranscodeWorkers != 4 {
		t.Errorf("expected transcode workers 4, got: %d", cfg.TranscodeWorkers)
	}
	if cfg.StreamTokenTTL.Hours() != 12 {
		t.Errorf("expected token TTL 12h, got: %v", cfg.StreamTokenTTL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got: %s", cfg.LogLevel)
	}
	if len(cfg.AllowedNetworks) != 2 {
		t.Errorf("expected 2 allowed networks, got: %d", len(cfg.AllowedNetworks))
	}
	if cfg.AllowedNetworks[0] != "192.168.0.0/16" {
		t.Errorf("expected first network 192.168.0.0/16, got: %s", cfg.AllowedNetworks[0])
	}
}

func TestLoad_InvalidTranscodeWorkers(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("BRIDGE_TRANSCODE_WORKERS", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid transcode workers")
	}

	if !strings.Contains(err.Error(), "BRIDGE_TRANSCODE_WORKERS") {
		t.Errorf("expected error about transcode workers, got: %v", err)
	}
}

func TestLoad_InvalidTokenTTL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("BRIDGE_STREAM_TOKEN_TTL", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid token TTL")
	}

	if !strings.Contains(err.Error(), "BRIDGE_STREAM_TOKEN_TTL") {
		t.Errorf("expected error about token TTL, got: %v", err)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("BRIDGE_LOG_LEVEL", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}

	if !strings.Contains(err.Error(), "BRIDGE_LOG_LEVEL") {
		t.Errorf("expected error about log level, got: %v", err)
	}
}

func TestDatabasePath(t *testing.T) {
	clearEnv()
	setRequiredEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "/config/bridge.db"
	if cfg.DatabasePath() != expected {
		t.Errorf("expected database path %s, got: %s", expected, cfg.DatabasePath())
	}
}
