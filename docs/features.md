# Geplante Features

## Sonos-Gruppierung

**Status:** Geplant
**Priorität:** Hoch (wichtig für finale App-Funktion)

### Beschreibung

Ermöglicht das Bilden und Auflösen von Sonos-Gruppen direkt aus der App heraus. Gruppierte Player spielen synchron dasselbe Audio ab.

### UI-Konzept

1. **Player-Auswahl im Sonos-Picker:**
   - User wählt einen Player aus der Liste
   - Wenn der Player eine Gruppe anführt (Coordinator mit GroupSize > 1), erscheint rechts neben dem Player-Namen ein **"Gruppe"**-Button

2. **Gruppen-Editor (Modal oder Slide-In):**
   - Zeigt alle verfügbaren Player als Checkbox-Liste
   - Aktuell gruppierte Player sind vorausgewählt
   - User kann Player an- und abwählen
   - Der aktuelle Coordinator ist markiert (z.B. Krone-Icon)

3. **Coordinator-Wechsel:**
   - Wenn der aktuelle Coordinator abgewählt wird:
     - Der oberste verbleibende Player wird automatisch zum neuen Coordinator
     - Bestätigungsdialog vor Ausführung: "Kamin wird die Gruppe verlassen. Küche wird neuer Gruppenführer."

4. **Bestätigung:**
   - Änderungen werden erst nach Klick auf "Übernehmen" ausgeführt
   - "Abbrechen" verwirft alle Änderungen

### UI-Mockup (ASCII)

```
┌─────────────────────────────────────┐
│  Select Sonos Device            ↻   │
├─────────────────────────────────────┤
│  ○ Annas Zimmer                     │
│  ○ Bad                              │
│  ● Kamin [+1]  [Gruppe]  ←── Button │
│  ○ Schlafzimmer                     │
│  ○ Büro                             │
└─────────────────────────────────────┘

         ↓ Klick auf [Gruppe]

┌─────────────────────────────────────┐
│  Gruppe bearbeiten              ✕   │
├─────────────────────────────────────┤
│  Wähle Player für diese Gruppe:     │
│                                     │
│  ☑ Kamin 👑 (Gruppenführer)         │
│  ☑ Küche                            │
│  ☐ Annas Zimmer                     │
│  ☐ Bad                              │
│  ☐ Schlafzimmer                     │
│  ☐ Büro                             │
│                                     │
│  [Abbrechen]         [Übernehmen]   │
└─────────────────────────────────────┘
```

### Technische Umsetzung

#### SOAP-Actions (UPnP AVTransport)

**1. Player zu Gruppe hinzufügen:**
```xml
<!-- SetAVTransportURI auf dem Player, der hinzugefügt werden soll -->
<u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
  <InstanceID>0</InstanceID>
  <CurrentURI>x-rincon:RINCON_COORDINATOR_UUID</CurrentURI>
  <CurrentURIMetaData></CurrentURIMetaData>
</u:SetAVTransportURI>
```

**2. Player aus Gruppe entfernen (standalone machen):**
```xml
<!-- BecomeCoordinatorOfStandaloneGroup auf dem Player -->
<u:BecomeCoordinatorOfStandaloneGroup xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
  <InstanceID>0</InstanceID>
</u:BecomeCoordinatorOfStandaloneGroup>
```

**3. Coordinator wechseln:**
- Neuen Coordinator aus der Gruppe entfernen (BecomeCoordinatorOfStandaloneGroup)
- Alte Gruppe-Mitglieder zum neuen Coordinator hinzufügen (SetAVTransportURI)
- Alten Coordinator zum neuen hinzufügen (falls er in der Gruppe bleiben soll)

#### Backend-Änderungen

1. **`internal/sonos/avtransport.go`** - Neue Methoden:
   ```go
   func (c *AVTransportClient) JoinGroup(ctx context.Context, coordinatorUUID string) error
   func (c *AVTransportClient) LeaveGroup(ctx context.Context) error
   ```

2. **`internal/web/sonos.go`** - Neue Endpoints:
   ```
   POST /sonos/group/join    - Player zu Gruppe hinzufügen
   POST /sonos/group/leave   - Player aus Gruppe entfernen
   POST /sonos/group/update  - Komplette Gruppe aktualisieren (Batch)
   ```

3. **Gruppenlogik:**
   - Bei Coordinator-Wechsel: Reihenfolge der SOAP-Calls ist wichtig
   - Erst neuen Coordinator erstellen, dann Mitglieder umziehen

#### Frontend-Änderungen

1. **`web/templates/partials/sonos-picker.html`:**
   - "Gruppe"-Button bei Coordinators mit GroupSize > 1
   - Auch bei Standalone-Playern optional (um neue Gruppe zu starten)

2. **Neues Template `sonos-group-editor.html`:**
   - Checkbox-Liste aller Player
   - Coordinator-Markierung
   - Übernehmen/Abbrechen Buttons

3. **JavaScript:**
   - Gruppen-Editor öffnen/schließen
   - Änderungen sammeln und als Batch senden
   - Optimistic UI vs. Warten auf Bestätigung

### Offene Fragen

1. **Neue Gruppe starten:** Soll man auch bei Standalone-Playern eine Gruppe starten können? (Vermutlich ja)

2. **Leere Gruppe:** Was passiert wenn alle Player abgewählt werden? → Alle werden standalone

3. **Playback bei Gruppierung:** Soll das aktuelle Playback beim Gruppieren weiterlaufen? Sonos macht das automatisch - der neue Player übernimmt den Stream des Coordinators.

4. **Fehlerbehandlung:** Was wenn ein Player nicht erreichbar ist während der Gruppierung?

5. **Live-Updates:** Soll die Gruppen-Ansicht live aktualisiert werden (WebSocket/Polling) oder nur bei manuellem Refresh?

### Abhängigkeiten

- Bestehende ZoneGroupTopology-Implementierung (vorhanden)
- AVTransport Client (vorhanden, muss erweitert werden)
- Device Discovery (vorhanden)

### Geschätzter Aufwand

| Komponente | Aufwand |
|------------|---------|
| Backend SOAP-Actions | 1-2h |
| Backend Endpoints | 1-2h |
| Gruppenlogik (Coordinator-Wechsel) | 2-3h |
| Frontend UI | 4-6h |
| Testing & Edge Cases | 2-3h |
| **Gesamt** | **10-16h** |

---

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
