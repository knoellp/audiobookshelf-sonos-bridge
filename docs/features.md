# Geplante Features

## Bibliotheks-Filter und Serien-Darstellung

**Status:** Geplant
**Priorität:** Hoch (wichtig für finale App-Funktion)

### Beschreibung

Übernahme der Filter- und Darstellungsmöglichkeiten aus Audiobookshelf:
- **Serien (Reihen):** Zusammengehörige Bücher gruppiert darstellen
- **Autoren:** Filtern und Browsen nach Autor
- **Genres:** Kategorien wie Science Fiction, Sachbuch, Krimi, etc.
- **Tags:** Benutzerdefinierte Tags
- **Sprecher (Narrators):** Filtern nach Hörbuch-Sprecher

### Bestandsaufnahme: Was ist bereits vorhanden?

#### API-Infrastruktur (bereits implementiert)

1. **`GetFilterData()`** in `internal/abs/client.go:205`
   ```go
   // Gibt zurück: Authors, Series, Genres, Tags, Narrators, Languages, Publishers
   func (c *Client) GetFilterData(ctx context.Context, libraryID string) (*FilterData, error)
   ```

2. **`ItemsOptions.Filter`** in `internal/abs/client.go:443`
   - Unterstützt bereits Filter-Parameter
   - Format: `filter=authors.BASE64_ID` oder `filter=genres.BASE64_VALUE`

3. **Metadaten pro Buch** in `BookMetadata`:
   - `Series []Series` mit ID, Name und **Sequence** (Reihenfolge!)
   - `Authors []Author` mit ID und Name
   - `Genres []string`

#### Audiobookshelf Filter-Syntax

```
# Filter nach Autor (ID ist Base64-kodiert)
filter=authors.YXV0X3ozbGVpbWd5Ymw3dWYzeTRhYg==

# Filter nach Genre (Wert ist Base64-kodiert)
filter=genres.U2NpZW5jZSBGaWN0aW9u

# Filter nach Serie
filter=series.c2VyX2FiYzEyMw==

# Serien zusammenfassen (zeigt nur ein Item pro Serie)
collapseseries=1
```

### UI-Konzept

#### 1. Responsive Navigation

**Desktop (Tab-Bar, immer sichtbar):**
```
┌─────────────────────────────────────────────────────────────┐
│  Logo                [Zuletzt] [Serien] [Autoren] [Genres]  │
└─────────────────────────────────────────────────────────────┘
```

**Mobil (Burger-Menü, platzsparend):**
```
┌─────────────────────────┐
│  Logo            [☰]   │  ← Burger-Icon
└─────────────────────────┘

      ↓ Klick auf [☰]

┌─────────────────────────┐
│  Navigation         ✕  │
├─────────────────────────┤
│  📚 Zuletzt gehört      │
│  📖 Alle Bücher         │
│  📚 Serien              │
│  👤 Autoren             │
│  🏷️ Genres              │
└─────────────────────────┘
```

#### 2. "Zuletzt gehört" Ansicht

```
┌─────────────────────────────────────────────────┐
│  Zuletzt gehört                                 │
├─────────────────────────────────────────────────┤
│  ┌─────┐                                        │
│  │Cover│  Der Herr der Ringe            [▶️]   │
│  │     │  45% · Vor 2 Stunden                   │
│  └─────┘                                        │
│  ┌─────┐                                        │
│  │Cover│  Die drei ??? - Folge 42       [▶️]   │
│  │     │  23% · Gestern                         │
│  └─────┘                                        │
│  ┌─────┐                                        │
│  │Cover│  Sherlock Holmes               [▶️]   │
│  │     │  100% · Vor 3 Tagen (Fertig)           │
│  └─────┘                                        │
└─────────────────────────────────────────────────┘
```

**Datenquelle:** Audiobookshelf `/api/me/items-in-progress` oder lokale Playback-Historie

#### 3. Serien-Ansicht

**Liste aller Serien:**
```
┌─────────────────────────────────────────────────┐
│  Serien                              🔍 Filter  │
├─────────────────────────────────────────────────┤
│  ┌─────┐                                        │
│  │Cover│  Die drei ???                          │
│  │     │  12 Bücher                             │
│  └─────┘                                        │
│  ┌─────┐                                        │
│  │Cover│  Harry Potter                          │
│  │     │  7 Bücher                              │
│  └─────┘                                        │
│  ┌─────┐                                        │
│  │Cover│  Sherlock Holmes                       │
│  │     │  4 Bücher                              │
│  └─────┘                                        │
└─────────────────────────────────────────────────┘
```

