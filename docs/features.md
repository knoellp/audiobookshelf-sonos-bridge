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

## Sonos-Gruppen-Wiedergabe & Lautstärkeregelung

**Status:** Geplant
**Priorität:** Hoch (kritisch für Gruppen-Nutzung)

### Problem 1: Wiedergabe nur auf Gruppenführer

**Symptom:** Wenn ein gruppierter Lautsprecher als Ziel ausgewählt wird, spielt das Audio nur auf dem Gruppenführer (Coordinator), nicht auf allen Gruppenmitgliedern.

**Ursache:** Die aktuelle Implementierung sendet AVTransport-Befehle (SetAVTransportURI, Play, Pause, etc.) direkt an die IP-Adresse des vom Benutzer ausgewählten Geräts. Bei Sonos-Gruppen müssen **alle Befehle an den Coordinator** gesendet werden - nur dieser kann die gesamte Gruppe steuern.

**Beispiel des Problems:**
```
Gruppe: Kamin (Coordinator) + Küche (Member)
Benutzer wählt: Küche
Aktuell: SetAVTransportURI → 192.168.1.50 (Küche) → Nur Küche spielt
Korrekt: SetAVTransportURI → 192.168.1.40 (Kamin) → Ganze Gruppe spielt
```

### Problem 2: Gruppen-Lautstärkeregelung

**Symptom:** Die aktuelle Lautstärkeregelung kann nur einzelne Lautsprecher steuern. Bei Gruppen fehlen:
1. **Relative Gruppen-Lautstärke:** Alle Lautsprecher proportional lauter/leiser
2. **Individuelle Lautstärke:** Einzelne Lautsprecher in der Gruppe anpassen

### Lösung: Coordinator-Routing

#### Schritt 1: Coordinator ermitteln

Die bestehende `GetGroupInfo()` Funktion in `internal/sonos/zonegroupstate.go` liefert bereits:
```go
type GroupInfo struct {
    CoordinatorUUID string   // UUID des Gruppenführers
    CoordinatorIP   string   // IP-Adresse des Gruppenführers
    Members         []Member // Alle Gruppenmitglieder
    GroupSize       int      // Anzahl der Mitglieder
}
```

#### Schritt 2: AVTransport-Befehle an Coordinator routen

**Vor dem Senden von AVTransport-Befehlen:**
1. ZoneGroupTopology des ausgewählten Geräts abfragen
2. Coordinator-IP aus GroupInfo extrahieren
3. Alle AVTransport-Befehle an Coordinator-IP senden

**Betroffene Stellen in `internal/web/player.go`:**

| Handler | Aktuelle Logik | Neue Logik |
|---------|---------------|------------|
| `HandlePlay` | Sendet an `playback.SonosIP` | Coordinator-IP ermitteln, dahin senden |
| `HandleResume` | Sendet an `playback.SonosIP` | Coordinator-IP ermitteln, dahin senden |
| `HandlePause` | Sendet an `playback.SonosIP` | Coordinator-IP ermitteln, dahin senden |
| `HandleStop` | Sendet an `playback.SonosIP` | Coordinator-IP ermitteln, dahin senden |
| `HandleSeek` | Sendet an `playback.SonosIP` | Coordinator-IP ermitteln, dahin senden |

**Implementierungsvorschlag:**

```go
// Neue Hilfsfunktion in player.go oder sonos package
func (h *PlayerHandler) getCoordinatorIP(ctx context.Context, deviceIP string) (string, error) {
    zgClient := sonos.NewZoneGroupClient(deviceIP)
    groupInfo, err := zgClient.GetGroupInfo(ctx)
    if err != nil {
        // Fallback: Gerät ist standalone, eigene IP verwenden
        return deviceIP, nil
    }
    if groupInfo.CoordinatorIP != "" {
        return groupInfo.CoordinatorIP, nil
    }
    return deviceIP, nil
}

// Verwendung in HandlePlay:
func (h *PlayerHandler) HandlePlay(...) {
    // ... bestehender Code ...

    // NEU: Coordinator-IP ermitteln
    targetIP, err := h.getCoordinatorIP(ctx, selectedDeviceIP)
    if err != nil {
        slog.Warn("could not get coordinator, using selected device", "error", err)
        targetIP = selectedDeviceIP
    }

    // AVTransport-Client mit Coordinator-IP erstellen
    avt := sonos.NewAVTransportClient(targetIP)
    avt.SetAVTransportURI(ctx, streamURL, metadata)
    avt.Play(ctx)

    // Playback-Session speichert weiterhin die UUID des AUSGEWÄHLTEN Geräts
    // (für UI-Anzeige), aber Befehle gehen an Coordinator
}
```

