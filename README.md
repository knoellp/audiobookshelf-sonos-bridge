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
  -e BRIDGE_ABS_URL=http://your-audiobookshelf-server:13378 \
  -e BRIDGE_PUBLIC_URL=http://your-host-ip:8080 \
  -e BRIDGE_SESSION_SECRET=$(openssl rand -hex 32) \
  -v /path/to/media:/media:ro \
  -v /path/to/cache:/cache \
  -v /path/to/config:/config \
  ghcr.io/knoellp/audiobookshelf-sonos-bridge:latest
```

### Using Docker Compose

1. Download the example compose file:
```bash
curl -O https://raw.githubusercontent.com/knoellp/audiobookshelf-sonos-bridge/main/docker-compose.example.yml
mv docker-compose.example.yml docker-compose.yml
```

2. Edit `docker-compose.yml` with your settings:
   - `BRIDGE_ABS_URL`: Your Audiobookshelf server URL
   - `BRIDGE_PUBLIC_URL`: This server's IP (must be accessible from Sonos)
   - `BRIDGE_SESSION_SECRET`: Generate with `openssl rand -hex 32`
   - Volume path for `/media`: Same path as Audiobookshelf uses

3. Start the service:
```bash
docker compose up -d
```

4. Open `http://your-server-ip:8080` in your browser

### Building from Source

```bash
# Clone the repository
git clone https://github.com/knoellp/audiobookshelf-sonos-bridge.git
cd audiobookshelf-sonos-bridge

# Build (requires Go 1.22+ and ffmpeg)
go build -o bridge ./cmd/bridge

# Run
BRIDGE_ABS_URL=http://localhost:13378 \
BRIDGE_PUBLIC_URL=http://localhost:8080 \
BRIDGE_SESSION_SECRET=dev-secret-at-least-32-characters \
./bridge
```

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `BRIDGE_ABS_URL` | Audiobookshelf server URL | **Required** |
| `BRIDGE_PUBLIC_URL` | Public URL for this service (must be accessible from Sonos) | **Required** |
| `BRIDGE_SESSION_SECRET` | Secret for session encryption (min 32 chars) | **Required** |
| `BRIDGE_PORT` | HTTP server port | `8080` |
| `BRIDGE_MEDIA_DIR` | Path to media files inside container | `/media` |
| `BRIDGE_CACHE_DIR` | Directory for transcoded audio cache | `/cache` |
| `BRIDGE_CONFIG_DIR` | Directory for configuration and database | `/config` |
| `BRIDGE_LOG_LEVEL` | Logging level: debug, info, warn, error | `info` |
| `BRIDGE_TRANSCODE_WORKERS` | Number of concurrent transcoding workers | `2` |
| `BRIDGE_ABS_MEDIA_PREFIX` | Path prefix ABS uses for media files | `/audiobooks` |

**Docker Compose volume paths** (in `.env` file):

| Variable | Description |
|----------|-------------|
| `MEDIA_PATH` | Host path to your audiobooks directory |
| `CACHE_PATH` | Host path for transcoded audio cache |
| `CONFIG_PATH` | Host path for configuration and database |

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
2. **Transcoding**: Remuxes or transcodes audio to Sonos-compatible formats (AAC/MP3/FLAC)
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