**Serien-Detail (nach Klick):**
```
┌─────────────────────────────────────────────────┐
│  ← Zurück                                       │
│                                                 │
│  Die drei ???                                   │
│  12 Bücher · ~84 Stunden                        │
├─────────────────────────────────────────────────┤
│  1. Das Gespensterschloss        [▶️]  45%     │
│  2. Der Super-Papagei            [▶️]  100%    │
│  3. Der Karpatenhund             [▶️]  0%      │
│  ...                                            │
└─────────────────────────────────────────────────┘
```

**Wichtig:** Die `Sequence`-Nummer aus den Metadaten bestimmt die Reihenfolge!

#### 4. Autoren-Ansicht

**Liste aller Autoren:**
```
┌─────────────────────────────────────────────────┐
│  Autoren                             🔍 Filter  │
├─────────────────────────────────────────────────┤
│  A                                              │
│  ────────────────────────────                   │
│  Agatha Christie (15 Bücher)               →    │
│  Arthur Conan Doyle (8 Bücher)             →    │
│                                                 │
│  B                                              │
│  ────────────────────────────                   │
│  Brandon Sanderson (12 Bücher)             →    │
└─────────────────────────────────────────────────┘
```

**Alphabetische Gruppierung** mit Buchstaben-Überschriften für bessere Navigation.

#### 5. Genres-Ansicht

```
┌─────────────────────────────────────────────────┐
│  Genres                                         │
├─────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐            │
│  │   Krimi      │  │  Sachbuch    │            │
│  │   23 Bücher  │  │  18 Bücher   │            │
│  └──────────────┘  └──────────────┘            │
│  ┌──────────────┐  ┌──────────────┐            │
│  │  Science     │  │  Fantasy     │            │
│  │  Fiction     │  │  31 Bücher   │            │
│  │  45 Bücher   │  │              │            │
│  └──────────────┘  └──────────────┘            │
└─────────────────────────────────────────────────┘
```

**Kachel-Darstellung** für Genres (ähnlich wie in Audiobookshelf).

#### 6. Filter in der Bibliotheks-Ansicht

Zusätzlich zur Navigation: Filter-Chips in der normalen Bücher-Ansicht:

```
┌─────────────────────────────────────────────────┐
│  Bibliothek: audible                   [Filter] │
├─────────────────────────────────────────────────┤
│  Aktive Filter: [Krimi ✕] [Agatha Christie ✕]  │
├─────────────────────────────────────────────────┤
│  📚 15 Ergebnisse                               │
│  ...                                            │
└─────────────────────────────────────────────────┘
```

### Technische Umsetzung

#### Backend-Änderungen

1. **Neue API-Endpoints:**
   ```
   GET /recent                          → Zuletzt gehörte Bücher
   GET /libraries/{id}/series           → Liste aller Serien
   GET /libraries/{id}/series/{seriesId} → Bücher einer Serie
   GET /libraries/{id}/authors          → Liste aller Autoren
   GET /libraries/{id}/authors/{authorId} → Bücher eines Autors
   GET /libraries/{id}/genres           → Liste aller Genres
   ```

2. **"Zuletzt gehört" Datenquelle:**
   - **Option A:** Audiobookshelf API `/api/me/items-in-progress`
   - **Option B:** Lokale `playback_sessions` Tabelle (bereits vorhanden)
   - **Empfehlung:** Kombination - ABS für Fortschritt, lokal für "Vor X Stunden"

3. **Erweiterung `ItemsOptions`:**
   ```go
   type ItemsOptions struct {
       // ... bestehende Felder ...
       CollapseSeries bool   // Serien zusammenfassen
       FilterType     string // "authors", "series", "genres", "tags"
       FilterValue    string // Base64-kodierte ID oder Wert
   }
   ```

4. **Neue Typen:**
   ```go
   type SeriesWithBooks struct {
       ID        string
       Name      string
       Books     []LibraryItem
       BookCount int
       TotalDuration float64
   }

   type AuthorWithBooks struct {
       ID        string
       Name      string
       BookCount int
   }
   ```

