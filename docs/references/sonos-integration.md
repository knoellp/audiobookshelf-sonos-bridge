# Sonos Integration Reference

Diese Dokumentation enthält alle relevanten Informationen zur Integration des Audiobookshelf Sonos Bridge Projekts mit Sonos-Geräten.

## Inhaltsverzeichnis

- [Sonos System Grundlagen](#sonos-system-grundlagen)
- [Integrationstypen](#integrationstypen)
- [Sonos Control API](#sonos-control-api)
- [Authentifizierung (OAuth 2.0)](#authentifizierung-oauth-20)
- [Discovery: Households, Groups & Players](#discovery-households-groups--players)
- [Unterstützte Audioformate](#unterstützte-audioformate)
- [Event-Subscriptions](#event-subscriptions)
- [Sound Experience Guidelines](#sound-experience-guidelines)
- [Relevanz für unser Projekt](#relevanz-für-unser-projekt)

---

## Sonos System Grundlagen

### Was ist ein Sonos Player?

Ein Sonos Player kann Musik eigenständig abspielen, entweder über integrierte Lautsprecher oder über angeschlossene externe Lautsprecher. Beispiele:
- **Sonos One**: Spielt Musik über eigene Lautsprecher
- **Sonos Amp**: Spielt Musik über angeschlossene Lautsprecher
- **SYMFONISK**: IKEA-Produkte funktionieren ebenfalls als Sonos Player

**Wichtig**: Ein Sonos Sub allein ist kein Player (kann kein Audio allein abspielen). Ein Boost ist ebenfalls kein Player.

### Kommunikation über Wi-Fi

Sonos Player kommunizieren untereinander über Wi-Fi, um synchron zu bleiben. Bei der Entwicklung von Steuerungen sollte beachtet werden, dass Netzwerküberlastung in Haushalten mit vielen Playern und Nutzern schnell auftreten kann.

### Player Bonding

Player können über die Sonos App gebondet werden und verhalten sich dann wie ein logischer Player:
- **Stereo-Paar**: Zwei gleiche Player für Links/Rechts-Kanal
- **Surround**: Home Theater Setup mit Surround-Kanälen
- **Sub-Bonding**: Sonos Sub mit anderem Player für Bass-Kanal

### Gruppen

**Sonos Player arbeiten immer in Gruppen**, auch wenn die Gruppe nur einen Player enthält:
- Alle Player einer Gruppe spielen dasselbe Audio synchron
- Transport-Steuerungen (Play, Pause, Skip) zielen auf **Gruppen**, nicht auf einzelne Player
- Player müssen zum selben Household gehören, um gruppiert zu werden

### Continuity of Control

Ein zentrales Sonos-Konzept: Musik kann über verschiedene Wege gestartet und gesteuert werden:
- Sonos App
- Partner-Apps und -Geräte
- Hardware-Buttons auf Sonos Playern
- Sprachsteuerung

Die Steuerung ist nahtlos und integriert - egal wie die Wiedergabe gestartet wurde.

---

## Integrationstypen

### Connected Home Partner (Relevant für unser Projekt)

Wenn man keinen eigenen Content-Katalog besitzt, aber Hardware oder Apps zur Steuerung von Sonos-Speakern anbieten möchte, ist man ein **Connected Home Partner**.

**Unser Projekt fällt in diese Kategorie**: Wir steuern Sonos-Player und streamen Content von Audiobookshelf.

### Content Service Partner

Für Anbieter mit eigenem Katalog (Musik, Radio, Podcasts, Hörbücher, On-Demand-Tracks). Erfordert separate Lizenzierung für Business/Commercial Accounts.

---

## Sonos Control API

Die Sonos Control API ist ein JSON-basiertes Anwendungsprotokoll zur Steuerung der Audio-Wiedergabe auf Sonos Playern.

### Architektur

```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│   Integration   │ ──────► │   Sonos Cloud   │ ──────► │  Sonos Player   │
│   (Unser Code)  │ ◄────── │  api.ws.sonos   │ ◄────── │   (Household)   │
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

**Wichtig**: Die Control API auf dem LAN ist **nicht für breite Veröffentlichung verfügbar**. Die Cloud-basierte API muss verwendet werden.

### API Gateway

```
Base URL: https://api.ws.sonos.com/control/api
Version: v1
```

### Request-Format

```http
https://api.ws.sonos.com/control/api/v{version}/{target}/{target ID}/{namespace}/{command}
```

**Beispiel**:
```http
POST https://api.ws.sonos.com/control/api/v1/groups/RINCON_00012345678001400:0/playback/play
```

### Targets

| Target | Beispiel-Pfad |
|--------|---------------|
| Household | `/households/[householdId]` |
| Group | `/groups/[groupId]` |
| Session | `/playbackSessions/[sessionId]` |
| Player | `/players/[playerId]` |

### HTTP Headers

| Header | Beschreibung |
|--------|--------------|
| `Authorization` | `Bearer {token}` |
| `Content-Type` | `application/json` |
| `Content-Length` | Anzahl der Zeichen (0 bei leerem Body) |
| `User-Agent` | Empfohlen zur Identifikation |

### Namespaces

Namespaces beschreiben zusammenhängende Commands und Events:

- **playback**: Play, Pause, Skip, Wiedergabestatus
- **groupVolume**: Gruppen-Lautstärke
- **playerVolume**: Player-Lautstärke
- **groups**: Gruppenverwaltung
- **favorites**: Favoriten
- **playlists**: Playlists

### Response Status Codes

| Code | Beschreibung |
|------|--------------|
| 200 | OK - Command erfolgreich |
| 400 | Bad Request - Syntaxfehler, fehlende/ungültige Parameter |
| 401 | Unauthorized - Token/API-Key ungültig |
| 403 | Forbidden - Kein Zugriff auf Ressource |
| 404 | Not Found - Unbekannte Ressource |
| 410 | Gone - Ressource existiert nicht mehr (z.B. gelöschte Gruppe) |
| 429 | Too Many Requests - Rate Limit erreicht |
| 499 | Custom - Playback/Session/Control API Fehler mit globalError |
| 500 | Internal Server Error |

### Beispiel: Play Command

**Request**:
```http
POST /groups/RINCON_00012345678001400:0/playback/play HTTP/1.1
Host: api.ws.sonos.com/control/api/v1
Content-Type: application/json
Authorization: Bearer <token>
User-Agent: AudiobookshelfSonosBridge/1.0
```

**Response**:
```http
HTTP/1.1 200 OK
Content-Type: application/json
X-Sonos-Type: none

{}
```

---

## Authentifizierung (OAuth 2.0)

Die Sonos API verwendet OAuth 2.0 mit Three-Legged Authentication.

### Ablauf

1. **Integration → User → Sonos**: User wird zum Sonos Login Service geschickt
2. **User → Sonos → Integration**: User authentifiziert, Sonos sendet Authorization Code
3. **Integration → Sonos**: Authorization Code wird gegen Access/Refresh Token getauscht

### Schritt 1: Client Credentials erstellen

1. Auf https://integration.sonos.com/integrations einloggen
2. "New control integration" klicken
3. Name, Beschreibung und Kategorie ausfüllen
4. Key-Name eingeben und speichern
5. Redirect URL registrieren (muss HTTPS und öffentlich erreichbar sein)

### Schritt 2: User zum Login schicken

```
GET https://api.sonos.com/login/v3/oauth
  ?client_id={API_KEY}
  &response_type=code
  &state={OPAQUE_STATE}
  &scope=playback-control-all
  &redirect_uri={URL_ENCODED_REDIRECT_URI}
```

**Scope `playback-control-all` ermöglicht**:
- Sehen was läuft
- Wiedergabe und Lautstärke ändern
- Räume und Gruppen ändern
- Favoriten und Playlists abspielen

### Schritt 3: Access Token abrufen

```bash
curl -X POST \
  -H "Content-Type: application/x-www-form-urlencoded;charset=utf-8" \
  -H "Authorization: Basic {BASE64_ENCODED_CLIENT_ID:SECRET}" \
  "https://api.sonos.com/login/v3/oauth/access" \
  -d "grant_type=authorization_code&code={AUTH_CODE}&redirect_uri={REDIRECT_URI}"
```

**Response**:
```json
{
  "access_token": "d7cdf58d-d43c-412c-8887-6d7f95b6557e",
  "token_type": "Bearer",
  "expires_in": 86400,
  "refresh_token": "585fb433-359c-4419-ac53-946106e5bbab",
  "scope": "playback-control-all"
}
```

### Token Refresh

Access Tokens laufen nach **24 Stunden** ab. Refresh mit:

```bash
curl -X POST \
  -H "Content-Type: application/x-www-form-urlencoded;charset=utf-8" \
  -H "Authorization: Basic {BASE64_ENCODED_CLIENT_ID:SECRET}" \
  "https://api.sonos.com/login/v3/oauth/access" \
  -d "grant_type=refresh_token&refresh_token={REFRESH_TOKEN}"
```

---

## Discovery: Households, Groups & Players

### Sonos Object Model

```
Account
  └── Household(s)
        └── Group(s)
              └── Player(s)
```

### Households abrufen

```http
GET /control/api/v1/households HTTP/1.1
Host: api.ws.sonos.com
Authorization: Bearer {token}
```

**Response**:
```json
{
  "households": [
    { "id": "Sonos_HHID-4321" },
    { "id": "Sonos_HHID-1234" }
  ]
}
```

**Hinweis**: Die meisten User haben nur ein Household. Bei mehreren sollte eine Auswahlmöglichkeit angeboten werden.

### Groups und Players abrufen

```http
GET /control/api/v1/households/{householdId}/groups HTTP/1.1
Host: api.ws.sonos.com
Authorization: Bearer {token}
```

**Response** (gekürzt):
```json
{
  "groups": [
    {
      "id": "RINCON_7BHBFF96BF5A34300",
      "name": "Playroom",
      "coordinatorId": "RINCON_8HJLQE01RW4B21097",
      "playbackState": "PLAYBACK_STATE_IDLE",
      "playerIds": ["RINCON_8HJLQE01RW4B21097"]
    }
  ],
  "players": [
    {
      "id": "RINCON_8HJLQE01RW4B21097",
      "name": "Playroom",
      "softwareVersion": "38.5-43170",
      "apiVersion": "1.0.0",
      "minApiVersion": "1.0.0",
      "capabilities": ["PLAYBACK", "CLOUD"]
    }
  ]
}
```

### ID-Eigenschaften

| ID-Typ | Eigenschaften |
|--------|---------------|
| Household ID | Stabil, kann sich aber bei Netzwerkwechsel ändern |
| Group ID | Ephemer (kurzlebig) |
| Session ID | Ephemer |
| Player ID | Permanent und immutabel (basiert auf MAC-Adresse) |

---

## Unterstützte Audioformate

### Codecs und Formate

| Codec | Formate | MIME Types | Sample Rates | Transport |
|-------|---------|------------|--------------|-----------|
| AAC-LC, HE-AAC, HEv2-AAC | .m4a, .mp4, .aac | audio/mp4, audio/aac, application/x-mpegURL | 8-48 kHz | HTTP, HTTPS |
| FLAC | .flac | audio/flac | 8-48 kHz | HTTP, HTTPS |
| MP3 | .mp3 | audio/mp3, audio/mpeg3, audio/mpeg | 16-48 kHz | HTTP, HTTPS |
| Ogg Vorbis | .ogg | application/ogg | 8-48 kHz | HTTP, HTTPS |
| WMA | .asf, .wma | audio/wma, audio/x-ms-wma | 8-48 kHz | HTTP, HTTPS, MMS, RTSP |

### Wichtige Hinweise

- **Fragmented MP4** wird unterstützt
- **WMA Voice Files** werden **nicht** unterstützt
- Sonos unterstützt beliebige Bitraten
- Bei **VBR-kodierten Dateien** ist eine **TOC im Xing Header** erforderlich für korrektes Scrubbing

### Metadata

**Für Streaming (nicht HLS)**: Icecast2 ICY Streaming Protocol
- Header: `icy-metadata: 1`
- `icy-metaint` und `icy-name` Header
- `StreamTitle` Key für eingebettete Metadaten

**Für HLS**: ID3v2 Frames
- TALB (Album), TIT2 (Titel), TPE1 (Künstler), TRCK (Track), etc.
- WXXX für dynamisches Artwork: `artworkURL_640x\0http://example.com/art.jpg`

---

## Event-Subscriptions

### Übersicht

Subscriptions ermöglichen den Empfang von State-Change-Events:
- Lautstärkeänderungen
- Wiedergabestatus
- Playback-Fehler
- Metadaten-Änderungen

### Callback URL registrieren

Im Integration Manager die Callback URL für jeden API Key registrieren. Anforderungen:
- HTTPS erforderlich
- HTTP 1.1 mit Persistent Connections
- SSL/TLS v1.2
- Gültiges CA-signiertes X.509 Zertifikat

### Subscribe

```http
POST /groups/{groupId}/groupVolume/subscription HTTP/1.1
Host: api.ws.sonos.com/control/api/v1
Authorization: Bearer {token}
```

### Event empfangen

Sonos sendet HTTP POST an die Callback URL mit folgenden Headern:

| Header | Beschreibung |
|--------|--------------|
| `X-Sonos-Household-Id` | Household-ID |
| `X-Sonos-Namespace` | Namespace des Events |
| `X-Sonos-Type` | Event-Typ |
| `X-Sonos-Target-Type` | Ziel-Typ (z.B. `groupId`) |
| `X-Sonos-Target-Value` | Ziel-ID |
| `X-Sonos-Event-Seq-Id` | Sequenz-ID für Reihenfolge |
| `X-Sonos-Event-Signature` | Kryptographische Signatur zur Verifizierung |

### Event-Signatur verifizieren

SHA-256 Hash aus:
1. X-Sonos-Event-Seq-Id
2. X-Sonos-Namespace
3. X-Sonos-Type
4. X-Sonos-Target-Type
5. X-Sonos-Target-Value
6. Client ID
7. Client Secret

Base64-kodiert (URL-safe, ohne Padding).

### Subscription Lifetime

- **Maximum**: 3 Tage
- Resubscribe verlängert um weitere 3 Tage
- Bei Player-Shutdown: Subscriptions werden nach 30 Sekunden gelöscht
- Andere Gruppenmitglieder können Subscriptions übernehmen

---

## Sound Experience Guidelines

### Kernprinzipien

1. **Joyous**: Erlebnisse sollen Freude bereiten
2. **Easy**: Einfach für die ganze Familie
3. **Awesome Sound**: Hervorragende Klangqualität

### Continuity of Control umsetzen

- Ermögliche Steuerung unabhängig davon, wie die Wiedergabe gestartet wurde
- Zeige an, was gerade läuft
- Ermögliche grundlegende Steuerung (Play/Pause, Lautstärke, Skip)

### Release-Richtlinien

**Erlaubt**:
- Sagen, dass App/Gerät "Compatible with Sonos" ist
- Über die Integration und Sonos-Komponenten sprechen
- Auf Sonos-Website verlinken

**Nicht erlaubt**:
- "Works with Sonos" Badge verwenden (nur mit Zertifizierung)
- Sonos Logo verwenden
- Offizielle Beziehung implizieren

---

## Relevanz für unser Projekt

### Architektur-Entscheidungen

Laut `CLAUDE.md` verwendet unser Projekt **UPnP für Sonos-Steuerung**. Die offizielle Sonos Cloud API bietet jedoch einige Vorteile:

| Aspekt | UPnP (lokal) | Cloud API |
|--------|--------------|-----------|
| Latenz | Niedrig | Höher |
| Internet | Nicht erforderlich | Erforderlich |
| Offiziell unterstützt | Begrenzt | Ja |
| OAuth erforderlich | Nein | Ja |
| Zuverlässigkeit | Abhängig von Netzwerk | Konsistent |

### Empfohlene Audioformate für Hörbücher

Für unser Audiobookshelf-Projekt empfehlen sich:

1. **MP3** - Universell kompatibel, gute Kompression für Sprache
2. **AAC/M4A** - Bessere Qualität bei gleicher Bitrate
3. **FLAC** - Für verlustfreie Archivierung (größere Dateien)

### Wichtige Endpunkte für Hörbuch-Wiedergabe

1. **Discovery**: `/households` → `/groups` für Raumauswahl
2. **Playback**: `/groups/{id}/playback/play`, `/pause`, `/skipToNextTrack`
3. **Volume**: `/groups/{id}/groupVolume`
4. **Seek**: Für Kapitelnavigation und Progress-Tracking

### Sicherheitsaspekte

Wie in `CLAUDE.md` definiert:
- Streaming-Endpoints nutzen kurzlebige, session-spezifische Tokens
- Audiobookshelf-Credentials bleiben serverseitig
- Niemals an Sonos exponieren

---

## Referenzen

- [Sonos Developer Portal](https://docs.sonos.com)
- [How Sonos Works](https://docs.sonos.com/docs/how-sonos-works)
- [Connected Home Get Started](https://docs.sonos.com/docs/connected-home-get-started)
- [Control API](https://docs.sonos.com/docs/control)
- [Authorize](https://docs.sonos.com/docs/authorize)
- [Discover](https://docs.sonos.com/docs/discover)
- [Subscribe](https://docs.sonos.com/docs/subscribe)
- [Supported Audio Formats](https://docs.sonos.com/docs/supported-audio-formats)
- [Sound Experience Guidelines](https://docs.sonos.com/docs/sound-experience-guidelines)
- [Control API Reference](https://docs.sonos.com/reference/about-control-api)
