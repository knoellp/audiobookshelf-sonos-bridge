# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-12-16

### Added

#### Core Features
- Audiobookshelf authentication with encrypted token storage
- Library browsing with grid and list views
- Search and filter functionality (by author, series, progress)
- Sonos device discovery via UPnP/SSDP
- Single-zone playback on Sonos speakers
- Transport controls (play, pause, seek, stop)
- Progress synchronization with Audiobookshelf
- Resume playback from last position

#### Technical Features
- File-based audio caching with MP3 transcoding (128kbps CBR)
- Background transcoding workers with queue management
- Secure streaming with HMAC-signed, time-limited tokens
- HTTP Range request support for seeking
- SQLite persistence for sessions, cache, devices, and playback state
- Structured logging with slog

#### Web Interface
- Mobile-responsive design with touch-friendly controls
- PWA support with web app manifest
- htmx-powered partial page updates
- Real-time playback status polling
- Loading states and error feedback

#### Robustness
- Automatic retry with exponential backoff for network requests
- Disk space checks before transcoding
- Startup cleanup for stale sessions and cache entries
- Graceful shutdown with proper resource cleanup

#### Operations
- Docker support with multi-stage builds
- Health check endpoint
- Configurable via environment variables
- Comprehensive logging

### Security
- AES-256-GCM encryption for stored tokens
- Session-specific streaming tokens
- Audiobookshelf credentials never exposed to Sonos
- No credentials stored in logs

## [Unreleased]

### Planned
- Chapter navigation support
- Multi-room/group playback
- Podcast support
- Cover art on Sonos display
- Sleep timer functionality