5. **Serien-Sortierung:**
   - Bücher innerhalb einer Serie nach `Sequence` sortieren
   - `Sequence` kann "1", "2", "1.5" (Zwischenbände) oder leer sein

#### Frontend-Änderungen

1. **Neue Templates:**
   ```
   web/templates/recent.html        → Zuletzt gehört
   web/templates/series.html        → Serien-Übersicht
   web/templates/series-detail.html → Serien-Detail
   web/templates/authors.html       → Autoren-Übersicht
   web/templates/author-detail.html → Autor-Detail
   web/templates/genres.html        → Genres-Übersicht
   ```

2. **Responsive Navigation:**
   - **Desktop:** Tab-Bar in `layout.html` (CSS: `display: flex` ab Breakpoint)
   - **Mobil:** Burger-Menü mit Slide-In (CSS: `display: none` unter Breakpoint)
   - Aktive Tab/Menüpunkt-Markierung
   - Breakpoint ca. 768px (Tablet/Desktop-Grenze)

3. **Partials:**
   ```
   web/templates/partials/nav-tabs.html      → Desktop Tab-Bar
   web/templates/partials/nav-burger.html    → Mobiles Burger-Menü
   web/templates/partials/recent-item.html   → Zuletzt gehört Eintrag
   web/templates/partials/filter-chips.html  → Aktive Filter anzeigen
   web/templates/partials/series-card.html   → Serien-Karte
   web/templates/partials/author-row.html    → Autor-Zeile
   ```

4. **CSS:**
   ```css
   /* Responsive Navigation */
   .nav-tabs { display: none; }
   .nav-burger { display: block; }

   @media (min-width: 768px) {
       .nav-tabs { display: flex; }
       .nav-burger { display: none; }
   }
   ```

### Datenfluss

```
┌─────────────────────────────────────────────────────────────┐
│                    Audiobookshelf API                        │
├─────────────────────────────────────────────────────────────┤
│  /api/libraries/{id}/filterdata  → Autoren, Serien, Genres  │
│  /api/libraries/{id}/items?filter=series.XXX&collapseseries │
│  /api/libraries/{id}/items?filter=authors.XXX               │
│  /api/libraries/{id}/items?filter=genres.XXX                │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    Bridge Backend                            │
├─────────────────────────────────────────────────────────────┤
│  LibraryHandler.HandleSeries()                               │
│  LibraryHandler.HandleSeriesDetail()                         │
│  LibraryHandler.HandleAuthors()                              │
│  LibraryHandler.HandleAuthorDetail()                         │
│  LibraryHandler.HandleGenres()                               │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (htmx)                           │
├─────────────────────────────────────────────────────────────┤
│  Tab-Navigation → GET /libraries/{id}/series                 │
│  Serien-Klick   → GET /libraries/{id}/series/{seriesId}      │
│  Filter-Chip    → GET /libraries/{id}/items?filter=...       │
└─────────────────────────────────────────────────────────────┘
```

### Entscheidungen

1. **Navigation:** Responsive Design
   - **Desktop:** Tab-Bar (immer sichtbar)
   - **Mobil:** Burger-Menü (platzsparend)

2. **Serien-Fortschritt:** Nicht anzeigen (z.B. "4 von 7 Büchern gehört" wird vorerst nicht implementiert)

3. **"Zuletzt gehört":** Ja, eigene Ansicht für kürzlich gehörte Bücher implementieren

### Offene Fragen

1. **Serien-Cover:** Erstes Buch der Serie oder eigenes Serien-Cover (falls vorhanden)?

2. **Leere Kategorien:** Genres/Tags ohne Bücher ausblenden?

3. **Caching:** FilterData cachen? (Autoren/Serien ändern sich selten)

### Abhängigkeiten

- `GetFilterData()` bereits implementiert
- `ItemsOptions.Filter` bereits implementiert
- Metadaten-Strukturen vorhanden (`Series`, `Author`, `Genre`)

### Geschätzter Aufwand

| Komponente | Aufwand |
|------------|---------|
| Backend: "Zuletzt gehört" Endpoint | 1-2h |
| Backend: Serien-Endpoints | 2-3h |
| Backend: Autoren-Endpoints | 1-2h |
| Backend: Genres-Endpoints | 1h |
| Frontend: Responsive Navigation (Desktop + Burger) | 3-4h |
| Frontend: "Zuletzt gehört" UI | 2h |
| Frontend: Serien-UI | 3-4h |
| Frontend: Autoren-UI | 2-3h |
| Frontend: Genres-UI | 2h |
| Frontend: Filter-Chips | 2h |
| Testing | 2-3h |
| **Gesamt** | **21-28h** |