**Wichtig:** Die `PlaybackSession` speichert weiterhin die UUID/IP des vom Benutzer ausgewählten Geräts (für UI-Konsistenz). Die Coordinator-Ermittlung erfolgt dynamisch bei jedem Befehl.

### Lösung: Gruppen-Lautstärkeregelung

#### Sonos-Services für Lautstärke

| Service | Port | Zweck |
|---------|------|-------|
| RenderingControl | 1400 | Einzelgerät: Lautstärke, Bass, Treble |
| GroupRenderingControl | 1400 | Gruppe: Relative Lautstärke aller Mitglieder |

#### GroupRenderingControl SOAP-Actions

**1. Gruppen-Lautstärke setzen (relativ):**
```xml
<u:SetGroupVolume xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1">
  <InstanceID>0</InstanceID>
  <DesiredVolume>50</DesiredVolume>
</u:SetGroupVolume>
```

**2. Gruppen-Lautstärke abfragen:**
```xml
<u:GetGroupVolume xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1">
  <InstanceID>0</InstanceID>
</u:GetGroupVolume>
```

**3. Relative Lautstärke einzelner Mitglieder:**
```xml
<u:SetRelativeGroupVolume xmlns:u="urn:schemas-upnp-org:service:GroupRenderingControl:1">
  <InstanceID>0</InstanceID>
  <Adjustment>-10</Adjustment>  <!-- Relativ: -100 bis +100 -->
</u:SetRelativeGroupVolume>
```

#### UI-Konzept für Gruppen-Lautstärke

**Aktuelle UI (Einzelgerät):**
```
┌─────────────────────────────────────┐
│  🔊 ━━━━━━━━━━━━━━━━━━━━━━━ 65%    │
└─────────────────────────────────────┘
```

**Neue UI (Gruppe):**
```
┌─────────────────────────────────────┐
│  Gruppen-Lautstärke                 │
│  🔊 ━━━━━━━━━━━━━━━━━━━━━━━ 65%    │  ← Steuert alle proportional
│                                     │
│  ▼ Einzelne Lautsprecher            │  ← Aufklappbar
│  ├─ Kamin 👑        🔊━━━━━ 70%    │
│  └─ Küche           🔊━━━━━ 60%    │
└─────────────────────────────────────┘
```

**Verhalten:**
1. **Gruppen-Slider:** Ändert alle Mitglieder proportional (über GroupRenderingControl)
2. **Einzel-Slider:** Ändert nur dieses Gerät (über RenderingControl an jeweilige IP)
3. **Aufklappbar:** Einzelne Lautsprecher nur bei Bedarf sichtbar

### Technische Umsetzung

#### Backend-Änderungen

**1. Neuer Client: `internal/sonos/grouprendering.go`**
```go
type GroupRenderingClient struct {
    ip string
}

func NewGroupRenderingClient(ip string) *GroupRenderingClient

func (c *GroupRenderingClient) GetGroupVolume(ctx context.Context) (int, error)
func (c *GroupRenderingClient) SetGroupVolume(ctx context.Context, volume int) error
func (c *GroupRenderingClient) GetGroupMute(ctx context.Context) (bool, error)
func (c *GroupRenderingClient) SetGroupMute(ctx context.Context, mute bool) error
```

**2. Erweiterung `internal/sonos/rendering.go`:**
```go
// Bestehend - Einzelgerät:
func (c *RenderingClient) GetVolume(ctx context.Context) (int, error)
func (c *RenderingClient) SetVolume(ctx context.Context, volume int) error

// Neu - Für einzelne Gruppenmitglieder:
// (bereits vorhanden, wird an jeweilige Geräte-IP aufgerufen)
```

