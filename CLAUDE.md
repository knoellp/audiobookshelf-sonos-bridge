# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Audiobookshelf Sonos Bridge - A local service that enables audiobook playback from Audiobookshelf on Sonos devices with per-user progress tracking and a web UI for library browsing/filtering.

## WICHTIG: Immer Docker verwenden!

**Die App läuft IMMER als Docker Container. Niemals direkt ausführen!**

### App starten (Build + Run):
```bash
cd /home/knoellp/develop/server/audiobook-streamer
docker compose up --build -d
```

### Logs ansehen:
```bash
docker compose logs -f
```

### App stoppen:
```bash
docker compose down
```

### App neu starten nach Code-Änderungen:
```bash
docker compose up --build -d
```

## Development (Tests lokal)

```bash
# Tests ausführen (lokal, ohne Docker)
go test ./...

# Einzelner Test
go test -v -run TestHandler_HandleStream ./internal/stream/...

# Tests mit Coverage
go test -cover ./...

# Dependencies
go mod tidy
```

**Templates:** Standard Go `html/template` in `web/templates/`. Änderungen werden bei Container-Neustart geladen.

## Architecture Principles

**File-based caching over live transcoding**: Sonos receives file URLs from a cache directory, not live-transcoded streams.

**Audiobookshelf as source of truth**: User accounts, library data, metadata, and playback progress are read from and written back to Audiobookshelf.

**Sonos control via UPnP**: Device discovery and control use Universal Plug and Play; no deprecated Sonos mechanisms.

**Security model**: Streaming endpoints use short-lived, session-specific tokens. Audiobookshelf credentials stay server-side; never exposed to Sonos.

## Transcoding Strategy (WICHTIG!)

**Immer die schnellste Variante wählen!** Nur transkodieren wenn absolut nötig.

1. **Remux** (schnell): Wenn Codec Sonos-kompatibel ist (AAC, MP3, FLAC), nur Container ändern
2. **Transcode** (langsam): Nur bei inkompatiblen Codecs (Opus, Vorbis, WMA, etc.) → MP3 128kbps

**Dynamische Cache-Dateiendungen:** Basierend auf dem tatsächlichen Output-Format:
- AAC → `audio.m4a` (Content-Type: `audio/mp4`)
- MP3 → `audio.mp3` (Content-Type: `audio/mpeg`)
- FLAC → `audio.flac` (Content-Type: `audio/flac`)

**Cache-Pfade:** Direkt unter `/cache/{item_id}/audio.{ext}` (keine Versionierung im Pfad).

**Cache invalidieren:** Bei Breaking Changes einfach den Cache-Ordner löschen und Container neu starten.

### M4A/MP4 Kompatibilität mit älteren Sonos-Geräten

**Problem (entdeckt 2025-12-21):** Ältere Sonos-Geräte wie der **Sonos Connect (ZP90)** können M4A-Dateien mit dem Standard-ftyp-Brand `isom` (ISO Base Media) nicht abspielen. Die Wiedergabe geht in TRANSITIONING und dann direkt zu STOPPED, ohne Audio auszugeben.

**Lösung:** Beim Remuxen zu M4A muss der `ipod` Muxer mit `-brand M4A` verwendet werden:
```bash
ffmpeg -i input.m4b -map 0:a -map_chapters -1 -c:a copy -vn -movflags +faststart -brand M4A -f ipod output.m4a
```

**WICHTIG:** Der `-brand` Flag funktioniert NUR mit dem `ipod` Muxer, NICHT mit `-f mp4`! Der Standard `mp4` Muxer ignoriert den Brand-Flag und produziert immer `isom`.

**Warum funktioniert das?**
- Der `mp4` Muxer erzeugt immer `isom` Brand (ignoriert `-brand`)
- Der `ipod` Muxer respektiert den `-brand M4A` Flag
- `isom` ist ein generisches ISO Base Media Format
- `M4A ` ist das Apple-spezifische Audio-Format
- Ältere Sonos-Firmware (z.B. auf ZP90) erkennt nur `M4A ` als gültiges AAC-Audio-Format
- Neuere Geräte (z.B. Play:1, ZPS12) funktionieren mit beiden Brands

**Betroffene Geräte:**
- Sonos Connect (ZP90) - braucht `M4A` brand
- Sonos Play:1 (ZPS12) - funktioniert mit beiden

**Diese Lösung ist in `internal/cache/transcoder.go` implementiert** - sowohl in `Remux()` als auch in `RemuxMultiple()`.

## Data Flow

```
1. User klickt "Play" im Browser
2. PlayerHandler prüft Cache-Status (IsCached)
3. Falls nicht gecached: Worker transkodiert/remuxt → speichert mit korrekter Endung
4. StreamHandler generiert Token-URL
5. Sonos ruft /stream/{token}/audio.* ab
6. StreamHandler liest CacheEntry, ermittelt Format, liefert Datei mit korrektem Content-Type
7. ProgressSyncer synchronisiert Position zurück zu Audiobookshelf
```

## Module Separation

The codebase maintains clear separation between:
- `internal/abs/` - Audiobookshelf API client
- `internal/sonos/` - Sonos UPnP client (discovery, AVTransport)
- `internal/cache/` - Cache index, transcoding, background workers
- `internal/stream/` - Streaming endpoints with token validation
- `internal/web/` - HTTP handlers, authentication, templates
- `internal/store/` - SQLite persistence layer
- `internal/config/` - Configuration management

## Container Layout

| Mount Point | Purpose | Mode |
|-------------|---------|------|
| `/media` | Media files (local or network share) | read-only |
| `/cache` | Transcoded audio cache | read-write |
| `/config` | Configuration and database | read-write |

Host network mode is recommended for reliable UPnP device discovery.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ABS_URL` | Audiobookshelf server URL | Required |
| `PUBLIC_URL` | Public URL for streaming | Required |
| `PORT` | HTTP server port | `8080` |
| `MEDIA_PATH` | Path to media files | `/media` |
| `CACHE_DIR` | Cache directory | `/cache` |
| `CONFIG_DIR` | Config directory | `/config` |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `SESSION_SECRET` | 32-byte hex secret for encryption | Auto-generated |
| `TRANSCODE_WORKERS` | Number of transcoding workers | `2` |

## Version 1 Scope Boundaries

In scope: Playback, seek, resume, progress sync, library search/filter, single zone selection.

Out of scope: Sonos app integration, multiroom grouping, chapter navigation, library management.

## Production Environment (knoellp's Server)

**Host-Pfade:**
- Audiobooks: `/mnt/roon-music/Audiobooks/audible` (read-only)
- Cache: `/mnt/sonos-cache`

**URLs:**
- ABS Server: `http://192.168.1.160:13378`
- Bridge Public URL: `http://100.89.56.20:8081`

**Path Mapping:**
- ABS verwendet `/audiobooks` als Media-Root
- Mappt zu lokalem `/mnt/roon-music/Audiobooks`

## Test Credentials

**WICHTIG:** Die Test-Zugangsdaten für E2E-Tests befinden sich in `.test-credentials`. Diese Datei MUSS bei jedem Test gelesen werden!

```bash
cat .test-credentials
```

Enthält: `TEST_USER`, `TEST_PASS`, `SONOS_DEVICE` (Büro)