### Priorisierung (Vorschlag)

1. **Phase 1:** Responsive Navigation + "Zuletzt gehört" (Grundgerüst für alle weiteren Features)
2. **Phase 2:** Serien-Ansicht (höchster Mehrwert für Hörbuch-Nutzer)
3. **Phase 3:** Autoren-Ansicht
4. **Phase 4:** Genres und Filter-Chips

### Quellen

- [Audiobookshelf API Reference](https://api.audiobookshelf.org/)
- [GitHub Issue: Collapse Series Bug](https://github.com/advplyr/audiobookshelf/issues/3049)

---

## Lokale Browser-Wiedergabe

**Status:** Teilweise implementiert
**Priorität:** Hoch
**Genehmigt:** 2025-12-22

### Erledigte Aufgaben

- [x] **Sleep Timer (Server-Side)** - 2025-12-23
  - `SleepTimerWorker` Background Service
  - `sleep_at` Spalte in `playback_sessions` Tabelle
  - POST/DELETE/GET `/sleep-timer` Endpoints
  - Sleep Timer UI (Modal mit 15/30/45/60/90/120 Min Optionen)
  - Countdown-Anzeige auf Button
  - Funktioniert für Sonos-Wiedergabe
  - Timer wird bei Buchwechsel automatisch gelöscht

### Übersicht

Zusätzlich zur Sonos-Wiedergabe soll die Web-App auch lokale Wiedergabe im Browser unterstützen - genau wie Audiobookshelf selbst.

```
┌─────────────────────────────────────────────────────────────────┐
│                    AKTUELLE ARCHITEKTUR                         │
│                                                                 │
│   Browser ──HTTP──► Bridge ──UPnP──► Sonos                     │
│   (Fernbedienung)           (Streaming)                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

                              ▼

┌─────────────────────────────────────────────────────────────────┐
│                    ERWEITERTE ARCHITEKTUR                       │
│                                                                 │
│   Browser ──HTTP──► Bridge ──UPnP──► Sonos                     │
│      │              (Streaming)                                 │
│      │                                                          │
│      └──HTML5 Audio──► Bridge Cache                            │
│        (Lokale Wiedergabe)                                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Kernkonzepte

#### Eine Session pro User

- Jeder User kann nur **eine aktive Playback-Session** haben
- Bei Wechsel des Targets (Sonos → Browser oder umgekehrt) wird die alte Session gestoppt
- Progress wird vor dem Stoppen zu Audiobookshelf synchronisiert
- User ist gedacht als Mensch - niemand hört zwei Hörbücher gleichzeitig

```
┌─────────────────────────────────────────────────────────────────┐
│  User "Peter" hört auf Sonos Küche                              │
│                                                                 │
│  Peter klickt "Play" auf iPhone (Browser)                      │
│                              ↓                                  │
│  1. Sonos Küche → STOP + Progress Sync                         │
│  2. Alte PlaybackSession → Cleanup                             │
│  3. Neue PlaybackSession → Browser                             │
│  4. Audio startet im Browser                                   │
└─────────────────────────────────────────────────────────────────┘
```

#### Sonos und Browser nicht gleichzeitig

- Wenn User auf Browser-Playback wechselt → Sonos stoppt automatisch
- Wenn User auf Sonos wechselt → Browser-Playback stoppt
- Verhindert Konflikte bei Progress-Sync

---

### Streaming-Strategie

**Phase 1: Eigenen Cache nutzen**

| Aspekt | Details |
|--------|---------|
| Quelle | `/cache/{item_id}/audio.{ext}` |
| Vorteile | Bereits vorhanden, kein zusätzliches Transcoding, offline-fähig |
| Nachteile | Keine Kapitel-Navigation (ein File = ganzes Buch) |
| Seeking | Via HTTP Range Requests (bereits implementiert) |

**Phase 2 (später): ABS Direct Stream**

| Aspekt | Details |
|--------|---------|
| Quelle | ABS `/api/items/{id}/play` → HLS/Direct |
| Vorteile | Kapitelweise Navigation, kein lokaler Cache nötig |
| Nachteile | Zusätzliche API-Komplexität, ABS muss erreichbar sein |

---

### UI-Konzept

#### Player-Auswahl (erweitert)

```
┌─────────────────────────────────────┐
│  Player auswählen               ↻   │
├─────────────────────────────────────┤
│  📱 Dieses Gerät                    │  ← NEU
│  ─────────────────────────────────  │
│  🔊 Sonos Geräte                    │
│  ○ Kamin [+1]                       │
│  ○ Küche                            │
│  ○ Bad                              │
└─────────────────────────────────────┘
```

**Hinweise bei "Dieses Gerät":**
- Kann nicht mit Sonos gruppiert werden
- Wiedergabe stoppt wenn Tab geschlossen wird (außer mit Media Session)
- Progress wird zu Audiobookshelf synchronisiert

#### Browser-Player UI

**Desktop:**
```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  ┌─────────┐                                                    │
│  │         │  Der Herr der Ringe                                │
│  │  Cover  │  J.R.R. Tolkien                                    │
│  │         │  Gelesen von Gert Heidenreich                      │
│  └─────────┘                                                    │
│                                                                 │
│  ━━━━━━━━━━━━━━━━━━●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│  2:34:12                                              12:45:30  │
│                                                                 │
│              ◀️30s      ▶️⏸️      30s▶️                         │
│                                                                 │
│  🔊 ━━━━━━━━━━━━━━━━━━━━━━━ 75%                                 │
│                                                                 │
│  Geschwindigkeit: [0.75x] [1x] [1.25x] [1.5x] [2x]             │
│                                                                 │
│  😴 Sleep Timer: Aus  [Einstellen]                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Mobile:**
```
┌───────────────────────────────┐
│  Der Herr der Ringe           │
│  ━━━━━━━━●━━━━━━━━━━━━━━━━━━  │
│  2:34:12          12:45:30    │
│                               │
│     ◀️30   ▶️⏸️   30▶️        │
│                               │
│  🔊━━━━━━━━━ 75%    1x ▼      │
│                               │
│  😴 Sleep: 30 Min             │
└───────────────────────────────┘
```

---

### Media Session API

Die Media Session API ermöglicht native OS-Integration für Mediensteuerung.

**Was sie ermöglicht:**

| Gerät/OS | Wo sichtbar |
|----------|-------------|
| iPhone/iPad | Lock Screen, Control Center, CarPlay |
| Android | Notification, Lock Screen, Quick Settings |
| macOS | Control Center, Touch Bar, Now Playing Widget |
| Windows | System Media Controls, Bluetooth Geräte |

**Zusätzliche Vorteile:**
- Bluetooth-Kopfhörer Play/Pause-Taste funktioniert
- AirPods Doppeltippen = Skip
- Keyboard Media Keys (⏯️ ⏮️ ⏭️) funktionieren

**Beispiel Lock Screen (iPhone):**
```
┌─────────────────────────────────────┐
│  iPhone Sperrbildschirm             │
│  ┌─────────────────────────────┐   │
│  │  🎧 Der Herr der Ringe      │   │
│  │     J.R.R. Tolkien          │   │
│  │  ┌────────┐                 │   │
│  │  │ Cover  │  ▶️ ABS-Sonos   │   │
│  │  └────────┘     Bridge      │   │
│  │                              │   │
│  │  ◀️◀️    ▶️⏸️    ▶️▶️       │   │
│  │  ━━━━━━━━━●━━━━━━━━━━       │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

---

### Sleep Timer (Server-Side)

Der Sleep Timer wird server-side implementiert, da:
- Sonos nur vom Server gestoppt werden kann (UPnP)
- Konsistentes Verhalten für beide Targets (Sonos + Browser)

**Architektur:**

```
┌─────────────────────────────────────────────────────────────────┐
│  User stellt Timer: 30 Minuten                                 │
│                              ↓                                  │
│  PlaybackSession.SleepAt = now() + 30min                       │
│                              ↓                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  SleepTimerWorker (Background Goroutine)                │   │
│  │                                                          │   │
│  │  Alle 10 Sekunden:                                       │   │
│  │  FOR each session WHERE SleepAt != NULL:                 │   │
│  │      IF now() >= SleepAt:                                │   │
│  │          IF target == SONOS:                             │   │
│  │              → Send UPnP Pause                           │   │
│  │          IF target == BROWSER:                           │   │
│  │              → Set session.SleepTriggered = true         │   │
│  │          → Sync progress to ABS                          │   │
│  │          → Clear SleepAt                                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Für Browser-Playback:**

```
Browser sendet Position-Update alle 5 Sekunden:

POST /play/browser/position
{ position: 1234.5, playing: true }

Server prüft: session.SleepTriggered == true?
                    ↓
Response: { shouldStop: true, reason: "sleep_timer" }
                    ↓
Browser: audio.pause();
         showNotification("Sleep Timer abgelaufen");
```

**Sleep Timer UI:**

```
┌─────────────────────────────────────┐
│  Sleep Timer                    ✕   │
├─────────────────────────────────────┤
│                                     │
│  ○ Aus                              │
│  ○ 15 Minuten                       │
│  ● 30 Minuten  ← Ausgewählt        │
│  ○ 45 Minuten                       │
│  ○ 60 Minuten                       │
│  ○ 90 Minuten                       │
│  ○ 120 Minuten                      │
│                                     │
│  Verbleibend: 24:32                 │
│                                     │
└─────────────────────────────────────┘
```

**API Endpoints:**

| Endpoint | Methode | Beschreibung |
|----------|---------|--------------|
| `POST /sleep-timer` | POST | Timer setzen: `{ minutes: 30 }` |
| `DELETE /sleep-timer` | DELETE | Timer löschen |
| `GET /sleep-timer` | GET | Verbleibende Zeit abfragen |

---

### AirPlay und Google Cast

**Gute Nachricht:** AirPlay und Google Cast sind "gratis" wenn wir Browser-Playback haben!

**Wie es funktioniert:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    AIRPLAY VIA BROWSER                          │
│                                                                 │
│   ┌──────────────────┐                                          │
│   │  iPhone Safari   │                                          │
│   │                  │                                          │
│   │  <audio> Element │──── AirPlay ────► HomePod / Apple TV    │
│   │                  │     (natives iOS)                        │
│   └──────────────────┘                                          │
│                                                                 │
│   Der Browser spielt das Audio ab.                              │
│   iOS bietet nativ AirPlay an.                                  │
│   Wir müssen NICHTS implementieren!                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Browser-Unterstützung:**

| Browser | AirPlay | Google Cast | Bemerkung |
|---------|---------|-------------|-----------|
| Safari macOS | ✅ Nativ | ❌ | AirPlay über Menüleiste |
| Safari iOS | ✅ Nativ | ❌ | AirPlay-Button im Player |
| Chrome | ❌ | ✅ Nativ | Cast-Button im Browser |
| Chrome Android | ❌ | ✅ Nativ | Cast-Button im Player |
| Firefox | ❌ | ❌ | Keine Cast-Unterstützung |
| Edge | ❌ | ✅ | Über Chromium |

**Streaming-Methoden im Vergleich:**

```
SONOS (Remote Rendering)
────────────────────────
Bridge ──Stream URL──► Sonos ──Audio──► Lautsprecher
Sonos holt sich den Stream selbst

BROWSER (Local Rendering)
─────────────────────────
Bridge ──Stream──► Browser ──Audio──► Gerät-Lautsprecher
Browser spielt ab, Audio kommt aus dem Gerät

AIRPLAY via BROWSER (Local + Cast)
──────────────────────────────────
Bridge ──Stream──► Browser ──AirPlay──► HomePod
Browser spielt ab, iOS streamt zu AirPlay
```

**Keine Gruppierung:**
- AirPlay-Geräte können nicht mit Sonos gruppiert werden
- Das ist technisch nicht möglich (unterschiedliche Protokolle)
- Kein zusätzlicher Implementierungsaufwand

---

### Technische Architektur

#### Backend-Änderungen

**1. PlaybackSession erweitern (`internal/store/playback.go`):**

```go
type PlaybackTarget string

const (
    TargetSonos   PlaybackTarget = "sonos"
    TargetBrowser PlaybackTarget = "browser"
)

type PlaybackSession struct {
    // ... bestehende Felder ...

    // Target-Typ
    Target         PlaybackTarget  // "sonos" oder "browser"

    // Browser-spezifisch
    BrowserPlaying bool            // Aktueller Play-State

    // Sleep Timer (für beide Targets)
    SleepAt        *time.Time      // NULL = kein Timer
    SleepTriggered bool            // Für Browser: Signal zum Stoppen
}
```

**2. Neue Endpoints:**

| Endpoint | Methode | Beschreibung |
|----------|---------|--------------|
| `POST /play/browser` | POST | Startet Browser-Wiedergabe |
| `GET /play/browser/status` | GET | Status für Browser-Player |
| `POST /play/browser/position` | POST | Position-Update vom Browser |
| `POST /play/browser/pause` | POST | Pause im Browser |
| `POST /play/browser/resume` | POST | Resume im Browser |
| `POST /sleep-timer` | POST | Sleep Timer setzen |
| `DELETE /sleep-timer` | DELETE | Sleep Timer löschen |
| `GET /sleep-timer` | GET | Verbleibende Zeit |

**3. Neue Background Worker:**

- `SleepTimerWorker`: Prüft alle 10s ob Timer abgelaufen

#### Frontend-Änderungen

**1. Neues Partial: `web/templates/partials/browser-player.html`**

```html
<div id="browser-player" class="hidden">
    <audio id="audio-element"
           x-webkit-airplay="allow"
           preload="metadata">
    </audio>

    <!-- Custom Controls -->
    <div class="player-controls">
        <!-- Progress Bar -->
        <input type="range" id="seek-slider" />

        <!-- Transport -->
        <button id="skip-back">-30s</button>
        <button id="play-pause">▶️</button>
        <button id="skip-forward">+30s</button>

        <!-- Volume -->
        <input type="range" id="volume-slider" />

        <!-- Playback Speed -->
        <select id="playback-rate">
            <option value="0.75">0.75x</option>
            <option value="1" selected>1x</option>
            <option value="1.25">1.25x</option>
            <option value="1.5">1.5x</option>
            <option value="2">2x</option>
        </select>

        <!-- Sleep Timer -->
        <button id="sleep-timer-btn">😴</button>
    </div>
</div>
```

**2. JavaScript-Klasse: `web/static/js/browser-player.js`**

```javascript
class BrowserPlayer {
    constructor(audioElement) {
        this.audio = audioElement;
        this.sessionId = null;
        this.positionSyncInterval = null;
    }

    async play(streamUrl, startPosition) {
        this.audio.src = streamUrl;
        this.audio.currentTime = startPosition;
        await this.audio.play();
        this.setupMediaSession();
        this.startPositionSync();
    }

    setupMediaSession() {
        if ('mediaSession' in navigator) {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: this.bookTitle,
                artist: this.author,
                artwork: [{ src: this.coverUrl }]
            });

            navigator.mediaSession.setActionHandler('play', () => this.resume());
            navigator.mediaSession.setActionHandler('pause', () => this.pause());
            navigator.mediaSession.setActionHandler('seekbackward', () => this.skip(-30));
            navigator.mediaSession.setActionHandler('seekforward', () => this.skip(30));
        }
    }

    startPositionSync() {
        this.positionSyncInterval = setInterval(async () => {
            const response = await this.syncPosition();
            if (response.shouldStop) {
                this.pause();
                this.showNotification(response.reason);
            }
        }, 5000);
    }

    async syncPosition() {
        return fetch('/play/browser/position', {
            method: 'POST',
            body: JSON.stringify({
                position: this.audio.currentTime,
                playing: !this.audio.paused
            })
        }).then(r => r.json());
    }
}
```

---

### Datenfluss

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. User wählt "Dieses Gerät" als Player                         │
│    → localStorage.selectedPlayer = { type: "browser" }          │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 2. User klickt "Play" auf einem Buch                            │
│                                                                 │
│    POST /play/browser                                           │
│    Body: { itemId: "abc123" }                                   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 3. Backend:                                                     │
│    - Prüft auf existierende Session (stoppt ggf. Sonos)        │
│    - Prüft Cache (wie bei Sonos)                               │
│    - Holt gespeicherte Position von ABS                        │
│    - Erstellt PlaybackSession (Target: browser)                │
│    - Generiert Stream-Token                                     │
│    - Gibt zurück: { streamUrl, position, duration, metadata }  │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 4. Frontend:                                                    │
│    - Setzt <audio src="streamUrl">                             │
│    - Springt zu gespeicherter Position                         │
│    - Startet Wiedergabe                                         │
│    - Registriert Media Session (Lock Screen Controls)          │
│    - Startet Position-Sync (alle 5s)                           │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 5. Während Wiedergabe:                                          │
│                                                                 │
│    Browser ──POST /play/browser/position──► Backend            │
│             { position: 1234.5, playing: true }                │
│                              ↓                                  │
│    Backend prüft:                                               │
│    - Sleep Timer abgelaufen? → { shouldStop: true }            │
│    - Sync zu ABS (alle 30 Sekunden)                            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 6. Bei Pause/Stop:                                              │
│    - Sofortiger Sync zu ABS                                    │
│    - Session cleanup (bei Stop)                                │
└─────────────────────────────────────────────────────────────────┘
```

---

### Browser-Kompatibilität

| Browser | MP3 | AAC (M4A) | FLAC | Media Session |
|---------|-----|-----------|------|---------------|
| Chrome | ✅ | ✅ | ✅ | ✅ |
| Safari | ✅ | ✅ | ❌ | ✅ |
| Firefox | ✅ | ✅ | ✅ | ✅ |
| Safari iOS | ✅ | ✅ | ❌ | ✅ |
| Chrome Android | ✅ | ✅ | ✅ | ✅ |

**Hinweis:** Safari unterstützt kein FLAC nativ. Für Safari-User würden FLAC-Bücher zu MP3 transkodiert (nicht nur gemuxt).

---

### Phasen der Implementierung

#### Phase 1: Basis-Player

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 1.1 | PlaybackSession erweitern | `Target` und `SleepAt` Fields |
| 1.2 | Eine-Session-pro-User Logik | Alte Session stoppen bei neuem Play |
| 1.3 | `POST /play/browser` Endpoint | Startet Browser-Session |
| 1.4 | `POST /play/browser/position` Endpoint | Position-Updates |
| 1.5 | `POST /play/browser/pause` Endpoint | Pause-Handling |
| 1.6 | Player-Picker erweitern | "Dieses Gerät" Option |
| 1.7 | Browser-Player Partial | HTML + CSS |
| 1.8 | `BrowserPlayer` JS-Klasse | Audio-Steuerung |
| 1.9 | Progress Sync | Browser-Sessions zu ABS syncen |

#### Phase 2: Sleep Timer ✅ ERLEDIGT (2025-12-23)

| # | Aufgabe | Beschreibung | Status |
|---|---------|--------------|--------|
| 2.1 | `SleepTimerWorker` | Background Goroutine | ✅ |
| 2.2 | Sleep Timer Endpoints | POST/DELETE/GET | ✅ |
| 2.3 | Sleep Timer UI | Modal mit Optionen | ✅ |
| 2.4 | Sonos-Integration | Sleep Timer auch für Sonos | ✅ |

#### Phase 3: Erweiterte Features

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 3.1 | Media Session API | Lock Screen Controls |
| 3.2 | Playback Speed | 0.5x - 2x |
| 3.3 | Skip-Buttons | ±30s, ±10s |
| 3.4 | Volume-Slider | Lokale Lautstärke |
| 3.5 | Keyboard Shortcuts | Space, Pfeiltasten |
| 3.6 | Responsive UI | Mobile-optimiert |

#### Phase 4: Optimierungen (optional)

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 4.1 | Kapitel-Navigation | Via ABS Direct Stream |
| 4.2 | Lesezeichen | Manuelle Marker |
| 4.3 | Offline-Mode | Service Worker |

---

### Vergleich: Sonos vs. Browser

| Aspekt | Sonos | Browser |
|--------|-------|---------|
| Steuerung | UPnP SOAP | HTML5 Audio API |
| Stream-Quelle | Cache via HTTP | Cache via HTTP (identisch) |
| Volume | Sonos-Hardware | Browser/OS Volume |
| Seek | AVTransport Seek | `audio.currentTime` |
| Status-Polling | UPnP GetPositionInfo | JavaScript `timeupdate` Event |
| Gruppierung | Ja (Sonos Groups) | Nein |
| Sleep Timer | Server-side (UPnP Stop) | Server-side (Signal via Response) |
| Background Play | Immer aktiv | Media Session API |
| Lock Screen | N/A | Media Session API |
| AirPlay | Nein | Ja (Safari/iOS nativ) |
| Google Cast | Nein | Ja (Chrome nativ) |