**3. Neue API-Endpoints in `internal/web/player.go`:**
```
GET  /volume/group         → Gruppen-Lautstärke abfragen
POST /volume/group         → Gruppen-Lautstärke setzen
GET  /volume/members       → Lautstärke aller Mitglieder
POST /volume/member/{uuid} → Einzelgerät-Lautstärke setzen
```

**4. Coordinator-Routing für alle AVTransport-Befehle:**

In jedem Handler vor AVTransport-Aufrufen:
```go
coordinatorIP, _ := h.getCoordinatorIP(ctx, playback.SonosIP)
avt := sonos.NewAVTransportClient(coordinatorIP)
```

#### Frontend-Änderungen

**1. `web/templates/partials/transport.html`:**
- Gruppen-Lautstärke-Slider hinzufügen
- Aufklappbare Einzelgeräte-Liste
- Unterscheidung: Standalone vs. Gruppe

**2. JavaScript-Erweiterungen:**
```javascript
// Prüfen ob Gruppe aktiv
async function checkGroupStatus() {
    const response = await fetch('/sonos/group-info/' + currentDeviceUUID);
    const data = await response.json();
    if (data.groupSize > 1) {
        showGroupVolumeControls(data.members);
    } else {
        showSingleVolumeControl();
    }
}

// Gruppen-Lautstärke ändern
async function setGroupVolume(volume) {
    await fetch('/volume/group', {
        method: 'POST',
        body: JSON.stringify({ volume: volume })
    });
}

// Einzelgerät-Lautstärke ändern
async function setMemberVolume(uuid, volume) {
    await fetch('/volume/member/' + uuid, {
        method: 'POST',
        body: JSON.stringify({ volume: volume })
    });
}
```

### Datenfluss bei Gruppen-Wiedergabe

```
┌────────────────────────────────────────────────────────────────┐
│ 1. Benutzer wählt "Küche" (Mitglied einer Gruppe)              │
└────────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────────┐
│ 2. Backend: getCoordinatorIP("Küche-IP")                       │
│    → ZoneGroupTopology abfragen                                │
│    → Coordinator = "Kamin-IP"                                  │
└────────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────────┐
│ 3. AVTransport-Befehle → Kamin-IP (Coordinator)                │
│    SetAVTransportURI, Play, Pause, Seek, Stop                  │
└────────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────────┐
│ 4. Sonos-Gruppe: Alle Mitglieder spielen synchron              │
│    Kamin + Küche spielen dasselbe Audio                        │
└────────────────────────────────────────────────────────────────┘
```

### Datenfluss bei Gruppen-Lautstärke

```
┌────────────────────────────────────────────────────────────────┐
│ Gruppen-Slider bewegen                                         │
└────────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────────┐
│ POST /volume/group { volume: 60 }                              │
│ → GroupRenderingControl.SetGroupVolume(60) an Coordinator      │
│ → Alle Mitglieder werden proportional angepasst                │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ Einzel-Slider (Küche) bewegen                                  │
└────────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────────┐
│ POST /volume/member/RINCON_KÜCHE { volume: 45 }                │
│ → RenderingControl.SetVolume(45) an Küche-IP                   │
│ → Nur Küche wird angepasst                                     │
└────────────────────────────────────────────────────────────────┘
```

### Offene Fragen

1. **Coordinator-Wechsel während Wiedergabe:** Was passiert, wenn sich die Gruppe während der Wiedergabe ändert (z.B. Coordinator verlässt Gruppe)?
   - **Vorschlag:** Bei jedem Befehl Coordinator neu ermitteln (nicht cachen)

2. **Status-Polling bei Gruppen:** Soll der Status vom Coordinator oder vom ausgewählten Gerät gelesen werden?
   - **Vorschlag:** Vom Coordinator, da dieser den aktuellen Playback-Status hat

3. **UI bei Gruppenwechsel:** Soll die Lautstärke-UI automatisch aktualisiert werden, wenn sich Gruppen ändern?
   - **Vorschlag:** Bei jedem Status-Poll auch Gruppen-Info prüfen

4. **Latenz bei Coordinator-Ermittlung:** Jede AVTransport-Aktion erfordert einen zusätzlichen HTTP-Request für ZoneGroupTopology
   - **Vorschlag:** Coordinator-Info kurzzeitig cachen (5-10 Sekunden)

