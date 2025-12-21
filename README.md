# Audiobookshelf Sonos Bridge

Play audiobooks from [Audiobookshelf](https://www.audiobookshelf.org/) on your Sonos speakers with automatic progress synchronization.

## Features

- Browse your Audiobookshelf library from a mobile-friendly web interface
- Play audiobooks on any Sonos speaker on your network
- Automatic progress synchronization with Audiobookshelf
- Resume playback from where you left off
- Search and filter your library
- Background cache warming for faster playback
- Secure streaming with short-lived tokens

## Requirements

- [Audiobookshelf](https://www.audiobookshelf.org/) server
- Sonos speakers on the same network
- Docker (recommended) or Go 1.22+
- ffmpeg (for audio transcoding)

## Quick Start

### Using Docker (Recommended)

```bash
docker run -d \
  --name abs-sonos-bridge \
  --network host \
  -e ABS_URL=http://your-audiobookshelf-server:13378 \
  -e PUBLIC_URL=http://your-host-ip:8080 \
  -v /path/to/media:/media:ro \
  -v /path/to/cache:/cache \
  -v /path/to/config:/config \
  ghcr.io/your-username/abs-sonos-bridge:latest
```

### Using Docker Compose

```yaml
version: '3.8'
services:
  abs-sonos-bridge:
    image: ghcr.io/your-username/abs-sonos-bridge:latest
    network_mode: host
    environment:
      - ABS_URL=http://your-audiobookshelf-server:13378
      - PUBLIC_URL=http://your-host-ip:8080
      - LOG_LEVEL=info
    volumes:
      - /path/to/media:/media:ro
      - ./cache:/cache
      - ./config:/config
    restart: unless-stopped
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/your-username/abs-sonos-bridge.git
cd abs-sonos-bridge

# Build
go build -o bridge ./cmd/bridge

# Run
ABS_URL=http://localhost:13378 PUBLIC_URL=http://localhost:8080 ./bridge
```

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `ABS_URL` | Audiobookshelf server URL | **Required** |
| `PUBLIC_URL` | Public URL for this service (must be accessible from Sonos) | **Required** |
| `PORT` | HTTP server port | `8080` |
| `MEDIA_PATH` | Path to media files (same as Audiobookshelf) | `/media` |
| `CACHE_DIR` | Directory for transcoded audio cache | `/cache` |
| `CONFIG_DIR` | Directory for configuration and database | `/config` |
| `LOG_LEVEL` | Logging level: debug, info, warn, error | `info` |
| `SESSION_SECRET` | 32-byte hex secret for session encryption | Auto-generated |
| `TRANSCODE_WORKERS` | Number of concurrent transcoding workers | `2` |

## Usage

1. Open the web interface at `http://your-host:8080`
2. Log in with your Audiobookshelf credentials
3. Browse your library and select an audiobook
4. Click "Refresh Devices" to discover your Sonos speakers
5. Select a speaker and click "Play"

## Network Requirements

This service uses UPnP (SSDP) to discover Sonos devices on your local network. For discovery to work:

- **Docker**: Use `--network host` mode
- **Firewall**: Allow UDP port 1900 (SSDP) and TCP connections to Sonos devices
- **Sonos Access**: The `PUBLIC_URL` must be accessible from your Sonos speakers

## How It Works

1. **Authentication**: Uses your Audiobookshelf credentials for library access
2. **Transcoding**: Converts audio files to MP3 128kbps (Sonos-compatible)
3. **Streaming**: Generates secure, time-limited URLs for Sonos playback
4. **Progress Sync**: Periodically updates your progress in Audiobookshelf

## Troubleshooting

### No Sonos devices found

- Ensure Docker is running with `--network host`
- Check that UDP port 1900 is not blocked
- Verify Sonos speakers are on the same network subnet
- Try clicking "Refresh Devices" multiple times

### Playback doesn't start

- Verify the `PUBLIC_URL` is accessible from your Sonos speakers
- Check that ffmpeg is installed and working
- Review logs with `LOG_LEVEL=debug`

### Progress not syncing

- Ensure you're logged in with valid Audiobookshelf credentials
- Check network connectivity to Audiobookshelf server
- Review logs for sync errors

## Architecture

```
                    +-----------------+
                    | Audiobookshelf  |
                    |     Server      |
                    +--------+--------+
                             |
                    API Calls|Progress Sync
                             |
+---------------+   +--------v--------+   +---------------+
|  Web Browser  |<->| ABS Sonos Bridge|<->| Sonos Speaker |
| (Mobile/PC)   |   |   (This App)    |   |   (UPnP)      |
+---------------+   +--------+--------+   +---------------+
                             |
                    +--------v--------+
                    |  Media Files    |
                    |   (Local/NFS)   |
                    +-----------------+
```

## License

MIT License - See [LICENSE](LICENSE) for details.

## Acknowledgments

- [Audiobookshelf](https://www.audiobookshelf.org/) - Self-hosted audiobook server
- [go-sonos](https://github.com/szatmary/sonern) - Sonos UPnP implementation inspiration