### Abhängigkeiten

- ZoneGroupTopology-Implementierung (vorhanden in `zonegroupstate.go`)
- RenderingControl-Implementierung (vorhanden in `rendering.go`)
- AVTransport-Implementierung (vorhanden in `avtransport.go`)
- GroupRenderingControl (NEU zu implementieren)

### Geschätzter Aufwand

| Komponente | Aufwand |
|------------|---------|
| Coordinator-Routing (Backend) | 2-3h |
| GroupRenderingControl Client | 1-2h |
| Volume API Endpoints | 1-2h |
| Frontend: Gruppen-Lautstärke UI | 3-4h |
| Frontend: Einzelgeräte-Liste | 2-3h |
| Testing mit echten Gruppen | 2-3h |
| **Gesamt** | **11-17h** |

### Priorisierung

1. **Phase 1:** Coordinator-Routing (Problem 1 lösen - Wiedergabe funktioniert auf Gruppen)
2. **Phase 2:** Gruppen-Lautstärke (Haupt-Slider)
3. **Phase 3:** Einzelgeräte-Lautstärke (Feintuning)

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

---

## Öffentliche Installation & CI/CD

**Status:** Geplant
**Priorität:** Mittel

### Beschreibung

Verbesserungen für die öffentliche Nutzung des Projekts auf GitHub.

### Erledigte Aufgaben (2025-12-22)

- [x] CLAUDE.md bereinigt (private Pfade/IPs entfernt)
- [x] README.md Umgebungsvariablen korrigiert (`ABS_URL` → `BRIDGE_ABS_URL` etc.)
- [x] LICENSE Datei erstellt (MIT)
- [x] docker-compose.yml aufgeräumt (projektspezifische Volumes entfernt)

### Offene Aufgaben

#### Phase 1: GitHub Actions CI/CD

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 1.1 | **GitHub Actions Workflow** | Automatischer Docker-Build bei git push/tag |
| 1.2 | **Multi-Arch Build** | AMD64 + ARM64 für Raspberry Pi / Mac Silicon |
| 1.3 | **ghcr.io Publishing** | Images unter `ghcr.io/knoellp/audiobookshelf-sonos-bridge` veröffentlichen |
| 1.4 | **Versionierung** | `v1.0.0` Tags → Docker-Tags automatisch erstellen |

**Beispiel `.github/workflows/docker.yml`:**
```yaml
name: Build and Push Docker Image

on:
  push:
    tags: ['v*']
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.ref_name }}
            ghcr.io/${{ github.repository }}:latest
```

#### Phase 2: Benutzerfreundlichkeit

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 2.1 | **Startup-Validierung** | Beim Start prüfen: ffmpeg vorhanden? ABS erreichbar? |
| 2.2 | **HEALTHCHECK fixen** | Port dynamisch oder entfernen (Docker health via /health reicht) |
| 2.3 | **Quickstart Guide** | Vereinfachte 5-Minuten-Anleitung |

**Startup-Validierung Beispiel:**
```go
func validateStartup(cfg *config.Config) error {
    // Check ffmpeg
    if _, err := exec.LookPath("ffmpeg"); err != nil {
        return fmt.Errorf("ffmpeg not found in PATH")
    }

    // Check ABS connectivity
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := http.Get(cfg.ABSURL + "/ping")
    if err != nil {
        return fmt.Errorf("cannot reach Audiobookshelf at %s: %w", cfg.ABSURL, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("Audiobookshelf returned status %d", resp.StatusCode)
    }

    return nil
}
```

#### Phase 3: Fortgeschritten (optional)

| # | Aufgabe | Beschreibung |
|---|---------|--------------|
| 3.1 | **Helm Chart** | Für Kubernetes-Nutzer |
| 3.2 | **Unraid Template** | Für Unraid Community Apps |
| 3.3 | **Config Wizard** | Web-UI zur Erstkonfiguration |

### Geschätzter Aufwand

| Komponente | Aufwand |
|------------|---------|
| GitHub Actions Workflow | 1-2h |
| Multi-Arch Build testen | 1h |
| Startup-Validierung | 1h |
| HEALTHCHECK fix | 15min |
| **Gesamt Phase 1+2** | **3-5h** |
